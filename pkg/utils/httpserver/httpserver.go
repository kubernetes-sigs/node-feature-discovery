/*
Copyright 2026 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package httpserver serves the /metrics and /healthz endpoints of the NFD
// daemons, over plain HTTP by default or over HTTPS with a certificate read
// from files.
package httpserver

import (
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"sync"

	"k8s.io/klog/v2"
)

// Flag names are part of the Helm chart's contract (templates/_helpers.tpl,
// "node-feature-discovery.tls.args") and match k8s.io/apiserver secure serving.
const (
	certFileFlag = "tls-cert-file"
	keyFileFlag  = "tls-private-key-file"
)

// TLSFiles names the PEM-encoded certificate and private key that the endpoint
// presents. The zero value means plain HTTP. Any other value means HTTPS: Load
// refuses a half pair instead of falling back to plain HTTP.
type TLSFiles struct {
	CertFile string
	KeyFile  string
}

// AddFlags registers -tls-cert-file and -tls-private-key-file on flagset,
// bound to t. Call Load after the flagset is parsed.
func (t *TLSFiles) AddFlags(flagset *flag.FlagSet) {
	flagset.StringVar(&t.CertFile, certFileFlag, "",
		"File with the PEM-encoded certificate (chain) for serving the metrics and healthz endpoints over HTTPS. "+
			"Requires -"+keyFileFlag+". The file is re-read on every new connection.")
	flagset.StringVar(&t.KeyFile, keyFileFlag, "",
		"File with the PEM-encoded private key matching -"+certFileFlag+".")
}

// Load turns the flags into what Start serves. Neither flag set returns nil,
// nil for plain HTTP, and a half pair is an error. For a pair it loads the
// files once, so a missing, unreadable or mismatched pair is a startup error,
// and the pair it loaded is what Start presents if the files break before the
// first handshake.
func (t TLSFiles) Load() (*ServingCert, error) {
	switch {
	case t.CertFile != "" && t.KeyFile == "":
		return nil, fmt.Errorf("-%s is set but -%s is not: set both to serve metrics over HTTPS, or neither to serve HTTP",
			certFileFlag, keyFileFlag)
	case t.CertFile == "" && t.KeyFile != "":
		return nil, fmt.Errorf("-%s is set but -%s is not: set both to serve metrics over HTTPS, or neither to serve HTTP",
			keyFileFlag, certFileFlag)
	case t == TLSFiles{}:
		return nil, nil
	}
	cert, err := loadKeyPair(t)
	if err != nil {
		return nil, err
	}
	return &ServingCert{files: t, lastGood: cert}, nil
}

// Start serves handler on port from a background goroutine and returns the
// server for the caller to Close. A listen failure is logged and does not stop
// the daemon.
//
// With a nil cert it serves plain HTTP. Otherwise it serves HTTPS with TLS 1.3
// as the minimum version, and re-reads both files on every new connection, so
// a renewed certificate (for example a cert-manager Secret that kubelet swaps
// atomically) is presented on the next connection without a restart. Existing
// keep-alive connections keep their session. Both modes speak HTTP/1 only.
//
// If a later load fails, the last pair that loaded stays in use, so a bad
// renewal does not fail kubelet's probes and restart the pod. Kubelet does not
// verify the certificate; scrapers that do fail once it expires. Start never
// falls back to plain HTTP for a non-nil cert.
func Start(port int, cert *ServingCert, handler http.Handler) *http.Server {
	// Without the pin, Go negotiates h2 over TLS, which plain HTTP never did.
	var protocols http.Protocols
	protocols.SetHTTP1(true)

	srv := &http.Server{
		Addr:      fmt.Sprintf(":%d", port),
		Handler:   handler,
		ErrorLog:  klog.NewStandardLogger("WARNING"),
		Protocols: &protocols,
	}
	serve := srv.ListenAndServe

	if cert != nil {
		srv.TLSConfig = &tls.Config{
			MinVersion:     tls.VersionTLS13,
			GetCertificate: cert.getCertificate,
			// A resumed session skips GetCertificate and keeps the identity it
			// was issued under for the ticket's lifetime. Without tickets every
			// connection runs a full handshake and sees a renewed pair.
			SessionTicketsDisabled: true,
		}
		serve = func() error { return srv.ListenAndServeTLS("", "") }
	}

	go func() {
		klog.InfoS("http server starting", "port", srv.Addr, "tls", srv.TLSConfig != nil)
		if err := serve(); !errors.Is(err, http.ErrServerClosed) {
			klog.ErrorS(err, "http server failed")
			return
		}
		klog.InfoS("http server stopped")
	}()
	return srv
}

// ServingCert is a certificate pair that has loaded at least once, and the
// files to re-read it from. Get one from TLSFiles.Load. It re-reads the files
// on every handshake and remembers the last pair that loaded. There is no file
// watcher and no goroutine, so a renewed certificate is picked up when the
// next client connects.
type ServingCert struct {
	files TLSFiles
	// load reads the pair; nil means loadKeyPair. Tests replace it to
	// simulate a read that straddles a Secret update.
	load func(TLSFiles) (*tls.Certificate, error)

	// mu is held across the load so that concurrent handshakes cannot store an
	// older pair over a newer one. A load takes well under a millisecond and
	// the endpoint sees few connections.
	mu       sync.Mutex
	lastGood *tls.Certificate
	failed   bool // whether the latest load failed
}

// getCertificate is the tls.Config.GetCertificate. A failed load returns the
// last good pair if there is one, and the error otherwise. It logs only when
// the state changes, so a broken renewal does not log on every scrape.
func (s *ServingCert) getCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	load := s.load
	if load == nil {
		load = loadKeyPair
	}
	cert, err := load(s.files)
	if err != nil {
		// Kubelet updates a Secret volume with a single rename of ..data, so a
		// read that straddles the rename pairs the old cert with the new key.
		// A second read sees one generation.
		cert, err = load(s.files)
	}
	if err != nil {
		// Load always seeds lastGood. A zero ServingCert has no pair to fall
		// back to, so its handshakes fail.
		if s.lastGood == nil {
			s.failed = true
			return nil, err
		}
		if !s.failed {
			klog.ErrorS(err, "failed to reload TLS certificate, serving the last good one")
		}
		s.failed = true
		return s.lastGood, nil
	}
	if s.failed {
		klog.InfoS("TLS certificate reloaded")
	}
	s.failed = false
	s.lastGood = cert
	return cert, nil
}

// loadKeyPair reads and parses both files.
func loadKeyPair(files TLSFiles) (*tls.Certificate, error) {
	cert, err := tls.LoadX509KeyPair(files.CertFile, files.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("load TLS certificate %q and key %q: %w", files.CertFile, files.KeyFile, err)
	}
	return &cert, nil
}

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

package httpserver

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"k8s.io/klog/v2"
)

func TestTLSFilesLoad(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	writeKeyPair(t, a, "a")
	writeKeyPair(t, b, "b")
	notPEM := filepath.Join(a, "not.pem")
	if err := os.WriteFile(notPEM, []byte("not a certificate"), 0o644); err != nil {
		t.Fatal(err)
	}
	crt, key := filepath.Join(a, "tls.crt"), filepath.Join(a, "tls.key")

	tcs := []struct {
		name     string
		files    TLSFiles
		wantCert bool
		wantErr  []string
	}{
		{name: "neither", files: TLSFiles{}},
		{name: "valid pair", files: TLSFiles{CertFile: crt, KeyFile: key}, wantCert: true},
		{name: "cert only", files: TLSFiles{CertFile: crt}, wantErr: []string{"-tls-cert-file is set but -tls-private-key-file is not"}},
		{name: "key only", files: TLSFiles{KeyFile: key}, wantErr: []string{"-tls-private-key-file is set but -tls-cert-file is not"}},
		{name: "missing file", files: TLSFiles{CertFile: filepath.Join(a, "missing.crt"), KeyFile: key}, wantErr: []string{filepath.Join(a, "missing.crt"), key}},
		{name: "not PEM", files: TLSFiles{CertFile: notPEM, KeyFile: key}, wantErr: []string{notPEM, key}},
		{name: "mismatched key", files: TLSFiles{CertFile: crt, KeyFile: filepath.Join(b, "tls.key")}, wantErr: []string{crt, filepath.Join(b, "tls.key")}},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			cert, err := tc.files.Load()
			if (cert != nil) != tc.wantCert {
				t.Fatalf("Load() cert = %v, want non-nil %v", cert, tc.wantCert)
			}
			if len(tc.wantErr) == 0 {
				if err != nil {
					t.Fatalf("Load() error = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatal("Load() error = nil, want an error")
			}
			for _, want := range tc.wantErr {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("Load() error = %q, want it to contain %q", err, want)
				}
			}
		})
	}
}

func TestTLSFilesAddFlags(t *testing.T) {
	var files TLSFiles
	flags := flag.NewFlagSet("test", flag.ContinueOnError)
	files.AddFlags(flags)
	if files != (TLSFiles{}) {
		t.Fatalf("defaults = %+v, want zero", files)
	}
	if err := flags.Parse([]string{"-tls-cert-file=/c", "-tls-private-key-file=/k"}); err != nil {
		t.Fatal(err)
	}
	if want := (TLSFiles{CertFile: "/c", KeyFile: "/k"}); files != want {
		t.Fatalf("parsed = %+v, want %+v", files, want)
	}
}

func TestStartServesPlainHTTP(t *testing.T) {
	port := freePort(t)
	srv := Start(port, nil, okHandler())
	defer srv.Close() // nolint: errcheck

	resp := eventually(t, func() (*http.Response, error) {
		return http.Get(fmt.Sprintf("http://127.0.0.1:%d/", port))
	})
	if body := readBody(t, resp); body != "ok" {
		t.Fatalf("body = %q, want ok", body)
	}
}

func TestStartServesHTTPS(t *testing.T) {
	dir := t.TempDir()
	cert := writeKeyPair(t, dir, "first")
	port := freePort(t)
	srv := Start(port, mustLoad(t, dir), okHandler())
	defer srv.Close() // nolint: errcheck

	roots := x509.NewCertPool()
	roots.AddCert(cert)
	// ForceAttemptHTTP2 makes the client offer h2, which the server must decline.
	client := &http.Client{Transport: &http.Transport{
		TLSClientConfig:   &tls.Config{RootCAs: roots, ServerName: "nfd.test"},
		ForceAttemptHTTP2: true,
	}}
	resp := eventually(t, func() (*http.Response, error) {
		return client.Get(fmt.Sprintf("https://127.0.0.1:%d/", port))
	})
	state := resp.TLS
	if body := readBody(t, resp); body != "ok" {
		t.Fatalf("body = %q, want ok", body)
	}
	if state.Version != tls.VersionTLS13 {
		t.Fatalf("TLS version = %#x, want TLS 1.3", state.Version)
	}
	if state.NegotiatedProtocol != "http/1.1" {
		t.Fatalf("negotiated protocol = %q, want http/1.1", state.NegotiatedProtocol)
	}

	plain, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/", port))
	if err == nil {
		_ = plain.Body.Close()
		if plain.StatusCode == http.StatusOK {
			t.Fatal("plain HTTP request to the TLS port got 200")
		}
	}
}

func TestStartReloadsRenewedCertificate(t *testing.T) {
	mount, swap := secretVolume(t)
	port := freePort(t)
	srv := Start(port, mustLoad(t, mount), okHandler())
	defer srv.Close() // nolint: errcheck
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	if cn, err := servedCommonName(t, addr, 0); err != nil || cn != "first" {
		t.Fatalf("before renewal: cn=%q err=%v, want first", cn, err)
	}
	swap(func(dir string) { writeKeyPair(t, dir, "second") })
	if cn, err := servedCommonName(t, addr, 0); err != nil || cn != "second" {
		t.Fatalf("after renewal: cn=%q err=%v, want second", cn, err)
	}

	other := t.TempDir()
	writeKeyPair(t, other, "other")
	swap(func(dir string) {
		writeKeyPair(t, dir, "broken")
		copyFile(t, filepath.Join(other, "tls.key"), filepath.Join(dir, "tls.key"))
	})
	if cn, err := servedCommonName(t, addr, 0); err != nil || cn != "second" {
		t.Fatalf("mismatched pair: cn=%q err=%v, want the last good pair second", cn, err)
	}
	swap(func(dir string) {})
	if cn, err := servedCommonName(t, addr, 0); err != nil || cn != "second" {
		t.Fatalf("unreadable pair: cn=%q err=%v, want the last good pair second", cn, err)
	}
	swap(func(dir string) { writeKeyPair(t, dir, "third") })
	if cn, err := servedCommonName(t, addr, 0); err != nil || cn != "third" {
		t.Fatalf("after repair: cn=%q err=%v, want third", cn, err)
	}
}

func TestStartServesGoodPairToConcurrentHandshakesDuringSwaps(t *testing.T) {
	mount, swap := secretVolume(t)
	port := freePort(t)
	srv := Start(port, mustLoad(t, mount), okHandler())
	defer srv.Close() // nolint: errcheck
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	if cn, err := servedCommonName(t, addr, 0); err != nil || cn != "first" {
		t.Fatalf("before swaps: cn=%q err=%v, want first", cn, err)
	}

	// The swaps run on the test goroutine, so the handshakers report back
	// over a channel instead of calling t.Fatal.
	const clients, connections = 8, 5
	results := make(chan string, clients*connections)
	var wg sync.WaitGroup
	for range clients {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range connections {
				cn, err := dialCommonName(addr, 0)
				if err != nil {
					results <- "error: " + err.Error()
					continue
				}
				results <- cn
			}
		}()
	}
	for i := range 10 {
		if i%2 == 0 {
			swap(func(dir string) { writeKeyPair(t, dir, "good") })
		} else {
			swap(func(dir string) {})
		}
	}
	wg.Wait()
	close(results)

	for got := range results {
		if got != "first" && got != "good" {
			t.Fatalf("handshake result = %q, want a successful handshake with first or good", got)
		}
	}
}

func TestStartRequiresTLS13(t *testing.T) {
	dir := t.TempDir()
	writeKeyPair(t, dir, "first")
	port := freePort(t)
	srv := Start(port, mustLoad(t, dir), okHandler())
	defer srv.Close() // nolint: errcheck
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	if _, err := servedCommonName(t, addr, 0); err != nil {
		t.Fatalf("default client: %v", err)
	}
	if _, err := servedCommonName(t, addr, tls.VersionTLS12); err == nil {
		t.Fatal("a TLS 1.2 client completed the handshake")
	}
}

func TestStartPresentsLoadedPairWhenFilesBreakBeforeFirstHandshake(t *testing.T) {
	mount, swap := secretVolume(t)
	cert := mustLoad(t, mount)

	other := t.TempDir()
	writeKeyPair(t, other, "other")
	swap(func(dir string) {
		writeKeyPair(t, dir, "broken")
		copyFile(t, filepath.Join(other, "tls.key"), filepath.Join(dir, "tls.key"))
	})

	port := freePort(t)
	srv := Start(port, cert, okHandler())
	defer srv.Close() // nolint: errcheck

	if cn, err := servedCommonName(t, fmt.Sprintf("127.0.0.1:%d", port), 0); err != nil || cn != "first" {
		t.Fatalf("first connection: cn=%q err=%v, want the pair Load read, first", cn, err)
	}
}

func TestStartPresentsRenewedCertificateToSessionCachingClient(t *testing.T) {
	mount, swap := secretVolume(t)
	port := freePort(t)
	srv := Start(port, mustLoad(t, mount), okHandler())
	defer srv.Close() // nolint: errcheck

	// One client for both requests, so a session ticket from the first
	// connection would be offered on the second.
	client := &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{ // nolint: gosec
			InsecureSkipVerify: true,
			ClientSessionCache: tls.NewLRUClientSessionCache(0),
		},
		DisableKeepAlives: true,
	}}
	url := fmt.Sprintf("https://127.0.0.1:%d/", port)
	connect := func() tls.ConnectionState {
		resp := eventually(t, func() (*http.Response, error) { return client.Get(url) })
		state := *resp.TLS
		readBody(t, resp)
		return state
	}

	if cn := connect().PeerCertificates[0].Subject.CommonName; cn != "first" {
		t.Fatalf("before renewal: cn=%q, want first", cn)
	}
	swap(func(dir string) { writeKeyPair(t, dir, "second") })
	state := connect()
	if state.DidResume {
		t.Fatal("the connection after renewal resumed a session, want a full handshake")
	}
	if cn := state.PeerCertificates[0].Subject.CommonName; cn != "second" {
		t.Fatalf("after renewal: cn=%q, want second", cn)
	}
}

func TestServingCertRereadsATornRead(t *testing.T) {
	first, second := loadedPair(t, "first"), loadedPair(t, "second")
	torn := errors.New("private key does not match public key")

	t.Run("second read succeeds", func(t *testing.T) {
		reads := 0
		s := &ServingCert{lastGood: first, load: func(TLSFiles) (*tls.Certificate, error) {
			reads++
			if reads == 1 {
				return nil, torn
			}
			return second, nil
		}}
		if got, err := s.getCertificate(nil); err != nil || commonName(got) != "second" {
			t.Fatalf("getCertificate() = %s, %v, want the renewed pair second", commonName(got), err)
		}
	})
	t.Run("both reads fail", func(t *testing.T) {
		s := &ServingCert{lastGood: first, load: func(TLSFiles) (*tls.Certificate, error) { return nil, torn }}
		if got, err := s.getCertificate(nil); err != nil || commonName(got) != "first" {
			t.Fatalf("getCertificate() = %s, %v, want the last good pair first", commonName(got), err)
		}
	})
}

func TestZeroServingCertHasNoPair(t *testing.T) {
	if got, err := (&ServingCert{}).getCertificate(nil); err == nil || got != nil {
		t.Fatalf("getCertificate() = %s, %v, want no pair and an error", commonName(got), err)
	}
}

func TestStartBindFailureIsNotFatal(t *testing.T) {
	busy, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer busy.Close() // nolint: errcheck

	srv := Start(busy.Addr().(*net.TCPAddr).Port, nil, okHandler())
	if srv == nil {
		t.Fatal("Start returned nil for a port in use")
	}
	// Give the serve goroutine time to hit the bind error.
	time.Sleep(100 * time.Millisecond)
	if err := srv.Close(); err != nil {
		t.Fatalf("Close() = %v, want nil", err)
	}
}

// A connection that closes before the handshake, as a TCP health check does,
// is logged at WARNING. A connection that completes the handshake is not.
func TestStartLogsFailedHandshakesAtWarning(t *testing.T) {
	dir := t.TempDir()
	writeKeyPair(t, dir, "first")
	port := freePort(t)
	srv := Start(port, mustLoad(t, dir), okHandler())
	defer srv.Close() // nolint: errcheck
	// The serve goroutine logs when srv.Close stops it. Capturing klog after
	// Start makes the restore run before that, so the two cannot race.
	state := klog.CaptureState()
	defer state.Restore()
	klog.LogToStderr(false)
	var output lockedBuffer
	klog.SetOutput(&output)
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	if _, err := servedCommonName(t, addr, 0); err != nil {
		t.Fatal(err)
	}

	completed, err := tls.Dial("tcp", addr, &tls.Config{InsecureSkipVerify: true}) // nolint: gosec
	if err != nil {
		t.Fatal(err)
	}
	completedFrom := completed.LocalAddr().String()
	_ = completed.Close()

	var failedFrom []string
	for range 3 {
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			t.Fatal(err)
		}
		failedFrom = append(failedFrom, conn.LocalAddr().String())
		_ = conn.Close()
	}

	// Connection goroutines write the log lines, so wait for them.
	for _, from := range failedFrom {
		var line string
		for range 100 {
			if line = output.handshakeError(from); line != "" {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		if !strings.HasPrefix(line, "W") || !strings.HasSuffix(line, ": EOF") {
			t.Fatalf("log for the connection from %s = %q, want a WARNING line ending in EOF", from, line)
		}
	}
	if line := output.handshakeError(completedFrom); line != "" {
		t.Fatalf("completed handshake from %s was logged: %q", completedFrom, line)
	}
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "ok") })
}

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close() // nolint: errcheck
	return l.Addr().(*net.TCPAddr).Port
}

// lockedBuffer collects klog output that server goroutines write while the
// test reads it.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

// handshakeError returns the first logged TLS handshake error for the client
// at addr, or "" if there is none.
func (b *lockedBuffer) handshakeError(addr string) string {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, line := range strings.Split(b.buf.String(), "\n") {
		if strings.Contains(line, "TLS handshake error from "+addr+":") {
			return line
		}
	}
	return ""
}

// mustLoad loads tls.crt and tls.key from dir.
func mustLoad(t *testing.T, dir string) *ServingCert {
	t.Helper()
	cert, err := TLSFiles{CertFile: filepath.Join(dir, "tls.crt"), KeyFile: filepath.Join(dir, "tls.key")}.Load()
	if err != nil {
		t.Fatal(err)
	}
	return cert
}

// loadedPair returns a parsed self-signed pair for commonName.
func loadedPair(t *testing.T, commonName string) *tls.Certificate {
	t.Helper()
	dir := t.TempDir()
	writeKeyPair(t, dir, commonName)
	cert, err := loadKeyPair(TLSFiles{CertFile: filepath.Join(dir, "tls.crt"), KeyFile: filepath.Join(dir, "tls.key")})
	if err != nil {
		t.Fatal(err)
	}
	return cert
}

// commonName returns the common name of the leaf certificate in c.
func commonName(c *tls.Certificate) string {
	if c == nil {
		return "<nil>"
	}
	leaf, err := x509.ParseCertificate(c.Certificate[0])
	if err != nil {
		return err.Error()
	}
	return leaf.Subject.CommonName
}

// eventually retries until the server goroutine is listening.
func eventually(t *testing.T, get func() (*http.Response, error)) *http.Response {
	t.Helper()
	var err error
	for range 100 {
		var resp *http.Response
		if resp, err = get(); err == nil {
			return resp
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal(err)
	return nil
}

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close() // nolint: errcheck
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// servedCommonName opens a new connection and returns the common name of the
// certificate the server presents. It retries while the server starts.
func servedCommonName(t *testing.T, addr string, maxVersion uint16) (string, error) {
	t.Helper()
	var cn string
	var err error
	for range 100 {
		cn, err = dialCommonName(addr, maxVersion)
		if err == nil || !strings.Contains(err.Error(), "connection refused") {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	return cn, err
}

// dialCommonName makes one connection and returns the common name of the
// certificate the server presents. It is safe to call from any goroutine.
func dialCommonName(addr string, maxVersion uint16) (string, error) {
	conn, err := tls.Dial("tcp", addr, &tls.Config{InsecureSkipVerify: true, MaxVersion: maxVersion}) // nolint: gosec
	if err != nil {
		return "", err
	}
	defer conn.Close() // nolint: errcheck
	return conn.ConnectionState().PeerCertificates[0].Subject.CommonName, nil
}

// writeKeyPair writes a self-signed ECDSA pair for nfd.test as tls.crt and
// tls.key in dir.
func writeKeyPair(t *testing.T, dir, commonName string) *x509.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: commonName},
		DNSNames:     []string{"nfd.test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tls.crt"), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tls.key"), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return cert
}

// secretVolume lays files out the way kubelet writes a Secret volume:
// tls.crt and tls.key are symlinks through ..data, and an update writes a new
// directory and swaps ..data with a rename.
func secretVolume(t *testing.T) (mount string, swap func(write func(dir string))) {
	t.Helper()
	mount = t.TempDir()
	generation := 0
	swap = func(write func(dir string)) {
		generation++
		dir := filepath.Join(mount, "..gen"+strconv.Itoa(generation))
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		write(dir)
		tmp := filepath.Join(mount, "..data_tmp")
		if err := os.Symlink(filepath.Base(dir), tmp); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(tmp, filepath.Join(mount, "..data")); err != nil {
			t.Fatal(err)
		}
	}
	swap(func(dir string) { writeKeyPair(t, dir, "first") })
	for _, name := range []string{"tls.crt", "tls.key"} {
		if err := os.Symlink(filepath.Join("..data", name), filepath.Join(mount, name)); err != nil {
			t.Fatal(err)
		}
	}
	return mount, swap
}

func copyFile(t *testing.T, from, to string) {
	t.Helper()
	b, err := os.ReadFile(from)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(to, b, 0o600); err != nil {
		t.Fatal(err)
	}
}

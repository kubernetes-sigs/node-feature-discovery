/*
Copyright 2021 The Kubernetes Authors.

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

package nfdclient

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"k8s.io/klog/v2"

	"sigs.k8s.io/node-feature-discovery/pkg/utils"
)

// NfdClient defines a common interface for NFD clients.
type NfdClient interface {
	Run() error
	Stop()
}

// NfdBaseClient is a common base type for handling connections to nfd-master.
type NfdBaseClient struct {
	args       Args
	clientConn *grpc.ClientConn
}

// Args holds the common command line arguments for all nfd clients.
type Args struct {
	CaFile             string
	CertFile           string
	KeyFile            string
	Kubeconfig         string
	Server             string
	ServerNameOverride string

	Klog map[string]*utils.KlogFlagVal
}

var nodeName string

func init() {
	nodeName = os.Getenv("NODE_NAME")
}

// NodeName returns the name of the k8s node we're running on.
func NodeName() string { return nodeName }

// NewNfdBaseClient creates a new NfdBaseClient instance.
func NewNfdBaseClient(args *Args) (NfdBaseClient, error) {
	nfd := NfdBaseClient{args: *args}

	// Check TLS related args
	if args.CertFile != "" || args.KeyFile != "" || args.CaFile != "" {
		if args.CertFile == "" {
			return nfd, fmt.Errorf("-cert-file needs to be specified alongside -key-file and -ca-file")
		}
		if args.KeyFile == "" {
			return nfd, fmt.Errorf("-key-file needs to be specified alongside -cert-file and -ca-file")
		}
		if args.CaFile == "" {
			return nfd, fmt.Errorf("-ca-file needs to be specified alongside -cert-file and -key-file")
		}
	}

	return nfd, nil
}

// ClientConn returns the grpc ClientConn object.
func (w *NfdBaseClient) ClientConn() *grpc.ClientConn { return w.clientConn }

// Connect creates a gRPC client connection to nfd-master.
func (w *NfdBaseClient) Connect() error {
	// Check that if a connection already exists
	if w.clientConn != nil {
		return fmt.Errorf("client connection already exists")
	}

	// Dial and create a client
	dialCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	dialOpts := []grpc.DialOption{grpc.WithBlock()}
	if w.args.CaFile != "" || w.args.CertFile != "" || w.args.KeyFile != "" {
		// Load client cert for client authentication
		cert, err := tls.LoadX509KeyPair(w.args.CertFile, w.args.KeyFile)
		if err != nil {
			return fmt.Errorf("failed to load client certificate: %v", err)
		}
		// Load CA cert for server cert verification
		caCert, err := os.ReadFile(w.args.CaFile)
		if err != nil {
			return fmt.Errorf("failed to read root certificate file: %v", err)
		}
		caPool := x509.NewCertPool()
		if ok := caPool.AppendCertsFromPEM(caCert); !ok {
			return fmt.Errorf("failed to add certificate from '%s'", w.args.CaFile)
		}
		// Create TLS config
		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
			RootCAs:      caPool,
			ServerName:   w.args.ServerNameOverride,
			MinVersion:   tls.VersionTLS13,
		}
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	} else {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	klog.Infof("connecting to nfd-master at %s ...", w.args.Server)
	conn, err := grpc.DialContext(dialCtx, w.args.Server, dialOpts...)
	if err != nil {
		return err
	}
	w.clientConn = conn

	return nil
}

// Disconnect closes the connection to NFD master
func (w *NfdBaseClient) Disconnect() {
	if w.clientConn != nil {
		klog.Infof("closing connection to nfd-master ...")
		w.clientConn.Close()
	}
	w.clientConn = nil
}

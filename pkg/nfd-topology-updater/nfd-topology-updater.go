/*
Copyright 2020 The Kubernetes Authors.

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

package nfdtopologyupdater

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"time"

	v1alpha1 "github.com/k8stopologyawareschedwg/noderesourcetopology-api/pkg/apis/topology/v1alpha1"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"sigs.k8s.io/node-feature-discovery/pkg/dumpobject"
	pb "sigs.k8s.io/node-feature-discovery/pkg/topologyupdater"
	"sigs.k8s.io/node-feature-discovery/pkg/version"
)

var (
	stdoutLogger = log.New(os.Stdout, "", log.LstdFlags)
	stderrLogger = log.New(os.Stderr, "", log.LstdFlags)
	nodeName     = os.Getenv("NODE_NAME")
)

// Command line arguments
type Args struct {
	CaFile             string
	CertFile           string
	KeyFile            string
	NoPublish          bool
	Oneshot            bool
	Server             string
	ServerNameOverride string
}

type NfdTopologyUpdater interface {
	Update(v1alpha1.ZoneList) error
}

type nfdTopologyUpdater struct {
	args       Args
	clientConn *grpc.ClientConn
	client     pb.NodeTopologyClient
	tmPolicy   string
}

// Create new NewTopologyUpdater instance.
func NewTopologyUpdater(args Args, policy string) (NfdTopologyUpdater, error) {
	nfd := &nfdTopologyUpdater{
		args:     args,
		tmPolicy: policy,
	}

	// Check TLS related args
	if args.CertFile != "" || args.KeyFile != "" || args.CaFile != "" {
		if args.CertFile == "" {
			return nfd, fmt.Errorf("--cert-file needs to be specified alongside --key-file and --ca-file")
		}
		if args.KeyFile == "" {
			return nfd, fmt.Errorf("--key-file needs to be specified alongside --cert-file and --ca-file")
		}
		if args.CaFile == "" {
			return nfd, fmt.Errorf("--ca-file needs to be specified alongside --cert-file and --key-file")
		}
	}

	return nfd, nil
}

// Run nfdTopologyUpdater client. Returns if a fatal error is encountered, or, after
// one request if OneShot is set to 'true' in the worker args.
func (w *nfdTopologyUpdater) Update(zones v1alpha1.ZoneList) error {
	stdoutLogger.Printf("Node Feature Discovery Topology Updater %s", version.Get())
	stdoutLogger.Printf("NodeName: '%s'", nodeName)
	stdoutLogger.Printf("Updating now received Zone: '%s'", dumpobject.DumpObject(zones))

	// Connect to NFD master
	err := w.connect()
	if err != nil {
		return fmt.Errorf("failed to connect: %v", err)
	}
	defer w.disconnect()

	if w.client == nil {
		return nil
	}

	err = advertiseNodeTopology(w.client, zones, w.tmPolicy)
	if err != nil {
		return fmt.Errorf("failed to advertise node topology: %s", err.Error())
	}

	return nil
}

// connect creates a client connection to the NFD master
func (w *nfdTopologyUpdater) connect() error {
	// Return a dummy connection in case of dry-run
	if w.args.NoPublish {
		return nil
	}

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
		caCert, err := ioutil.ReadFile(w.args.CaFile)
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
		}
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	} else {
		dialOpts = append(dialOpts, grpc.WithInsecure())
	}
	conn, err := grpc.DialContext(dialCtx, w.args.Server, dialOpts...)
	if err != nil {
		return err
	}
	w.clientConn = conn
	w.client = pb.NewNodeTopologyClient(conn)

	return nil
}

// disconnect closes the connection to NFD master
func (w *nfdTopologyUpdater) disconnect() {
	if w.clientConn != nil {
		w.clientConn.Close()
	}
	w.clientConn = nil
	w.client = nil
}

// advertiseNodeTopology advertises the topology CRD to a Kubernetes node
// via the NFD server.
func advertiseNodeTopology(client pb.NodeTopologyClient, zoneInfo v1alpha1.ZoneList, tmPolicy string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	zones := make([]*pb.Zone, 0)
	for _, zone := range zoneInfo {
		resInfo := make([]*pb.ResourceInfo, 0)
		for _, info := range zone.Resources {
			resInfo = append(resInfo, &pb.ResourceInfo{
				Name:        info.Name,
				Allocatable: info.Allocatable.String(),
				Capacity:    info.Capacity.String(),
			})
		}

		zones = append(zones, &pb.Zone{
			Name:      zone.Name,
			Type:      zone.Type,
			Resources: resInfo,
			Costs:     updateMap(zone.Costs),
		})
	}

	topologyReq := &pb.NodeTopologyRequest{
		Zones:            zones,
		NfdVersion:       version.Get(),
		NodeName:         nodeName,
		TopologyPolicies: []string{tmPolicy},
	}
	stdoutLogger.Printf("Sending NodeTopologyRequest to nfd-master: %v", dumpobject.DumpObject(topologyReq))

	_, err := client.UpdateNodeTopology(ctx, topologyReq)
	if err != nil {
		stderrLogger.Printf("failed to set node topology CRD: %v", err)
		return err
	}

	return nil
}
func updateMap(data []v1alpha1.CostInfo) []*pb.CostInfo {
	ret := make([]*pb.CostInfo, 0)
	for _, cost := range data {
		ret = append(ret, &pb.CostInfo{
			Name:  cost.Name,
			Value: int32(cost.Value),
		})
	}
	return ret
}

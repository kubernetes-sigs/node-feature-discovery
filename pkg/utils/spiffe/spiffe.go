/*
Copyright 2024 The Kubernetes Authors.
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

package spiffe

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	b64 "encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/spiffe/go-spiffe/v2/workloadapi"

	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/api/nfd/v1alpha1"
)

type SpiffeObject struct {
	Spec      nfdv1alpha1.NodeFeatureSpec
	Name      string
	Namespace string
	Labels    map[string]string
}

// WorkerSpiffeID is the SpiffeID of the worker
const WorkerSpiffeID = "spiffe://nfd.k8s-sigs.io/worker"

type SpiffeClient struct {
	WorkloadApiClient workloadapi.Client
}

var latestSignature []byte
var latestHash [32]byte

func NewSpiffeClient(socketPath string) (*SpiffeClient, error) {
	spiffeClient := SpiffeClient{}
	workloadApiClient, err := workloadapi.New(context.Background(), workloadapi.WithAddr(socketPath))
	if err != nil {
		return nil, err
	}
	spiffeClient.WorkloadApiClient = *workloadApiClient
	return &spiffeClient, nil
}

func SignData(data SpiffeObject, privateKey crypto.Signer) ([]byte, error) {
	stringifyData, err := json.Marshal(data)
	if err != nil {
		return []byte{}, err
	}

	dataHash := sha256.Sum256([]byte(stringifyData))
	if dataHash == latestHash && len(latestSignature) > 0 {
		return latestSignature, nil
	}

	var signedData []byte
	switch t := privateKey.(type) {
	case *rsa.PrivateKey:
		signedData, err = rsa.SignPKCS1v15(rand.Reader, privateKey.(*rsa.PrivateKey), crypto.SHA256, dataHash[:])
		if err != nil {
			return []byte{}, err
		}
	case *ecdsa.PrivateKey:
		signedData, err = ecdsa.SignASN1(rand.Reader, privateKey.(*ecdsa.PrivateKey), dataHash[:])
		if err != nil {
			return []byte{}, err
		}
	default:
		return nil, fmt.Errorf("unknown private key type: %v", t)
	}

	latestSignature = signedData
	latestHash = dataHash
	return signedData, nil
}

func VerifyDataSignature(data SpiffeObject, signedData string, privateKey crypto.Signer, publicKey crypto.PublicKey) (bool, error) {
	stringifyData, err := json.Marshal(data)
	if err != nil {
		return false, err
	}

	decodedSignature, err := b64.StdEncoding.DecodeString(signedData)
	if err != nil {
		return false, err
	}

	dataHash := sha256.Sum256([]byte(stringifyData))

	switch t := privateKey.(type) {
	case *rsa.PrivateKey:
		err = rsa.VerifyPKCS1v15(publicKey.(*rsa.PublicKey), crypto.SHA256, dataHash[:], decodedSignature)
		if err != nil {
			return false, err
		}
		return true, nil
	case *ecdsa.PrivateKey:
		verify := ecdsa.VerifyASN1(publicKey.(*ecdsa.PublicKey), dataHash[:], decodedSignature)
		return verify, nil
	default:
		return false, fmt.Errorf("unknown private key type: %v", t)
	}
}

func (s *SpiffeClient) GetWorkerKeys() (crypto.Signer, crypto.PublicKey, error) {
	ctx := context.Background()

	svids, err := s.WorkloadApiClient.FetchX509SVIDs(ctx)
	if err != nil {
		return nil, nil, err
	}

	for _, svid := range svids {
		if svid.ID.String() == WorkerSpiffeID {
			return svid.PrivateKey, svid.PrivateKey.Public(), nil
		}
	}

	return nil, nil, fmt.Errorf("cannot sign data: spiffe ID %s is not found", WorkerSpiffeID)
}

func (s *SpiffeClient) InvalidateCache() {
	latestSignature = []byte{}
	latestHash = [32]byte{}
}

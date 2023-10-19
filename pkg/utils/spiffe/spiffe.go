/*
Copyright 2023 The Kubernetes Authors.

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

package utils

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"github.com/spiffe/go-spiffe/v2/workloadapi"
)

type SpiffeClient struct {
	WorkloadApiClient workloadapi.Client
}

func NewSpiffeClient(socketPath string) (*SpiffeClient, error) {
	spiffeClient := SpiffeClient{}
	workloadApiClient, err := workloadapi.New(context.Background(), workloadapi.WithAddr(socketPath))
	if err != nil {
		return nil, err
	}
	spiffeClient.WorkloadApiClient = *workloadApiClient
	return &spiffeClient, nil
}

func GetSpiffeId(nodeName string) string {
	return fmt.Sprintf("spiffe://nfd.com/%s", nodeName)
}

func (s *SpiffeClient) SignData(data interface{}, spiffeId string) ([]byte, error) {
	ctx := context.Background()
	svids, err := s.WorkloadApiClient.FetchX509SVIDs(ctx)
	if err != nil {
		return []byte{}, nil
	}

	stringifyData, err := json.Marshal(data)
	if err != nil {
		return []byte{}, err
	}

	dataHash := sha256.Sum256([]byte(stringifyData))
	for _, svid := range svids {
		if svid.ID.String() == spiffeId {
			privateKey := svid.PrivateKey
			switch t := privateKey.(type) {
			case *rsa.PrivateKey:
				signedData, err := rsa.SignPKCS1v15(rand.Reader, privateKey.(*rsa.PrivateKey), crypto.SHA256, dataHash[:])
				if err != nil {
					return []byte{}, err
				}
				return signedData, nil
			case *ecdsa.PrivateKey:
				signedData, err := ecdsa.SignASN1(rand.Reader, privateKey.(*ecdsa.PrivateKey), dataHash[:])
				if err != nil {
					return []byte{}, err
				}

				return signedData, nil
			default:
				return nil, fmt.Errorf("unknown private key type: %v", t)
			}
		}
	}

	return nil, fmt.Errorf("cannot sign data: spiffe ID %s is not found", spiffeId)
}

func (s *SpiffeClient) VerifyDataSignature(data interface{}, spiffeId string, signedData []byte) (bool, error) {
	ctx := context.Background()
	svids, err := s.WorkloadApiClient.FetchX509SVIDs(ctx)
	if err != nil {
		return false, nil
	}

	stringifyData, err := json.Marshal(data)
	if err != nil {
		return false, err
	}

	dataHash := sha256.Sum256([]byte(stringifyData))
	for _, svid := range svids {
		if svid.ID.String() == spiffeId {
			privateKey := svid.PrivateKey
			switch t := privateKey.(type) {
			case *rsa.PrivateKey:
				err = rsa.VerifyPKCS1v15(svid.PrivateKey.Public().(*rsa.PublicKey), crypto.SHA256, dataHash[:], signedData)
				if err != nil {
					return false, err
				}
				return true, nil
			case *ecdsa.PrivateKey:
				verify := ecdsa.VerifyASN1(svid.PrivateKey.Public().(*ecdsa.PublicKey), dataHash[:], signedData)
				return verify, nil
			default:
				return false, fmt.Errorf("unknown private key type: %v", t)
			}
		}
	}

	return false, fmt.Errorf("cannot sign data: spiffe ID %s is not found", spiffeId)
}

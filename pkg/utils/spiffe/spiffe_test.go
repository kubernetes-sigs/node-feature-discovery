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

package utils

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	b64 "encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/node-feature-discovery/api/nfd/v1alpha1"
)

func mockNFRSpec() v1alpha1.NodeFeatureSpec {
	return v1alpha1.NodeFeatureSpec{
		Features: v1alpha1.Features{
			Flags: map[string]v1alpha1.FlagFeatureSet{
				"test": {
					Elements: map[string]v1alpha1.Nil{
						"test2": {},
					},
				},
			},
		},
	}
}

func mockWorkerECDSAPrivateKey() (*ecdsa.PrivateKey, *ecdsa.PublicKey) {
	privateKey, _ := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	return privateKey, &privateKey.PublicKey
}

func mockWorkerRSAPrivateKey() (*rsa.PrivateKey, *rsa.PublicKey) {
	privateKey, _ := rsa.GenerateKey(rand.Reader, 4096)
	return privateKey, &privateKey.PublicKey
}

func TestVerify(t *testing.T) {
	rsaPrivateKey, rsaPublicKey := mockWorkerRSAPrivateKey()
	ecdsaPrivateKey, ecdsaPublicKey := mockWorkerECDSAPrivateKey()
	spec := mockNFRSpec()

	tc := []struct {
		name       string
		privateKey crypto.Signer
		publicKey  crypto.PublicKey
		wantErr    bool
	}{
		{
			name:       "RSA Keys",
			privateKey: rsaPrivateKey,
			publicKey:  rsaPublicKey,
			wantErr:    true,
		},
		{
			name:       "ECDSA Keys",
			privateKey: ecdsaPrivateKey,
			publicKey:  ecdsaPublicKey,
			wantErr:    false,
		},
	}

	for _, tt := range tc {
		signedData, err := SignData(spec, tt.privateKey)
		assert.NoError(t, err)

		isVerified, err := VerifyDataSignature(spec, b64.StdEncoding.EncodeToString(signedData), tt.privateKey, tt.publicKey)
		assert.NoError(t, err)
		assert.True(t, isVerified)

		signedData = append(signedData, "random"...)
		isVerified, err = VerifyDataSignature(spec, b64.StdEncoding.EncodeToString(signedData), tt.privateKey, tt.publicKey)
		if tt.wantErr {
			assert.Error(t, err)
		} else {
			assert.False(t, isVerified)
		}
	}
}

func TestSignData(t *testing.T) {
	rsaPrivateKey, _ := mockWorkerRSAPrivateKey()
	ecdsaPrivateKey, _ := mockWorkerECDSAPrivateKey()
	spec := mockNFRSpec()

	tc := []struct {
		name       string
		privateKey crypto.Signer
	}{
		{
			name:       "RSA Keys",
			privateKey: rsaPrivateKey,
		},
		{
			name:       "ECDSA Keys",
			privateKey: ecdsaPrivateKey,
		},
	}

	for _, tt := range tc {
		_, err := SignData(spec, tt.privateKey)
		assert.NoError(t, err)
	}
}

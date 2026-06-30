/*
Copyright (c) 2023-2026 Microbus LLC and various contributors

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

package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"strings"
)

// signingKey ensures config.local.yaml carries a freshly generated Ed25519
// signing key for bearer.token.core. Generating the key in Go avoids depending
// on openssl, whose default build on macOS lacks the Ed25519 algorithm.
func (g *generator) signingKey() error {
	rel := "config.local.yaml"
	existing := g.read(rel)
	if strings.Contains(existing, "bearer.token.core") {
		fmt.Println("  signing key already configured: bearer.token.core")
		return nil
	}
	key, err := generateSigningKey()
	if err != nil {
		return err
	}
	block := fmt.Sprintf("\nbearer.token.core:\n  PrivateKey: %s\n", key)
	sep := ""
	if existing != "" && !strings.HasSuffix(existing, "\n") {
		sep = "\n"
	}
	return g.write(rel, existing+sep+block, "added signing key to")
}

// generateSigningKey returns a base64-encoded PKCS#8 DER Ed25519 private key,
// the form bearer.token.core's PrivateKey config accepts.
func generateSigningKey() (string, error) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", err
	}
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(der), nil
}

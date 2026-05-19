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

package httpx

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/microbus-io/testarossa"
)

// writePEMPair generates a self-signed certificate and writes it to certPath/keyPath.
func writePEMPair(t *testing.T, certPath, keyPath string, notBefore time.Time, dnsNames ...string) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(notBefore.UnixNano()),
		Subject:               pkix.Name{CommonName: dnsNames[0]},
		NotBefore:             notBefore,
		NotAfter:              notBefore.Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:              dnsNames,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, pub, priv)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	err = os.WriteFile(certPath, certPEM, 0o600)
	if err != nil {
		t.Fatalf("write cert: %v", err)
	}
	err = os.WriteFile(keyPath, keyPEM, 0o600)
	if err != nil {
		t.Fatalf("write key: %v", err)
	}
}

// writeTokenCert writes a {token}-cert.pem + -key.pem pair into dir.
func writeTokenCert(t *testing.T, dir, token string, notBefore time.Time, dnsNames ...string) {
	t.Helper()
	writePEMPair(t,
		filepath.Join(dir, token+certFileSuffix),
		filepath.Join(dir, token+keyFileSuffix),
		notBefore, dnsNames...)
}

// writeDefaultCert writes the server-wide default pair (no token).
func writeDefaultCert(t *testing.T, dir string, notBefore time.Time, dnsNames ...string) {
	t.Helper()
	writePEMPair(t,
		filepath.Join(dir, serverDefaultCertName),
		filepath.Join(dir, serverDefaultKeyName),
		notBefore, dnsNames...)
}

func TestHttpx_CertStoreExactAndWildcard(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)
	dir := t.TempDir()
	now := time.Now().Add(-time.Hour)
	writeTokenCert(t, dir, "app", now, "app.example.com")
	writeTokenCert(t, dir, "wild", now, "*.wild.example.com")

	cs := NewCertStore(dir, nil)
	cs.Load(context.Background())

	// Exact SAN match.
	c, err := cs.Get(443, "app.example.com")
	if assert.NoError(err) {
		assert.Equal("app.example.com", c.Leaf.DNSNames[0])
	}

	// Wildcard replaces exactly one label.
	c, err = cs.Get(443, "anything.wild.example.com")
	if assert.NoError(err) {
		assert.Equal("*.wild.example.com", c.Leaf.DNSNames[0])
	}

	// The wildcard does not match the bare parent, and there is no port default.
	_, err = cs.Get(443, "wild.example.com")
	assert.Error(err)

	// Unknown name with no port default fails the handshake.
	_, err = cs.Get(443, "no.match.test")
	assert.Error(err)
}

func TestHttpx_CertStoreExactBeatsWildcard(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)
	dir := t.TempDir()
	now := time.Now().Add(-time.Hour)
	writeTokenCert(t, dir, "wildcard", now, "*.example.com")
	writeTokenCert(t, dir, "exact", now, "api.example.com")

	cs := NewCertStore(dir, nil)
	cs.Load(context.Background())

	c, err := cs.Get(443, "api.example.com")
	if assert.NoError(err) {
		assert.Equal("api.example.com", c.Leaf.DNSNames[0])
	}

	// A different sub-domain still falls through to the wildcard.
	c, err = cs.Get(443, "other.example.com")
	if assert.NoError(err) {
		assert.Equal("*.example.com", c.Leaf.DNSNames[0])
	}
}

func TestHttpx_CertStorePortDefaultFallback(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)
	dir := t.TempDir()
	now := time.Now().Add(-time.Hour)
	// A purely numeric token registers the per-listener default for that port.
	writeTokenCert(t, dir, "443", now, "default.example.com")

	cs := NewCertStore(dir, nil)
	cs.Load(context.Background())

	// No SNI match, but the :443 listener has a port-named default.
	c, err := cs.Get(443, "unknown.host")
	if assert.NoError(err) {
		assert.Equal("default.example.com", c.Leaf.DNSNames[0])
	}

	// A different listener port has no default and fails closed.
	_, err = cs.Get(8443, "unknown.host")
	assert.Error(err)

	// The default cert is still reachable by its SAN.
	_, err = cs.Get(8443, "default.example.com")
	assert.NoError(err)
}

func TestHttpx_CertStoreServerDefaultFallback(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)
	dir := t.TempDir()
	now := time.Now().Add(-time.Hour)
	// The server-wide default has no token in its filename.
	writeDefaultCert(t, dir, now, "server-default.example.com")
	// A port-named cert: takes precedence over the server default for its own port.
	writeTokenCert(t, dir, "443", now, "port-default.example.com")

	cs := NewCertStore(dir, nil)
	cs.Load(context.Background())

	// Unknown SNI on a listener with a port-named default returns the port default, not the
	// server-wide default.
	c, err := cs.Get(443, "unknown.host")
	if assert.NoError(err) {
		assert.Equal("port-default.example.com", c.Leaf.DNSNames[0])
	}

	// Unknown SNI on a listener without a port-named default falls through to the server-wide
	// default.
	c, err = cs.Get(8443, "unknown.host")
	if assert.NoError(err) {
		assert.Equal("server-default.example.com", c.Leaf.DNSNames[0])
	}

	// The server default's SAN is still reachable by exact match.
	c, err = cs.Get(8443, "server-default.example.com")
	if assert.NoError(err) {
		assert.Equal("server-default.example.com", c.Leaf.DNSNames[0])
	}
}

func TestHttpx_CertStoreNewestNotBeforeWins(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)
	dir := t.TempDir()
	older := time.Now().Add(-3 * time.Hour)
	newer := time.Now().Add(-1 * time.Hour)
	// Both cover the same name; the one with the later NotBefore must win deterministically,
	// regardless of file name order.
	writeTokenCert(t, dir, "zold", older, "dup.example.com")
	writeTokenCert(t, dir, "anew", newer, "dup.example.com")

	cs := NewCertStore(dir, nil)
	cs.Load(context.Background())

	c, err := cs.Get(443, "dup.example.com")
	if assert.NoError(err) {
		assert.Equal(newer.Unix(), c.Leaf.NotBefore.Unix())
	}
}

// TestHttpx_CertStorePrefixedTokenBackCompat verifies that legacy filenames like
// "httpingress-443-cert.pem" yield token "443" by extracting the substring after the last "-".
func TestHttpx_CertStorePrefixedTokenBackCompat(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)
	dir := t.TempDir()
	now := time.Now().Add(-time.Hour)
	// Legacy prefixed form. Token = the substring after the last "-", i.e. "443".
	writeTokenCert(t, dir, "httpingress-443", now, "legacy.example.com")

	cs := NewCertStore(dir, nil)
	cs.Load(context.Background())

	// Port 443 default served on unknown-SNI lookup.
	c, err := cs.Get(443, "unknown.host")
	if assert.NoError(err) {
		assert.Equal("legacy.example.com", c.Leaf.DNSNames[0])
	}
}

// TestHttpx_CertStorePortDefaultTiebreak verifies that two files claiming the same numeric token
// (e.g. "443-cert.pem" and "httpingress-443-cert.pem" coexisting) resolve by newest NotBefore.
func TestHttpx_CertStorePortDefaultTiebreak(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)
	dir := t.TempDir()
	older := time.Now().Add(-3 * time.Hour)
	newer := time.Now().Add(-1 * time.Hour)
	// Both claim port 443 via the numeric token. Different SANs so we can identify which won.
	writeTokenCert(t, dir, "httpingress-443", older, "legacy.example.com")
	writeTokenCert(t, dir, "443", newer, "new.example.com")

	cs := NewCertStore(dir, nil)
	cs.Load(context.Background())

	c, err := cs.Get(443, "unknown.host")
	if assert.NoError(err) {
		assert.Equal(newer.Unix(), c.Leaf.NotBefore.Unix())
	}
}

func TestHttpx_CertStoreDynamicReload(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)
	dir := t.TempDir()
	now := time.Now().Add(-time.Hour)
	writeTokenCert(t, dir, "a", now, "a.example.com")

	cs := NewCertStore(dir, nil)
	cs.Load(context.Background())

	_, err := cs.Get(443, "a.example.com")
	assert.NoError(err)
	_, err = cs.Get(443, "b.example.com")
	assert.Error(err)

	// No change yet.
	assert.False(cs.ReloadIfChanged(context.Background()))

	// Add a new cert and reload picks it up.
	writeTokenCert(t, dir, "b", now, "b.example.com")
	assert.True(cs.ReloadIfChanged(context.Background()))

	_, err = cs.Get(443, "b.example.com")
	assert.NoError(err)
	// The original cert is still served.
	_, err = cs.Get(443, "a.example.com")
	assert.NoError(err)

	// A subsequent reload with no change is a no-op.
	assert.False(cs.ReloadIfChanged(context.Background()))
}

func TestHttpx_CertStoreNotLoaded(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)
	cs := NewCertStore(t.TempDir(), nil)
	_, err := cs.Get(443, "any.host")
	assert.Error(err)
}

// TestHttpx_CertStoreWatch verifies the fsnotify watcher reloads the index when a new cert
// appears, and that it returns when its context is cancelled.
func TestHttpx_CertStoreWatch(t *testing.T) {
	assert := testarossa.For(t)
	dir := t.TempDir()
	now := time.Now().Add(-time.Hour)
	writeTokenCert(t, dir, "a", now, "a.example.com")

	cs := NewCertStore(dir, nil)
	cs.Load(context.Background())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- cs.Watch(ctx) }()

	// The new SAN is not resolvable until the watcher reloads.
	_, err := cs.Get(443, "b.example.com")
	assert.Error(err)

	writeTokenCert(t, dir, "b", now, "b.example.com")

	deadline := time.Now().Add(5 * time.Second)
	for {
		_, err = cs.Get(443, "b.example.com")
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("watcher did not reload the new certificate within the timeout")
		}
		time.Sleep(50 * time.Millisecond)
	}
	assert.NoError(err)

	cancel()
	select {
	case werr := <-done:
		assert.Error(werr) // returns ctx.Err() on cancel
	case <-time.After(2 * time.Second):
		t.Fatal("watch did not return after context cancellation")
	}
}

// TestHttpx_CertStoreGetCertificate wires through the tls.Config-shaped callback.
func TestHttpx_CertStoreGetCertificate(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)
	dir := t.TempDir()
	now := time.Now().Add(-time.Hour)
	writeTokenCert(t, dir, "app", now, "app.example.com")

	cs := NewCertStore(dir, nil)
	cs.Load(context.Background())

	get := cs.GetCertificate(443)
	c, err := get(&tls.ClientHelloInfo{ServerName: "app.example.com"})
	if assert.NoError(err) {
		assert.Equal("app.example.com", c.Leaf.DNSNames[0])
	}
	_, err = get(&tls.ClientHelloInfo{ServerName: "missing.example.com"})
	assert.Error(err)
}

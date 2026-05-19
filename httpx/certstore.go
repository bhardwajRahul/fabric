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
	"crypto/tls"
	"crypto/x509"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/microbus-io/errors"
)

// CertStore file-naming conventions:
//   - {token}-cert.pem + {token}-key.pem -- a cert/key pair. The two files must share the full
//     prefix portion (everything before -cert.pem / -key.pem) so they pair up.
//   - {token} is the substring after the last "-" in the prefix portion. So both
//     "443-cert.pem" and "httpingress-443-cert.pem" yield token "443". Existing deployments
//     that use a service-name prefix continue to work without renaming files.
//   - When token parses as a port number (1-65535), the cert is also registered as that
//     listener's default for unmatched SNI.
//   - cert.pem + key.pem (no token, no prefix) -- the server-wide default served on any listener
//     as a last-resort fallback when no SAN, wildcard, or port-named cert matches.
const (
	certFileSuffix        = "-cert.pem"
	keyFileSuffix         = "-key.pem"
	serverDefaultCertName = "cert.pem"
	serverDefaultKeyName  = "key.pem"
)

// CertLogger is the minimal logger CertStore needs. It matches the Microbus connector's LogInfo
// and LogWarn so a microservice can be passed directly. A nil logger is silent.
type CertLogger interface {
	LogInfo(ctx context.Context, msg string, args ...any)
	LogWarn(ctx context.Context, msg string, args ...any)
}

// nopCertLogger discards all log records.
type nopCertLogger struct{}

func (nopCertLogger) LogInfo(context.Context, string, ...any) {}
func (nopCertLogger) LogWarn(context.Context, string, ...any) {}

// loadedCert is a parsed certificate together with the file it was loaded from.
type loadedCert struct {
	cert *tls.Certificate
	file string
}

// certIndex is an immutable snapshot of the loaded certificates, indexed for SNI resolution.
type certIndex struct {
	exact         map[string]*loadedCert // lowercased SAN DNS name -> cert
	wildcard      map[string]*loadedCert // parent domain of a "*.parent" SAN -> cert
	byPort        map[int]*loadedCert    // numeric-token cert -> that listener's default
	serverDefault *loadedCert            // cert.pem -> server-wide fallback
	fp            string                 // fingerprint of the source files, for change detection
}

/*
CertStore discovers TLS cert/key file pairs in a directory and serves them by SNI. The file-naming
convention (see the package-level constants) ties a cert to its key by a shared filename prefix and
optionally marks a per-port default or the server-wide default. Certificate matching is on the
parsed SAN DNS names of each certificate, not on the file name, so a misnamed file cannot cause a
wrong-certificate mis-serve.

Resolution order on every TLS handshake:

 1. Exact SAN match against the SNI.
 2. Wildcard SAN match.
 3. The listener's port-named default (a cert whose token parses as the listener's port).
 4. The server-wide default (cert.pem / key.pem).
 5. With no match, the handshake fails rather than serve an arbitrary default.

The active index is swapped atomically so resolution stays lock-free on the handshake path. Reload
is driven by fsnotify on the directory (not individual files) so atomic-rename and symlink-swap
rotations - Kubernetes secret mounts, certbot - are detected.
*/
type CertStore struct {
	dir string
	log CertLogger
	idx atomic.Pointer[certIndex]
}

// NewCertStore creates a store scanning dir (empty means the working directory). A nil log is
// treated as silent.
func NewCertStore(dir string, log CertLogger) *CertStore {
	if log == nil {
		log = nopCertLogger{}
	}
	return &CertStore{dir: dir, log: log}
}

// Get resolves a certificate for the given SNI name on the given listener port, in the order
// documented on CertStore. It returns an error rather than serving a wrong certificate.
func (cs *CertStore) Get(port int, serverName string) (*tls.Certificate, error) {
	idx := cs.idx.Load()
	if idx == nil {
		return nil, errors.New("certificate store not loaded")
	}
	name := strings.ToLower(strings.TrimSuffix(serverName, "."))
	if name != "" {
		if c := idx.exact[name]; c != nil {
			return c.cert, nil
		}
		if _, parent, ok := strings.Cut(name, "."); ok {
			if c := idx.wildcard[parent]; c != nil {
				return c.cert, nil
			}
		}
	}
	if c := idx.byPort[port]; c != nil {
		return c.cert, nil
	}
	if idx.serverDefault != nil {
		return idx.serverDefault.cert, nil
	}
	return nil, errors.New("no certificate for server name '%s'", serverName)
}

// GetCertificate returns a callback bound to a listener port, suitable for tls.Config.GetCertificate.
// It saves the caller from writing the closure manually.
func (cs *CertStore) GetCertificate(listenPort int) func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	return func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
		return cs.Get(listenPort, hello.ServerName)
	}
}

// Load builds the index and stores it atomically. Call once before the first handshake.
func (cs *CertStore) Load(ctx context.Context) {
	cs.idx.Store(cs.build(ctx))
}

// ReloadIfChanged rebuilds and atomically swaps the index when the source files have changed since
// the last load, logging an info record when it does. It returns true if a reload occurred.
func (cs *CertStore) ReloadIfChanged(ctx context.Context) bool {
	cur := cs.idx.Load()
	if cur != nil && cur.fp == cs.fingerprint() {
		return false
	}
	cs.idx.Store(cs.build(ctx))
	cs.log.LogInfo(ctx, "Reloaded TLS certificates")
	return true
}

// Watch observes the certificate directory and reloads the index when the cert/key files change.
// It watches the directory rather than the individual files so that atomic-rename and symlink-swap
// rotations are detected. Events are debounced to coalesce the multi-file burst of a single
// rotation and to avoid reading a half-written pair. Watch runs until ctx is cancelled.
func (cs *CertStore) Watch(ctx context.Context) (err error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return errors.Trace(err)
	}
	defer w.Close()
	dir := cs.dir
	if dir == "" {
		dir = "."
	}
	err = w.Add(dir)
	if err != nil {
		return errors.Trace(err)
	}
	// Catch a change that landed between the initial load and the watch being established.
	cs.ReloadIfChanged(ctx)

	var debounce <-chan time.Time
	for {
		select {
		case <-ctx.Done():
			return errors.Trace(ctx.Err())
		case ev := <-w.Events:
			if cs.isCertFile(filepath.Base(ev.Name)) {
				debounce = time.After(500 * time.Millisecond)
			}
		case <-debounce:
			debounce = nil
			cs.ReloadIfChanged(ctx)
		case e := <-w.Errors:
			cs.log.LogWarn(ctx, "Certificate watch error", "error", e)
		}
	}
}

// isCertFile reports whether base names a cert or key file this store tracks: either the bare
// server-default form (cert.pem / key.pem) or any file ending in -cert.pem / -key.pem.
func (cs *CertStore) isCertFile(base string) bool {
	if base == serverDefaultCertName || base == serverDefaultKeyName {
		return true
	}
	return strings.HasSuffix(base, certFileSuffix) || strings.HasSuffix(base, keyFileSuffix)
}

// fingerprint summarizes the cert/key files so a reload can detect changes cheaply.
func (cs *CertStore) fingerprint() string {
	files, _ := filepath.Glob(filepath.Join(cs.dir, "*"+certFileSuffix))
	keys, _ := filepath.Glob(filepath.Join(cs.dir, "*"+keyFileSuffix))
	files = append(files, keys...)
	files = append(files,
		filepath.Join(cs.dir, serverDefaultCertName),
		filepath.Join(cs.dir, serverDefaultKeyName),
	)
	sort.Strings(files)
	var b strings.Builder
	for _, f := range files {
		st, err := os.Stat(f)
		if err != nil {
			continue
		}
		b.WriteString(f)
		b.WriteByte(0)
		b.WriteString(strconv.FormatInt(st.Size(), 10))
		b.WriteByte(0)
		b.WriteString(strconv.FormatInt(st.ModTime().UnixNano(), 10))
		b.WriteByte('\n')
	}
	return b.String()
}

// build scans the directory for cert/key pairs and returns a fresh SNI index. A pair with a
// missing or invalid key is skipped with a warning and never aborts the build.
func (cs *CertStore) build(ctx context.Context) *certIndex {
	idx := &certIndex{
		exact:    map[string]*loadedCert{},
		wildcard: map[string]*loadedCert{},
		byPort:   map[int]*loadedCert{},
		fp:       cs.fingerprint(),
	}
	certFiles, _ := filepath.Glob(filepath.Join(cs.dir, "*"+certFileSuffix))
	sort.Strings(certFiles)
	for _, certFile := range certFiles {
		base := filepath.Base(certFile)
		// fullPrefix is everything before "-cert.pem". The matching key file shares the same
		// fullPrefix, so two services' files don't accidentally cross-pair.
		fullPrefix := strings.TrimSuffix(base, certFileSuffix)
		if fullPrefix == "" {
			continue
		}
		// Token is the substring after the last "-", or the whole prefix if no "-". This is what
		// covers backward-compat: "httpingress-443-cert.pem" and "443-cert.pem" both yield "443".
		token := fullPrefix
		if i := strings.LastIndexByte(fullPrefix, '-'); i >= 0 {
			token = fullPrefix[i+1:]
		}
		keyFile := filepath.Join(filepath.Dir(certFile), fullPrefix+keyFileSuffix)
		lc := cs.loadPair(ctx, certFile, keyFile)
		if lc == nil {
			continue
		}
		// A token that is purely a port number marks that listener's default cert.
		if port, err := strconv.Atoi(token); err == nil && port >= 1 && port <= 65535 {
			cs.putByPort(ctx, idx, port, lc)
		}
		cs.indexSANs(ctx, idx, lc)
	}
	// Server-wide default: cert.pem + key.pem (no token).
	defaultCertFile := filepath.Join(cs.dir, serverDefaultCertName)
	if _, err := os.Stat(defaultCertFile); err == nil {
		defaultKeyFile := filepath.Join(cs.dir, serverDefaultKeyName)
		if lc := cs.loadPair(ctx, defaultCertFile, defaultKeyFile); lc != nil {
			idx.serverDefault = lc
			cs.indexSANs(ctx, idx, lc)
		}
	}
	return idx
}

// loadPair loads and parses one cert/key pair, returning nil with a logged warning on any failure
// so the surrounding build continues with the remaining certificates.
func (cs *CertStore) loadPair(ctx context.Context, certFile, keyFile string) *loadedCert {
	pair, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		cs.log.LogWarn(ctx, "Skipping TLS certificate", "file", certFile, "error", err)
		return nil
	}
	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		cs.log.LogWarn(ctx, "Skipping TLS certificate", "file", certFile, "error", err)
		return nil
	}
	pair.Leaf = leaf
	return &loadedCert{cert: &pair, file: certFile}
}

// indexSANs registers lc under its parsed DNS names, splitting exact and wildcard SANs.
func (cs *CertStore) indexSANs(ctx context.Context, idx *certIndex, lc *loadedCert) {
	for _, name := range lc.cert.Leaf.DNSNames {
		// SNI and SAN DNS names are ASCII A-labels; only case needs normalizing.
		name = strings.ToLower(strings.TrimSuffix(name, "."))
		if name == "" {
			continue
		}
		if parent, ok := strings.CutPrefix(name, "*."); ok {
			cs.put(ctx, idx.wildcard, parent, lc)
		} else {
			cs.put(ctx, idx.exact, name, lc)
		}
	}
}

// put inserts lc under key in a SAN map, resolving a collision deterministically: the certificate
// with the later NotBefore wins and the shadowed file is logged.
func (cs *CertStore) put(ctx context.Context, m map[string]*loadedCert, key string, lc *loadedCert) {
	cur, ok := m[key]
	if !ok {
		m[key] = lc
		return
	}
	keep, drop := lc, cur
	if cur.cert.Leaf.NotBefore.After(lc.cert.Leaf.NotBefore) {
		keep, drop = cur, lc
	}
	m[key] = keep
	cs.log.LogWarn(ctx, "Shadowed TLS certificate", "name", key, "kept", keep.file, "shadowed", drop.file)
}

// putByPort inserts lc as the per-port default for port, mirroring put's newest-NotBefore tiebreak.
// This matters when both "443-cert.pem" and a prefixed "httpingress-443-cert.pem" coexist; the
// shared token would otherwise race silently.
func (cs *CertStore) putByPort(ctx context.Context, idx *certIndex, port int, lc *loadedCert) {
	cur, ok := idx.byPort[port]
	if !ok {
		idx.byPort[port] = lc
		return
	}
	keep, drop := lc, cur
	if cur.cert.Leaf.NotBefore.After(lc.cert.Leaf.NotBefore) {
		keep, drop = cur, lc
	}
	idx.byPort[port] = keep
	cs.log.LogWarn(ctx, "Shadowed TLS port-default certificate",
		"port", port, "kept", keep.file, "shadowed", drop.file)
}

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

package connector

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/env"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// resolveOTLPSetting returns the value of an OTLP exporter setting for the given signal ("TRACES", "METRICS" or
// "LOGS"), falling back to the generic setting shared by all signals.
// https://opentelemetry.io/docs/specs/otel/protocol/exporter/
func resolveOTLPSetting(signal string, name string) string {
	v := env.Get("OTEL_EXPORTER_OTLP_" + signal + "_" + name)
	if v == "" {
		v = env.Get("OTEL_EXPORTER_OTLP_" + name)
	}
	return v
}

// resolveOTLPEndpoint returns the OTLP exporter endpoint for the given signal.
func resolveOTLPEndpoint(signal string) string {
	return resolveOTLPSetting(signal, "ENDPOINT")
}

// resolveOTLPProtocol returns the OTLP exporter protocol for the given signal. A gRPC endpoint on the conventional
// port :4317 is detected when no protocol is explicitly set. An empty result means the caller should default to HTTP.
func resolveOTLPProtocol(signal string, endpoint string) string {
	protocol := resolveOTLPSetting(signal, "PROTOCOL")
	if protocol == "" && strings.Contains(endpoint, ":4317") {
		protocol = "grpc"
	}
	return protocol
}

// otelResource builds the OpenTelemetry resource shared by the tracer, meter and logger providers, identifying this
// microservice across all signals. The namespace is the plane, except in TESTING where it is "testing".
// https://opentelemetry.io/docs/specs/semconv/attributes-registry/service/
func (c *Connector) otelResource() *resource.Resource {
	namespace := c.Plane()
	if c.deployment == TESTING {
		namespace = "testing"
	}
	return resource.NewSchemaless(
		attribute.String("service.namespace", namespace),
		attribute.String("service.name", c.Hostname()),
		attribute.Int("service.version", c.Version()),
		attribute.String("service.instance.id", c.ID()),
		attribute.String("deployment.environment", c.Deployment()),
	)
}

/*
otlpConn is a reference-counted OTLP transport shared by every connector in the executable that exports to the same
endpoint over the same protocol. Each export signal (traces, metrics, logs) of each connector holds one reference;
the underlying gRPC connection or HTTP client is created on the first acquire and torn down on the last release.

Sharing avoids opening a separate TCP/TLS connection per signal per connector when a single executable bundles many
microservices, which is the common deployment shape.
*/
type otlpConn struct {
	grpc *grpc.ClientConn
	http *http.Client
	refs int
}

// protocol reports which OTLP transport this connection carries, "grpc" or "http", selecting the exporter to build.
func (conn *otlpConn) protocol() string {
	if conn.grpc != nil {
		return "grpc"
	}
	return "http"
}

var (
	otlpConnsMu sync.Mutex
	otlpConns   = map[string]*otlpConn{}
)

/*
otlpDialConfig captures everything that determines an OTLP transport: the protocol and endpoint, plus the TLS
material resolved from the standard OTLP environment variables. Two signals share a connection only when all of
these match, so a connection secured with one CA or client certificate never aliases one secured differently.
*/
type otlpDialConfig struct {
	protocol   string
	endpoint   string
	caFile     string // OTEL_EXPORTER_OTLP[_signal]_CERTIFICATE - CA bundle verifying the collector
	clientCert string // OTEL_EXPORTER_OTLP[_signal]_CLIENT_CERTIFICATE - client cert for mTLS
	clientKey  string // OTEL_EXPORTER_OTLP[_signal]_CLIENT_KEY - client key for mTLS
	insecure   string // OTEL_EXPORTER_OTLP[_signal]_INSECURE - explicit transport-security override
}

// key uniquely identifies a shared connection. Including the TLS material and the insecure override prevents two
// signals pointed at the same endpoint but secured differently from sharing one connection.
func (cfg otlpDialConfig) key() string {
	return strings.Join([]string{cfg.protocol, cfg.endpoint, cfg.caFile, cfg.clientCert, cfg.clientKey, cfg.insecure}, "|")
}

// secure reports whether the gRPC connection uses transport security. An explicit OTEL_EXPORTER_OTLP[_signal]_INSECURE
// boolean takes precedence; otherwise security follows the endpoint URL scheme (https is secured).
func (cfg otlpDialConfig) secure() bool {
	if cfg.insecure != "" {
		if v, err := strconv.ParseBool(cfg.insecure); err == nil {
			return !v
		}
	}
	if u, err := url.Parse(cfg.endpoint); err == nil {
		return u.Scheme == "https"
	}
	return false
}

// acquireOTLPConn resolves the protocol and TLS material for the signal's endpoint, then returns the shared OTLP
// transport for that configuration, creating it on first use and incrementing its reference count. The returned
// key must be passed to releaseOTLPConn when done; the connection's protocol method selects the exporter to build.
func acquireOTLPConn(signal string, endpoint string) (conn *otlpConn, key string, err error) {
	cfg := otlpDialConfig{
		protocol:   resolveOTLPProtocol(signal, endpoint),
		endpoint:   endpoint,
		caFile:     resolveOTLPSetting(signal, "CERTIFICATE"),
		clientCert: resolveOTLPSetting(signal, "CLIENT_CERTIFICATE"),
		clientKey:  resolveOTLPSetting(signal, "CLIENT_KEY"),
		insecure:   resolveOTLPSetting(signal, "INSECURE"),
	}
	key = cfg.key()
	otlpConnsMu.Lock()
	defer otlpConnsMu.Unlock()
	conn = otlpConns[key]
	if conn == nil {
		conn = &otlpConn{}
		if cfg.protocol == "grpc" {
			conn.grpc, err = dialOTLPGRPC(cfg)
			if err != nil {
				return nil, "", errors.Trace(err)
			}
		} else {
			// A cloned default transport gives this client its own keep-alive connection pool, shared across all
			// exporters that resolve to the same key.
			tr := http.DefaultTransport.(*http.Transport).Clone()
			tlsCfg, err := otlpTLSConfig(cfg)
			if err != nil {
				return nil, "", errors.Trace(err)
			}
			if tlsCfg != nil {
				tr.TLSClientConfig = tlsCfg
			}
			conn.http = &http.Client{Transport: tr}
		}
		otlpConns[key] = conn
	}
	conn.refs++
	return conn, key, nil
}

// releaseOTLPConn decrements the reference count of the shared OTLP transport identified by key, tearing it down
// once no signal references it. It is safe to call with an empty key (no-op).
func releaseOTLPConn(key string) {
	if key == "" {
		return
	}
	otlpConnsMu.Lock()
	defer otlpConnsMu.Unlock()
	conn := otlpConns[key]
	if conn == nil {
		return
	}
	conn.refs--
	if conn.refs > 0 {
		return
	}
	if conn.grpc != nil {
		_ = conn.grpc.Close()
	}
	if conn.http != nil {
		conn.http.CloseIdleConnections()
	}
	delete(otlpConns, key)
}

// dialOTLPGRPC creates a lazy gRPC client connection to the OTLP endpoint. Transport security follows the endpoint
// URL scheme (https is secured) unless an explicit insecure override is set. When secured, the TLS configuration
// honors the OTLP certificate environment variables (custom CA, client cert for mTLS), defaulting to the host's root
// CAs. The connection is not established until the first export, so it never blocks Startup.
func dialOTLPGRPC(cfg otlpDialConfig) (*grpc.ClientConn, error) {
	target := cfg.endpoint
	if u, err := url.Parse(cfg.endpoint); err == nil && u.Host != "" {
		target = u.Host
	}
	var creds credentials.TransportCredentials
	if cfg.secure() {
		tlsCfg, err := otlpTLSConfig(cfg)
		if err != nil {
			return nil, errors.Trace(err)
		}
		creds = credentials.NewTLS(tlsCfg) // A nil config uses the host's root CAs
	} else {
		creds = insecure.NewCredentials()
	}
	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(creds))
	if err != nil {
		return nil, errors.Trace(err)
	}
	return conn, nil
}

// otlpTLSConfig builds a TLS configuration from the OTLP certificate environment variables, or returns nil (meaning
// the host's root CAs with no client certificate) when none are set. A custom CA verifies a collector presenting a
// privately-signed certificate; a client certificate and key enable mutual TLS.
func otlpTLSConfig(cfg otlpDialConfig) (*tls.Config, error) {
	if cfg.caFile == "" && cfg.clientCert == "" && cfg.clientKey == "" {
		return nil, nil
	}
	tlsCfg := &tls.Config{}
	if cfg.caFile != "" {
		pem, err := os.ReadFile(cfg.caFile)
		if err != nil {
			return nil, errors.Trace(err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, errors.New("no OTLP CA certificates parsed from file", "file", cfg.caFile)
		}
		tlsCfg.RootCAs = pool
	}
	if cfg.clientCert != "" || cfg.clientKey != "" {
		crt, err := tls.LoadX509KeyPair(cfg.clientCert, cfg.clientKey)
		if err != nil {
			return nil, errors.Trace(err)
		}
		tlsCfg.Certificates = []tls.Certificate{crt}
	}
	return tlsCfg, nil
}

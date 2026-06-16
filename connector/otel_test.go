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
	"os"
	"path/filepath"
	"testing"

	"github.com/microbus-io/fabric/env"
	"github.com/microbus-io/testarossa"
)

// connRefs reads the reference count of a shared OTLP connection, or -1 if it is not present.
func connRefs(key string) int {
	otlpConnsMu.Lock()
	defer otlpConnsMu.Unlock()
	if conn := otlpConns[key]; conn != nil {
		return conn.refs
	}
	return -1
}

// TestConnector_OTLPConnSharing verifies that connectors exporting to the same endpoint over the same protocol
// share a single reference-counted connection, and that it is torn down only when the last reference is released.
func TestConnector_OTLPConnSharing(t *testing.T) {
	// No parallel - mutates the process-global OTLP connection registry and environment
	assert := testarossa.For(t)

	env.Push("OTEL_EXPORTER_OTLP_PROTOCOL", "grpc")
	defer env.Pop("OTEL_EXPORTER_OTLP_PROTOCOL")

	const endpoint = "http://127.0.0.1:14317" // Valid format, never dialed (grpc.NewClient is lazy)

	// First reference (one connector's trace exporter) creates the gRPC connection.
	c1, key, err := acquireOTLPConn("TRACES", endpoint)
	assert.NoError(err)
	assert.Equal("grpc", c1.protocol())
	assert.True(c1.grpc != nil, "grpc protocol should yield a gRPC connection")
	assert.True(c1.http == nil, "grpc protocol should not yield an HTTP client")
	assert.Equal(1, connRefs(key))

	// More references - the same connector's metric exporter, plus a second connector's two signals - all resolve
	// to the same key and reuse the one connection.
	c2, _, _ := acquireOTLPConn("METRICS", endpoint)
	c3, _, _ := acquireOTLPConn("TRACES", endpoint)
	c4, _, _ := acquireOTLPConn("METRICS", endpoint)
	assert.True(c1 == c2 && c2 == c3 && c3 == c4, "all references should point to the same shared connection")
	assert.Equal(4, connRefs(key))

	// Releasing all but the last keeps the connection alive.
	releaseOTLPConn(key)
	releaseOTLPConn(key)
	releaseOTLPConn(key)
	assert.Equal(1, connRefs(key))

	// Releasing the last reference tears it down and removes it from the registry.
	releaseOTLPConn(key)
	assert.Equal(-1, connRefs(key))

	// Releasing an empty or unknown key is a no-op.
	releaseOTLPConn("")
	releaseOTLPConn(key)
}

// TestConnector_OTLPConnProtocol verifies that the resolved protocol selects the transport kind and that an HTTP and
// a gRPC connection to the same endpoint do not alias.
func TestConnector_OTLPConnProtocol(t *testing.T) {
	// No parallel - mutates the process-global OTLP connection registry and environment
	assert := testarossa.For(t)

	const endpoint = "http://127.0.0.1:14318"

	env.Push("OTEL_EXPORTER_OTLP_PROTOCOL", "http")
	httpConn, httpKey, err := acquireOTLPConn("TRACES", endpoint)
	env.Pop("OTEL_EXPORTER_OTLP_PROTOCOL")
	assert.NoError(err)
	assert.Equal("http", httpConn.protocol())
	assert.True(httpConn.http != nil, "http protocol should yield an HTTP client")
	assert.True(httpConn.grpc == nil, "http protocol should not yield a gRPC connection")

	env.Push("OTEL_EXPORTER_OTLP_PROTOCOL", "grpc")
	grpcConn, grpcKey, err := acquireOTLPConn("TRACES", endpoint)
	env.Pop("OTEL_EXPORTER_OTLP_PROTOCOL")
	assert.NoError(err)
	assert.Equal("grpc", grpcConn.protocol())
	assert.True(httpKey != grpcKey, "http and grpc to the same endpoint must not share a key")
	assert.True(httpConn != grpcConn, "different protocols must not alias to the same connection")

	releaseOTLPConn(httpKey)
	releaseOTLPConn(grpcKey)
	assert.Equal(-1, connRefs(httpKey))
	assert.Equal(-1, connRefs(grpcKey))
}

// TestConnector_OTLPDialConfigKey verifies that protocol, endpoint, and each piece of TLS material all participate in
// the connection key, so differently-configured signals never alias to one shared connection.
func TestConnector_OTLPDialConfigKey(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	base := otlpDialConfig{protocol: "grpc", endpoint: "https://collector:4317"}
	variants := []otlpDialConfig{
		{protocol: "http", endpoint: "https://collector:4317"},
		{protocol: "grpc", endpoint: "https://other:4317"},
		{protocol: "grpc", endpoint: "https://collector:4317", caFile: "/ca.pem"},
		{protocol: "grpc", endpoint: "https://collector:4317", clientCert: "/c.pem"},
		{protocol: "grpc", endpoint: "https://collector:4317", clientKey: "/k.pem"},
		{protocol: "grpc", endpoint: "https://collector:4317", insecure: "true"},
	}
	for _, v := range variants {
		assert.True(base.key() != v.key(), "key must differ for %+v", v)
	}
}

// TestConnector_OTLPSecure verifies that transport security follows the endpoint scheme by default and that an
// explicit OTEL_EXPORTER_OTLP[_signal]_INSECURE boolean overrides it.
func TestConnector_OTLPSecure(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	// Scheme governs when no override is set.
	assert.True(otlpDialConfig{endpoint: "https://collector:4317"}.secure(), "https should be secure")
	assert.True(!otlpDialConfig{endpoint: "http://collector:4318"}.secure(), "http should be insecure")

	// An explicit insecure override wins over the scheme, either way.
	assert.True(!otlpDialConfig{endpoint: "https://collector:4317", insecure: "true"}.secure(), "insecure=true overrides https")
	assert.True(otlpDialConfig{endpoint: "http://collector:4318", insecure: "false"}.secure(), "insecure=false overrides http")

	// A non-boolean override is ignored, falling back to the scheme.
	assert.True(otlpDialConfig{endpoint: "https://collector:4317", insecure: "maybe"}.secure(), "unparseable override falls back to scheme")
}

// TestConnector_OTELProviders verifies that the tracer and meter provider accessors never return nil - a no-op
// provider when the signal is disabled, and the live SDK provider once configured.
func TestConnector_OTELProviders(t *testing.T) {
	// No parallel - sets environment
	assert := testarossa.For(t)
	ctx := t.Context()

	// Before Startup, with no exporter configured, the accessors return usable no-op providers.
	con := New("otel.providers.connector")
	assert.NotNil(con.TracerProvider())
	assert.NotNil(con.MeterProvider())
	con.TracerProvider().Tracer("test").Start(ctx, "span") // Must not panic
	con.MeterProvider().Meter("test")                      // Must not panic

	// With both signals configured, the accessors return the live SDK providers.
	env.Push("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "nil") // nil trace client
	env.Push("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT", "http://127.0.0.1:14320")
	defer env.Pop("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT")
	defer env.Pop("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT")

	con = New("otel.providers.connector")
	err := con.Startup(ctx)
	assert.NoError(err)
	defer con.Shutdown(ctx)
	assert.NotNil(con.TracerProvider())
	assert.NotNil(con.MeterProvider())
	assert.True(con.TracerProvider() == con.traceProvider, "should return the live tracer provider")
	// MeterProvider returns the identity-injecting decorator wrapping the live SDK provider.
	mp, ok := con.MeterProvider().(attributedMeterProvider)
	assert.True(ok, "meter provider should be the attribute-injecting decorator")
	assert.True(mp.inner == con.meterProvider, "decorator should wrap the live meter provider")
}

// TestConnector_OTLPTLSConfig verifies that the OTLP TLS configuration is nil when no certificate environment is
// set, and that bad certificate material surfaces an error rather than silently falling back to no TLS.
func TestConnector_OTLPTLSConfig(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	// No certificate settings - use the host's root CAs, no client certificate.
	tlsCfg, err := otlpTLSConfig(otlpDialConfig{})
	assert.NoError(err)
	assert.True(tlsCfg == nil, "no certificate env should yield a nil TLS config")

	// A CA file that does not exist is an error, not a silent fallback.
	_, err = otlpTLSConfig(otlpDialConfig{caFile: filepath.Join(t.TempDir(), "missing.pem")})
	assert.Error(err)

	// A CA file with no parseable certificates is an error.
	badCA := filepath.Join(t.TempDir(), "bad.pem")
	assert.NoError(os.WriteFile(badCA, []byte("not a certificate"), 0o600))
	_, err = otlpTLSConfig(otlpDialConfig{caFile: badCA})
	assert.Error(err)

	// A missing client key alongside a client cert is an error.
	_, err = otlpTLSConfig(otlpDialConfig{clientCert: filepath.Join(t.TempDir(), "missing.crt")})
	assert.Error(err)
}

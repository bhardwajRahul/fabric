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
	"net/http"
	"strings"
	"testing"

	"github.com/microbus-io/fabric/sub"
	"github.com/microbus-io/fabric/utils"
	"github.com/microbus-io/testarossa"
)

func TestConnector_EncodePathPart(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	// Path segments use 2-digit lowercase %xx per byte. Stricter than url.PathEscape:
	// '.', '_', and any URL-special char all get encoded so the segment is
	// unambiguous in a NATS subject.
	testCases := []string{
		"UPPERCASE", "UPPERCASE",
		"file.html", "file%2ehtml",
		"a_b_c", "a%5fb%5fc",
		"two-W0rds", "two-W0rds",
		"special!", "special%21",
		"asteri*", "asteri%2a",
		"", "",
	}
	for i := 0; i < len(testCases); i += 2 {
		var b strings.Builder
		escapePathPart(&b, testCases[i])
		assert.Equal(testCases[i+1], b.String())
	}
}

func TestConnector_SubjectOfSubscription(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	// Argument order: (plane, port, hostname, position, method, path) - same as the on-wire order.
	// Hostname segments still flatten '.' to '_' (legacy form, preserves readability for
	// typical service identities). Path segments use 2-digit lowercase %xx for every byte
	// outside [A-Za-z0-9-], including '.'. Bare position (no id/locality) is `_`.
	assert.Equal("p0.safe.80.*.example_com._.GET.PATH.to.file%2ehtml", SubjectOfRequestSub("p0", "80", "EXAMPLE.com", "", "GET", "PATH/to/file.html"))
	assert.Equal("p0.safe.80.*.example_com._.GET.PATH._", SubjectOfRequestSub("p0", "80", "EXAMPLE.com", "", "GET", "PATH/"))
	assert.Equal("p0.safe.123.*.example_com._.POST.DIR.>", SubjectOfRequestSub("p0", "123", "example.com", "", "POST", "DIR/{...}"))
	assert.Equal("p0.safe.123.*.example_com._.PATCH.DIR.>", SubjectOfRequestSub("p0", "123", "example.com", "", "PATCH", "/DIR/{file...}"))
	assert.Equal("p0.safe.443.*.www_example_com._.DELETE.>", SubjectOfRequestSub("p0", "443", "www.example.com", "", "delete", "/{...}"))
	assert.Equal("p0.safe.443.*.www_example_com._.*._", SubjectOfRequestSub("p0", "443", "www.example.com", "", "ANY", ""))
	assert.Equal("p0.safe.*.*.example_com._.GET.PATH.to.file%2ehtml", SubjectOfRequestSub("p0", "0", "EXAMPLE.com", "", "GET", "PATH/to/file.html"))
	assert.Equal("p0.safe.443.*.example_com._.GET.foo.*.bar.*", SubjectOfRequestSub("p0", "443", "example.com", "", "GET", "/foo/{foo}/bar/{bar}"))
	assert.Equal("p0.safe.*.*.example_com._.*.foo.*.bar.*.>", SubjectOfRequestSub("p0", "0", "example.com", "", "ANY", "/foo/{foo}/bar/{bar}/{appendix...}"))
	assert.Equal("p0.safe.80.*.example_com._.GET.empty._._._", SubjectOfRequestSub("p0", "80", "EXAMPLE.com", "", "GET", "empty///"))
	// Trust-root :666 emits the danger segment.
	assert.Equal("p0.danger.666.*.example_com._.POST.mint", SubjectOfRequestSub("p0", "666", "example.com", "", "POST", "mint"))
	// Per-instance position
	assert.Equal("p0.safe.443.*.example_com.id-abcd1234.GET.path", SubjectOfRequestSub("p0", "443", "example.com", "id-abcd1234", "GET", "path"))
	// Per-locality position (single segment, hyphen-flattened)
	assert.Equal("p0.safe.443.*.example_com.loc-us-west.GET.path", SubjectOfRequestSub("p0", "443", "example.com", "loc-us-west", "GET", "path"))
	// Position is lowercased
	assert.Equal("p0.safe.443.*.example_com.id-abcd1234.GET.path", SubjectOfRequestSub("p0", "443", "example.com", "ID-ABCD1234", "GET", "path"))
	// URL-special character in hostname segment: '$' becomes %24, '.' stays '_'.
	assert.Equal("p0.safe.443.*.my%24_xml._.GET.path", SubjectOfRequestSub("p0", "443", "my$.xml", "", "GET", "path"))
}

func TestConnector_SubjectOfRequest(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	// Argument order: (plane, port, srcHostname, hostname, position, method, path) - same as the on-wire order.
	// Hostnames flatten '.' to '_'; '{' and '}' in published path segments become %7b/%7d.
	assert.Equal("p0.safe.80.by_com.example_com._.GET.PATH.to.file%2ehtml", SubjectOfRequest("p0", "80", "BY.com", "EXAMPLE.com", "", "GET", "PATH/to/file.html"))
	assert.Equal("p0.safe.80.by_com.example_com._.GET.PATH._", SubjectOfRequest("p0", "80", "BY.com", "EXAMPLE.com", "", "GET", "PATH/"))
	assert.Equal("p0.safe.123.by_com.example_com._.PATCH.DIR._", SubjectOfRequest("p0", "123", "by.com", "example.com", "", "PATCH", "/DIR/"))
	assert.Equal("p0.safe.443.by_com.www_example_com._.ANY.method", SubjectOfRequest("p0", "443", "by.com", "www.example.com", "", "ANY", "/method"))
	assert.Equal("p0.safe.443.by_com.www_example_com._.DELETE._", SubjectOfRequest("p0", "443", "by.com", "www.example.com", "", "delete", "/"))
	assert.Equal("p0.safe.443.by_com.www_example_com._.OPTIONS._", SubjectOfRequest("p0", "443", "by.com", "www.example.com", "", "OPTIONS", ""))
	assert.Equal("p0.safe.0.by_com.example_com._.GET.PATH.to.file%2ehtml", SubjectOfRequest("p0", "0", "BY.com", "EXAMPLE.com", "", "GET", "PATH/to/file.html"))
	assert.Equal("p0.safe.443.by_com.example_com._.GET.foo.%7bfoo%7d.bar.%7bbar%7d", SubjectOfRequest("p0", "443", "by.com", "example.com", "", "GET", "/foo/{foo}/bar/{bar}"))
	assert.Equal("p0.safe.80.by_com.example_com._.GET.empty._._._", SubjectOfRequest("p0", "80", "BY.com", "EXAMPLE.com", "", "GET", "empty///"))
	// Trust-root :666 emits the danger segment.
	assert.Equal("p0.danger.666.by_com.example_com._.POST.mint", SubjectOfRequest("p0", "666", "by.com", "example.com", "", "POST", "mint"))
	// Per-instance position
	assert.Equal("p0.safe.443.by_com.example_com.id-abcd1234.GET.path", SubjectOfRequest("p0", "443", "by.com", "example.com", "id-abcd1234", "GET", "path"))
	// Per-locality position
	assert.Equal("p0.safe.443.by_com.example_com.loc-us-west.GET.path", SubjectOfRequest("p0", "443", "by.com", "example.com", "loc-us-west", "GET", "path"))
}

func TestConnector_subjectOfResponseSub(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	// Reply subjects use `reply._` for trust+port; the trust segment alone
	// identifies the channel. Hostnames keep the legacy '.' → '_' flattening.
	assert.Equal("p0.reply._.*.example_com.id-1234", subjectOfResponseSub("p0", "example.com", "id-1234"))
	assert.Equal("p0.reply._.*.www_example_com.id-abcd1234", subjectOfResponseSub("p0", "www.example.com", "id-abcd1234"))
	assert.Equal("p0.reply._.*.www_example_com.id-abcd1234", subjectOfResponseSub("p0", "www.EXAMPLE.com", "ID-ABCD1234"))
}

func TestConnector_subjectOfResponse(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	// Argument order: (plane, srcHostname, hostname, id) - same as the on-wire order.
	assert.Equal("p0.reply._.by_com.example_com.id-1234", subjectOfResponse("p0", "by.com", "example.com", "id-1234"))
	assert.Equal("p0.reply._.by_com.www_example_com.id-abcd1234", subjectOfResponse("p0", "by.com", "www.example.com", "id-abcd1234"))
	assert.Equal("p0.reply._.by_com.www_example_com.id-abcd1234", subjectOfResponse("p0", "by.com", "www.EXAMPLE.com", "ID-ABCD1234"))
}

func TestConnector_EscapeHostname(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	// Multi-segment hostnames: every period becomes an underscore
	assert.Equal("www_sub_example_com", escapeHostname("www.sub.example.com"))
	assert.Equal("www_example_com", escapeHostname("www.example.com"))
	assert.Equal("example_com", escapeHostname("example.com"))

	// Single-segment hostnames: no periods to flatten, returned as-is
	assert.Equal("com", escapeHostname("com"))
	assert.Equal("hostname", escapeHostname("hostname"))
	assert.Equal("", escapeHostname(""))

	// Trailing/leading periods are flattened where they appear
	assert.Equal("example_", escapeHostname("example."))
	assert.Equal("_example", escapeHostname(".example"))
	assert.Equal("__", escapeHostname(".."))

	// Existing underscores in the input are preserved alongside flattened periods.
	// This is the documented edge case: distinct inputs can collapse to the same
	// flat form (e.g. "a_b.c" and "a.b.c" both produce "a_b_c"). The framework
	// constrains service identity hostnames to forbid `_`, so this collision is
	// avoided by validation at registration time, not by the encoding itself.
	assert.Equal("a_b_c", escapeHostname("a_b.c"))
	assert.Equal("a_b_c", escapeHostname("a.b.c"))

	// URL-special characters in route hostnames are percent-encoded with lowercase hex.
	// '.' still uses the legacy '_' substitution; only the non-dot specials get %xx.
	assert.Equal("my%24_xml", escapeHostname("my$.xml"))
	assert.Equal("a%21b_c", escapeHostname("a!b.c"))
	assert.Equal("a%7eb_c", escapeHostname("a~b.c"))
}

func BenchmarkConnector_EscapeHostname(b *testing.B) {
	for b.Loop() {
		escapeHostname("www.sub.example.com")
	}
	// goos: darwin
	// goarch: arm64
	// pkg: github.com/microbus-io/fabric/connector
	// cpu: Apple M1 Pro
	// BenchmarkConnector_EscapeHostname-10    	76115713	        15.66 ns/op	       0 B/op	       0 allocs/op
}

func TestConnector_UnescapeHostname(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	// Multi-segment flat forms: every underscore becomes a period
	assert.Equal("www.sub.example.com", unescapeHostname("www_sub_example_com"))
	assert.Equal("www.example.com", unescapeHostname("www_example_com"))
	assert.Equal("example.com", unescapeHostname("example_com"))

	// Single-segment inputs without underscores are returned as-is
	assert.Equal("com", unescapeHostname("com"))
	assert.Equal("hostname", unescapeHostname("hostname"))
	assert.Equal("", unescapeHostname(""))

	// Percent-encoded special chars are decoded back to their literal form.
	assert.Equal("my$.xml", unescapeHostname("my%24_xml"))
	assert.Equal("a!b.c", unescapeHostname("a%21b_c"))

	// Round-trip: flatten then unflatten recovers the original (for inputs
	// without underscores, which is the only valid input for service identity
	// hostnames going forward)
	for _, host := range []string{
		"www.sub.example.com",
		"example.com",
		"com",
		"my-service.core",
		"my$.xml",
		"a!b.c",
	} {
		assert.Equal(host, unescapeHostname(escapeHostname(host)))
	}
}

func TestConnector_SplitSubject(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	// Request subject: 6 fixed segments + method + path. Source and destination
	// hostnames are returned unflattened (underscores → dots).
	plane, trust, port, src, dest, position := splitSubject("microbus.safe.443.payments_core.target_core._.GET.path.to.file_html")
	assert.Equal("microbus", plane)
	assert.Equal("safe", trust)
	assert.Equal("443", port)
	assert.Equal("payments.core", src)
	assert.Equal("target.core", dest)
	assert.Equal("_", position)

	// Per-instance position
	_, _, _, _, _, position = splitSubject("microbus.safe.443.payments_core.target_core.id-abc123.GET.path")
	assert.Equal("id-abc123", position)

	// Per-locality position
	_, _, _, _, _, position = splitSubject("microbus.safe.443.payments_core.target_core.loc-us-west.GET.path")
	assert.Equal("loc-us-west", position)

	// Reply subject: exactly 6 segments, position carries the recipient ID
	plane, trust, port, src, dest, position = splitSubject("microbus.reply._.payments_core.target_core.id-xyz")
	assert.Equal("microbus", plane)
	assert.Equal("reply", trust)
	assert.Equal("_", port)
	assert.Equal("payments.core", src)
	assert.Equal("target.core", dest)
	assert.Equal("id-xyz", position)

	// Wildcard source position is returned as-is (no underscores to unflatten)
	_, _, _, src, _, _ = splitSubject("microbus.safe.443.*.target_core._.GET.path")
	assert.Equal("*", src)

	// Single-segment hostnames round-trip without modification
	_, _, _, src, dest, _ = splitSubject("microbus.safe.443.foo.bar._.GET.path")
	assert.Equal("foo", src)
	assert.Equal("bar", dest)

	// Trust-root subjects carry the danger segment
	_, trust, port, _, _, _ = splitSubject("microbus.danger.666.payments_core.shell_core._.POST.execute")
	assert.Equal("danger", trust)
	assert.Equal("666", port)
}

func TestConnector_LocalitySlot(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	// Empty
	assert.Equal("", escapeLocality(""))
	// Single segment
	assert.Equal("loc-us", escapeLocality("us"))
	// Hyphen-form prefix maps directly into the slot
	assert.Equal("loc-us-west", escapeLocality("us-west"))
	assert.Equal("loc-us-west-b", escapeLocality("us-west-b"))
	assert.Equal("loc-us-west-b-1", escapeLocality("us-west-b-1"))
	// Mixed-case input is lowercased
	assert.Equal("loc-us-west", escapeLocality("US-WEST"))
}

func TestConnector_CutIDOrLocality(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	// No prefix
	host, slot := cutIDOrLocality("example.com")
	assert.Equal("example.com", host)
	assert.Equal("", slot)

	// id- prefix
	host, slot = cutIDOrLocality("id-abc123.example.com")
	assert.Equal("example.com", host)
	assert.Equal("id-abc123", slot)

	// loc- prefix
	host, slot = cutIDOrLocality("loc-us-west.example.com")
	assert.Equal("example.com", host)
	assert.Equal("loc-us-west", slot)

	// Mixed-case prefix is lowercased on the slot
	host, slot = cutIDOrLocality("ID-ABC123.example.com")
	assert.Equal("example.com", host)
	assert.Equal("id-abc123", slot)

	// Single-segment hostname has no detectable prefix slot
	host, slot = cutIDOrLocality("singleton")
	assert.Equal("singleton", host)
	assert.Equal("", slot)

	// First segment without `id-` or `loc-` prefix is left intact
	host, slot = cutIDOrLocality("other.example.com")
	assert.Equal("other.example.com", host)
	assert.Equal("", slot)
}

func TestConnector_ReservedHostnamePrefixRejected(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	// Service identities cannot start with the reserved id- or loc- prefixes.
	for _, name := range []string{
		idPrefix + "abc.example.com",
		idPrefix + "abc",
		localityPrefix + "us-west.example.com",
		localityPrefix + "us-west",
		"ID-ABC.example.com",
		"LOC-US-WEST.example.com",
	} {
		c := NewConnector()
		err := c.SetHostname(name)
		assert.Error(err, "expected SetHostname(%q) to fail", name)

		// New() also routes through SetHostname; the captured init error surfaces on Startup.
		c2 := New(name)
		err = c2.Startup(t.Context())
		assert.Error(err, "expected Startup with hostname %q to fail", name)
	}

	// Hostnames that merely contain id- or loc- in a non-leading segment are fine.
	for _, name := range []string{
		"my.id-thing.example.com",
		"foo.loc-bar.example.com",
		"identity.example.com", // segment starts with "id" but not "id-"
		"local.example.com",    // segment starts with "loc" but not "loc-"
	} {
		c := NewConnector()
		err := c.SetHostname(name)
		assert.NoError(err, "expected SetHostname(%q) to succeed", name)
	}

	// The literal "all" and any hostname ending in ".all" are reserved
	// for control-plane broadcast addressing.
	for _, name := range []string{
		"all",
		"foo.all",
		"my.service.all",
		"ALL",
		"My.Service.ALL",
	} {
		c := NewConnector()
		err := c.SetHostname(name)
		assert.Error(err, "expected SetHostname(%q) to fail", name)
	}
}

func TestConnector_ReservedRoutePrefixRejected(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	assert := testarossa.For(t)

	con := New("reserved.route.test.connector")
	err := con.Startup(ctx)
	assert.NoError(err)
	defer con.Shutdown(ctx)

	noop := func(w http.ResponseWriter, r *http.Request) error { return nil }

	// Absolute subscription routes (//host/path) cannot use the reserved id- or
	// loc- prefixes in the host position.
	for _, route := range []string{
		"//id-abc/path",
		"//id-abc.example.com/path",
		"//loc-us-west/path",
		"//loc-us-west.example.com/path",
	} {
		err := con.Subscribe("X"+utils.RandomIdentifier(12), noop, sub.At("GET", route), sub.Web())
		assert.Error(err, "expected Subscribe with route %q to fail", route)
	}

	// Absolute routes with non-reserved hostnames register normally.
	for _, route := range []string{
		"//other.example.com/path",
		"//id.example.com/path",  // no trailing hyphen on segment
		"//loc.example.com/path", // no trailing hyphen on segment
	} {
		err := con.Subscribe("X"+utils.RandomIdentifier(12), noop, sub.At("GET", route), sub.Web())
		assert.NoError(err, "expected Subscribe with route %q to succeed", route)
	}
}

func TestConnector_UppercaseHostnameAndLocalityRejected(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	// SetHostname and SetLocality enforce a canonical-form contract: lowercase letters,
	// digits, dot separators, and hyphens. Uppercase or whitespace input is rejected
	// rather than silently coerced. Callers are responsible for normalization.
	server := New("UPPER.HOST.CONNECTOR")
	assert.NotEqual("UPPER.HOST.CONNECTOR", server.Hostname()) // captureInitErr stashed the error, hostname is empty

	server2 := New("upper.host.connector")
	err := server2.SetLocality("WEST.US")
	assert.Error(err)
}

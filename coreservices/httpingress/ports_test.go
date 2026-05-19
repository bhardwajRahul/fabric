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

package httpingress

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/microbus-io/testarossa"
)

func TestHttpingress_ParsePorts(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	// No legacy cert files in the test working directory, so bare ports are plaintext.
	specs, err := parsePorts("8080")
	if assert.NoError(err) {
		assert.Equal([]portSpec{{8080, false}}, specs)
	}

	// Explicit marker, whitespace, case-insensitive, and skipped empties.
	specs, err = parsePorts(" , 80 , 443  TLS , 8080 ,")
	if assert.NoError(err) {
		assert.Equal([]portSpec{{80, false}, {443, true}, {8080, false}}, specs)
	}

	// Explicit "80 tls" is a startup error.
	_, err = parsePorts("80 tls")
	assert.Error(err)

	// An unrecognized marker is ignored, so the entry is treated as a bare port.
	specs, err = parsePorts("443 ssl")
	if assert.NoError(err) {
		assert.Equal([]portSpec{{443, false}}, specs)
	}

	// Extra tokens beyond "tls" are ignored; TLS still wins.
	specs, err = parsePorts("443 tls extra")
	if assert.NoError(err) {
		assert.Equal([]portSpec{{443, true}}, specs)
	}

	// Non-numeric and out-of-range ports.
	_, err = parsePorts("abc")
	assert.Error(err)
	_, err = parsePorts("70000")
	assert.Error(err)
}

// TestHttpingress_ParsePortsLegacyCert exercises the bare-port TLS auto-detection, which os.Stats
// httpingress-{port}-cert.pem / -key.pem in the working directory.
func TestHttpingress_ParsePortsLegacyCert(t *testing.T) {
	// No parallel - changes CWD
	assert := testarossa.For(t)
	tmp := t.TempDir()
	// parsePorts only checks file existence; the content does not need to be a real certificate.
	err := os.WriteFile(filepath.Join(tmp, "httpingress-8443-cert.pem"), nil, 0o600)
	if err != nil {
		t.Fatalf("write cert placeholder: %v", err)
	}
	err = os.WriteFile(filepath.Join(tmp, "httpingress-8443-key.pem"), nil, 0o600)
	if err != nil {
		t.Fatalf("write key placeholder: %v", err)
	}
	err = os.WriteFile(filepath.Join(tmp, "httpingress-80-cert.pem"), nil, 0o600)
	if err != nil {
		t.Fatalf("write port-80 cert placeholder: %v", err)
	}
	err = os.WriteFile(filepath.Join(tmp, "httpingress-80-key.pem"), nil, 0o600)
	if err != nil {
		t.Fatalf("write port-80 key placeholder: %v", err)
	}

	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	err = os.Chdir(tmp)
	if err != nil {
		t.Fatalf("chdir: %v", err)
	}

	// A bare port enables TLS when its legacy port-named cert/key files are present.
	specs, err := parsePorts("8443")
	if assert.NoError(err) {
		assert.Equal([]portSpec{{8443, true}}, specs)
	}

	// Port 80 stays plaintext even though httpingress-80-*.pem exist.
	specs, err = parsePorts("80")
	if assert.NoError(err) {
		assert.Equal([]portSpec{{80, false}}, specs)
	}

	// A bare port without legacy files stays plaintext.
	specs, err = parsePorts("9999")
	if assert.NoError(err) {
		assert.Equal([]portSpec{{9999, false}}, specs)
	}
}

func TestHttpingress_ParseAllowedInternalPorts(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	// Empty string yields an empty set, not an error.
	set, err := parseAllowedInternalPorts("")
	if assert.NoError(err) {
		assert.Equal(0, len(set))
	}

	// Single port and a range; whitespace tolerated.
	set, err = parseAllowedInternalPorts(" 1234 , 10000-10002 ")
	if assert.NoError(err) {
		assert.Equal(4, len(set))
		assert.True(set[1234])
		assert.True(set[10000])
		assert.True(set[10001])
		assert.True(set[10002])
	}

	// Range bounds at the edges of the allowed window.
	set, err = parseAllowedInternalPorts("1024, 65535")
	if assert.NoError(err) {
		assert.True(set[1024])
		assert.True(set[65535])
	}

	// Errors: below the floor, above the ceiling, inverted range, non-numeric.
	for _, bad := range []string{"1023", "65536", "999-1100", "5000-4000", "abc", "100-200"} {
		_, err = parseAllowedInternalPorts(bad)
		assert.Error(err)
	}
}

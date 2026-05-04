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

package llm

import (
	"testing"

	"github.com/microbus-io/testarossa"
)

func TestCanonicalizeToolURL(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		raw      string
		hostPort string
		docKey   string
	}{
		{
			name:     "explicit https port 443",
			raw:      "https://calculator.example:443/arithmetic",
			hostPort: "calculator.example:443",
			docKey:   "/calculator.example:443/arithmetic",
		},
		{
			name:     "missing port defaults to 443",
			raw:      "https://calculator.example/arithmetic",
			hostPort: "calculator.example:443",
			docKey:   "/calculator.example:443/arithmetic",
		},
		{
			name:     "missing scheme and port",
			raw:      "calculator.example/arithmetic",
			hostPort: "calculator.example:443",
			docKey:   "/calculator.example:443/arithmetic",
		},
		{
			name:     "http scheme defaults to 80",
			raw:      "http://calculator.example/arithmetic",
			hostPort: "calculator.example:80",
			docKey:   "/calculator.example:80/arithmetic",
		},
		{
			name:     "non-default internal port preserved",
			raw:      "https://calc.example:444/foo",
			hostPort: "calc.example:444",
			docKey:   "/calc.example:444/foo",
		},
		{
			name:     "named path arg passes through",
			raw:      "https://yellowpages.example:443/persons/{key}",
			hostPort: "yellowpages.example:443",
			docKey:   "/yellowpages.example:443/persons/{key}",
		},
		{
			name:     "greedy path arg has dots stripped",
			raw:      "https://files.example:443/load/{path...}",
			hostPort: "files.example:443",
			docKey:   "/files.example:443/load/{path}",
		},
		{
			name:     "greedy path arg mid-route",
			raw:      "https://files.example:443/load/{category}/{name...}",
			hostPort: "files.example:443",
			docKey:   "/files.example:443/load/{category}/{name}",
		},
		{
			name:     "anonymous path arg gets implicit name",
			raw:      "https://svc.example:443/path/{}",
			hostPort: "svc.example:443",
			docKey:   "/svc.example:443/path/{path1}",
		},
		{
			name:     "multiple anonymous path args numbered in order",
			raw:      "https://svc.example:443/path/{}/sub/{}",
			hostPort: "svc.example:443",
			docKey:   "/svc.example:443/path/{path1}/sub/{path2}",
		},
		{
			name:     "anonymous index counts all path args",
			raw:      "https://svc.example:443/a/{x}/b/{}/c/{y}/d/{}",
			hostPort: "svc.example:443",
			docKey:   "/svc.example:443/a/{x}/b/{path2}/c/{y}/d/{path4}",
		},
		{
			name:     "empty path becomes root",
			raw:      "https://svc.example:443",
			hostPort: "svc.example:443",
			docKey:   "/svc.example:443/",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert := testarossa.For(t)
			hostPort, docKey, err := canonicalizeToolURL(tc.raw)
			assert.NoError(err)
			assert.Equal(tc.hostPort, hostPort)
			assert.Equal(tc.docKey, docKey)
		})
	}
}

func TestCanonicalizeToolURL_Errors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		raw  string
	}{
		{name: "missing hostname", raw: ":443/path"},
		{name: "empty string", raw: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert := testarossa.For(t)
			_, _, err := canonicalizeToolURL(tc.raw)
			assert.Error(err)
		})
	}
}

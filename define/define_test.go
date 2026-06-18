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

package define

import (
	"testing"

	"github.com/microbus-io/fabric/httpx"
)

// TestURL_MatchesHttpx pins joinHostAndPath to httpx.JoinHostAndPath, guarding the verbatim copy
// against drift. The define package itself must not import httpx; the test may.
func TestURL_MatchesHttpx(t *testing.T) {
	host := "svc.example"
	routes := []string{
		"",
		":443/my-func",
		":444/internal",
		"/dashboard",
		"//alt.host:0/alt",
		"//root",
		"path/no/slash",
		"https://external.example/x",
	}
	for _, route := range routes {
		got := Function{Host: host, Route: route}.URL()
		want := httpx.JoinHostAndPath(host, route)
		if got != want {
			t.Errorf("route %q: URL()=%q, httpx=%q", route, got, want)
		}
	}
}

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

package openapi

import (
	"testing"

	"github.com/microbus-io/testarossa"
)

func TestEndpoint_Port(t *testing.T) {
	t.Parallel()

	cases := []struct {
		route string
		want  string
	}{
		{"", "443"},
		{"/", "443"},
		{"/foo", "443"},
		{"/foo/bar", "443"},
		{"//root", "443"},
		{"//host.example/path", "443"},
		{"//host.example:1080/path", "1080"},
		{"//host.example:1080", "1080"},
		{":443", "443"},
		{":443/foo", "443"},
		{":1080", "1080"},
		{":1080/foo", "1080"},
		{":417", "417"},
		{":428/my-task", "428"},
		{":444/internal", "444"},
		{":888/management", "888"},
		{":0", "0"},
		{":0/openapi.json", "0"},
		{"https://example.com/path", "443"},
		{"https://example.com:8443/path", "8443"},
		{"http://example.com/path", "80"},
		{"http://example.com:8080/path", "8080"},
	}
	for _, tc := range cases {
		t.Run(tc.route, func(t *testing.T) {
			assert := testarossa.For(t)
			ep := &Endpoint{Route: tc.route}
			assert.Expect(ep.Port(), tc.want)
		})
	}
}

func TestEndpoint_URL(t *testing.T) {
	t.Parallel()

	cases := []struct {
		hostname string
		route    string
		want     string
	}{
		{"my.service", "/foo", "https://my.service/foo"},
		{"my.service", ":1080/foo", "https://my.service:1080/foo"},
		{"my.service", "", "https://my.service"},
		{"my.service", "//root", "https://root"},
		{"my.service", "//host.example/path", "https://host.example/path"},
		{"my.service", "https://other.host/path", "https://other.host/path"},
	}
	for _, tc := range cases {
		t.Run(tc.hostname+" "+tc.route, func(t *testing.T) {
			assert := testarossa.For(t)
			ep := &Endpoint{Hostname: tc.hostname, Route: tc.route}
			assert.Expect(ep.URL(), tc.want)
		})
	}
}

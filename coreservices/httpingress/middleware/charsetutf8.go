/*
Copyright (c) 2023-2025 Microbus LLC and various contributors

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

package middleware

import (
	"net/http"
	"strings"

	"github.com/microbus-io/fabric/connector"
)

// CharsetUTF8 returns a middleware that augments the Content-Type header of text/* and application/json with the UTF-8 charset.
func CharsetUTF8() Middleware {
	return func(next connector.HTTPHandler) connector.HTTPHandler {
		return func(w http.ResponseWriter, r *http.Request) (err error) {
			err = next(w, r)
			contentType := strings.ToLower(w.Header().Get("Content-Type"))
			if contentType == "application/json" ||
				(strings.HasPrefix(contentType, "text/") && !strings.Contains(contentType, ";")) {
				w.Header().Set("Content-Type", w.Header().Get("Content-Type")+"; charset=utf-8")
			}
			return err // No trace
		}
	}
}

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

package middleware

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/httpx"
)

// ErrorPageRedirect returns a middleware that redirects HTTP errors to an error page.
// Only requests coming from a browser as indicated by Sec-Fetch-Mode:navigate and Sec-Fetch-Dest:document are redirected.
// The URL of the original page is passed in the "src" parameter to the error page.
func ErrorPageRedirect(statusCode int, errorPagePath string) Middleware {
	return func(next connector.HTTPHandler) connector.HTTPHandler {
		errorPagePath = "/" + strings.TrimLeft(errorPagePath, "/")
		return func(w http.ResponseWriter, r *http.Request) (err error) {
			// Ignore requests not coming from a browser for a top-level document
			browser := strings.HasPrefix(r.Header.Get("User-Agent"), "Mozilla/")
			mode := r.Header.Get("Sec-Fetch-Mode")
			if mode == "" {
				mode = "navigate"
			}
			dest := r.Header.Get("Sec-Fetch-Dest")
			if dest == "" {
				dest = "document"
			}
			if !browser || mode != "navigate" || dest != "document" {
				return next(w, r) // No trace
			}
			// Delegate the request downstream
			ww := httpx.NewResponseRecorder()
			err = next(ww, r)
			res := ww.Result()
			if res.StatusCode == statusCode || errors.StatusCode(err) == statusCode {
				parts := strings.Split(r.URL.String(), "/")
				srcParam := ""
				if len(parts) > 3 {
					srcParam = "?src=" + url.QueryEscape("/"+strings.Join(parts[3:], "/"))
				}
				http.Redirect(w, r, errorPagePath+srcParam, http.StatusTemporaryRedirect)
				return nil
			}
			if err != nil {
				return err // No trace
			}
			err = httpx.Copy(w, res)
			return errors.Trace(err)
		}
	}
}

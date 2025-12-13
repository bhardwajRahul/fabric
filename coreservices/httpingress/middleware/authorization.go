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
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/fabric/utils"
)

// Authorization returns a middleware that looks for a token in the "Authorization: Bearer" header or the "Authorization" cookie.
// If the token is validated with its issuer, it assicuates the corresponding actor with the request.
func Authorization(tokenValidator func(ctx context.Context, token string) (actor any, valid bool, err error)) Middleware {
	return func(next connector.HTTPHandler) connector.HTTPHandler {
		return func(w http.ResponseWriter, r *http.Request) (err error) {
			// Look for authorization token
			token := ""
			if c, _ := r.Cookie("Authorization"); c != nil {
				token = c.Value
			}
			authorizationHeader := r.Header.Get("Authorization")
			if strings.HasPrefix(authorizationHeader, "Bearer ") {
				token = authorizationHeader[7:]
			}
			if token != "" {
				// Validate the token
				actor, valid, err := tokenValidator(r.Context(), token) // Callback
				if err != nil {
					return errors.Trace(err)
				}
				// Set the actor header
				if valid && actor != nil {
					if str, ok := actor.(string); ok && strings.HasPrefix(str, "{") && strings.HasSuffix(str, "}") {
						r.Header.Set(frame.HeaderActor, str)
					} else {
						buf, err := json.Marshal(actor)
						if err != nil {
							return errors.Trace(err)
						}
						buf = bytes.TrimSpace(buf)
						str := utils.UnsafeBytesToString(buf)
						r.Header.Set(frame.HeaderActor, str)
					}
				}
			}

			err = next(w, r)
			return err // No trace
		}
	}
}

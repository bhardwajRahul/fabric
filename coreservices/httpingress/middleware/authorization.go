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
	"context"
	"net/http"
	"strings"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/frame"
)

// Authorization returns a middleware that looks for a token in the "Authorization: Bearer" header or the "Authorization" cookie.
// If the external bearer token is validated with its issuer, the exchange callback returns a signed internal access token JWT to set as the actor.
func Authorization(exchange func(ctx context.Context, bearerToken string) (accessToken string, err error)) Middleware {
	return func(next connector.HTTPHandler) connector.HTTPHandler {
		return func(w http.ResponseWriter, r *http.Request) (err error) {
			// Look for authorization bearerToken
			bearerToken := ""
			if c, _ := r.Cookie("Authorization"); c != nil {
				bearerToken = c.Value
			}
			authorizationHeader := r.Header.Get("Authorization")
			if strings.HasPrefix(authorizationHeader, "Bearer ") {
				bearerToken = authorizationHeader[7:]
			}
			if bearerToken != "" {
				// Validate the token and mint an internal JWT
				accessToken, err := exchange(r.Context(), bearerToken) // Callback
				if err != nil {
					return errors.Trace(err)
				}
				// Set the actor header
				if accessToken != "" {
					err = frame.Of(r).SetToken(accessToken)
					if err != nil {
						return errors.Trace(err)
					}
				}
			}

			err = next(w, r)
			return err // No trace
		}
	}
}

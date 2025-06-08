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
	"encoding/json"
	"fmt"
	"net/http"
	"unicode"

	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/errors"
	"github.com/microbus-io/fabric/httpx"
	"go.opentelemetry.io/otel/trace"
)

// ErrorPrinter returns a middleware that outputs any error to the response body.
// It should typically be the first middleware.
// Error details and stack trace are only printed on localhost.
func ErrorPrinter(deployment func() string) Middleware {
	return func(next connector.HTTPHandler) connector.HTTPHandler {
		return func(w http.ResponseWriter, r *http.Request) (err error) {
			ww := httpx.NewResponseRecorder()
			downstreamErr := next(ww, r) // No trace
			if downstreamErr == nil {
				err = httpx.Copy(w, ww.Result())
				return errors.Trace(err)
			}
			tracedError := errors.Convert(downstreamErr)

			// Status code
			statusCode := tracedError.StatusCode
			if statusCode <= 0 || statusCode >= 1000 {
				statusCode = http.StatusInternalServerError
			}
			// Enrich with trace ID
			span := trace.SpanFromContext(r.Context())
			if span != nil {
				traceID := span.SpanContext().TraceID().String()
				if tracedError.Properties == nil {
					tracedError.Properties = map[string]any{}
				}
				tracedError.Properties["trace"] = traceID
			}
			local := deployment() == connector.LOCAL
			var printedError *errors.TracedError
			if local {
				printedError = tracedError
			} else {
				printedError = &errors.TracedError{
					StatusCode: statusCode,
					Stack:      nil, // Redact stack trace
					Properties: make(map[string]any, len(tracedError.Properties)),
				}
				// Only reveal 4xx errors to external users
				if statusCode < 400 || statusCode >= 500 {
					printedError.Err = fmt.Errorf("internal server error")
				} else {
					printedError.Err = tracedError.Err
				}
				// Redact lowercase properties
				for k, v := range tracedError.Properties {
					if k != "" && unicode.IsUpper([]rune(k)[0]) {
						printedError.Properties[k] = v
					}
				}
			}
			encoder := json.NewEncoder(w)
			encoder.SetIndent("", "  ")
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "no-store")
			w.WriteHeader(statusCode)
			err = encoder.Encode(printedError)
			if err != nil {
				return errors.Trace(err)
			}
			return nil
		}
	}
}

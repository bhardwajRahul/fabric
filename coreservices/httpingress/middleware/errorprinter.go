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
	"strings"
	"unicode"

	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/errors"
	"github.com/microbus-io/fabric/httpx"
	"go.opentelemetry.io/otel/trace"
)

/*
ErrorPrinter returns a middleware that outputs any error to the response body.
It should typically be the first middleware in the chain in case any of the other middleware fail.
Error details and stack trace can be optionally redacted.

The printer outputs the error as a JSON object nested the property "err" of the root object,
with its nested properties up-leveled as if they are properties of the error.

	{
		"err": {
			"error": "message",
			"stack": [...],
			"statusCode": 500,
			"trace": "0123456789abcdef0123456789abcdef",
			"propName": "propValue"
		}
	}
*/
func ErrorPrinter(redact func() bool) Middleware {
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
			var printedError *errors.TracedError
			if !redact() {
				printedError = tracedError
			} else {
				printedError = &errors.TracedError{
					StatusCode: statusCode,
					Stack:      nil, // Redact stack trace
					Trace:      tracedError.Trace,
					Properties: make(map[string]any, len(tracedError.Properties)),
				}
				// Only reveal 4xx errors to external users
				if statusCode < 400 || statusCode >= 500 {
					printedError.Err = fmt.Errorf("internal server error")
				} else {
					printedError.Err = tracedError.Err
				}
				// Redact properties that start with underscore
				for k, v := range tracedError.Properties {
					if !strings.HasPrefix(k, "_") {
						printedError.Properties[k] = v
					}
				}
			}
			encoder := json.NewEncoder(w)
			encoder.SetIndent("", "  ")
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "no-cache, no-store, max-age=0")
			w.WriteHeader(statusCode)
			serializedErr := struct {
				Err error `json:"err"`
			}{
				Err: printedError,
			}
			err = encoder.Encode(serializedErr)
			if err != nil {
				return errors.Trace(err)
			}
			return nil
		}
	}
}

/*
LegacyErrorPrinter returns a middleware that outputs any error to the response body.
It should typically be the first middleware in the chain in case any of the other middleware fail.
Error details and stack trace are only printed on localhost.

The printer outputs the error as a root JSON object.

	{
		"error": "message",
		"properties": {
			"trace": "0123456789abcdef0123456789abcdef",
			"propName": "propValue"
		}
		"stack": [...],
		"statusCode": 500,
	}

Deprecated: Use the new ErrorPrinter
*/
func LegacyErrorPrinter(deployment func() string) Middleware {
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

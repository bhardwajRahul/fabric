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

package httpx

import (
	"encoding/json"
	"net/http"

	"github.com/microbus-io/errors"
)

// ParseRequestBody parses the body of an incoming request and populates the fields of a data object.
// It supports JSON and URL-encoded form data content types.
// Use json tags to designate the name of the argument to map to each field.
func ParseRequestBody(r *http.Request, data any) error {
	// Parse JSON in the body
	contentType := r.Header.Get("Content-Type")
	if contentType == "application/json" {
		err := json.NewDecoder(r.Body).Decode(&data)
		if err != nil {
			return errors.Trace(err)
		}
	}
	// Parse form in body
	if contentType == "application/x-www-form-urlencoded" {
		err := r.ParseForm()
		if err != nil {
			return errors.Trace(err)
		}
		err = DecodeDeepObject(r.PostForm, data)
		if err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

// ParseRequestData parses the body and query arguments of an incoming request
// and populates the fields of a data object.
// Use json tags to designate the name of the argument to map to each field.
// An argument name can be hierarchical using either notation "a[b][c]" or "a.b.c",
// in which case it is read into the corresponding nested field.
//
// Deprecated: No longer used.
func ParseRequestData(r *http.Request, data any) error {
	err := ParseRequestBody(r, data)
	if err == nil {
		err = DecodeDeepObject(r.URL.Query(), data)
	}
	return errors.Trace(err)
}

// ParseRequest parses the body, path arguments, and query arguments of an incoming request and populates
// the fields of the respective data objects. In most cases, all three references point to the same data object.
// The body can contain a JSON payload or URL-encoded form data.
// Use JSON tags to designate the name of the argument to map to each field.
// An argument name can be hierarchical using either notation "a[b][c]" or "a.b.c",
// in which case it is read into the corresponding nested field.
func ParseRequest(r *http.Request, route string, pathArgs any, body any, queryArgs any) (err error) {
	pathValues, err := PathValues(r, JoinHostAndPath("host", route))
	if err != nil {
		return errors.Trace(err)
	}
	err = DecodeDeepObject(pathValues, pathArgs)
	if err != nil {
		return errors.Trace(err)
	}
	err = ParseRequestBody(r, body)
	if err != nil {
		return errors.Trace(err)
	}
	err = DecodeDeepObject(r.URL.Query(), queryArgs)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

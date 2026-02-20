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
	"net/url"
	"reflect"

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

// ReadInputPayload parses the body, path arguments, and query arguments of an incoming request and populates them into an object.
// The body can contain a JSON payload or URL-encoded form data. Use JSON tags to designate the name of the argument to map to each field.
// An argument name can be hierarchical using either notation "a[b][c]" or "a.b.c", in which case it is read into the corresponding nested field.
func ReadInputPayload(r *http.Request, route string, in any) (err error) {
	pathValues, err := PathValues(r, JoinHostAndPath("host", route))
	if err != nil {
		return errors.Trace(err)
	}
	err = DecodeDeepObject(pathValues, in)
	if err != nil {
		return errors.Trace(err)
	}
	// If body has an HTTPRequestBody field, parse into that field instead
	bodyTarget := in
	v := reflect.ValueOf(in)
	if v.Kind() == reflect.Ptr && !v.IsNil() {
		v = v.Elem()
		if v.Kind() == reflect.Struct {
			f := v.FieldByName("HTTPRequestBody")
			if f.IsValid() && f.CanAddr() {
				bodyTarget = f.Addr().Interface()
			}
		}
	}
	err = ParseRequestBody(r, bodyTarget)
	if err != nil {
		return errors.Trace(err)
	}
	err = DecodeDeepObject(r.URL.Query(), in)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// MethodWithBody returns true if the HTTP method typically accepts a request body.
func MethodWithBody(method string) bool {
	switch method {
	case http.MethodGet, http.MethodDelete, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return false
	default:
		return true
	}
}

// WriteInputPayload determines how to deliver the input payload, via query arguments or the body of the request.
// It enables the HTTPRequestBody magic argument.
func WriteInputPayload(method string, in any) (query url.Values, body any, err error) {
	if MethodWithBody(method) {
		bodySource := in
		v := reflect.ValueOf(in)
		if v.Kind() == reflect.Pointer {
			v = v.Elem()
		}
		if v.Kind() == reflect.Struct {
			if f := v.FieldByName("HTTPRequestBody"); f.IsValid() {
				bodySource = f.Interface()
				query, err = EncodeDeepObject(in)
				if err != nil {
					return nil, nil, errors.Trace(err)
				}
				delete(query, "HTTPRequestBody")
			}
		}
		body = bodySource
	} else {
		query, err = EncodeDeepObject(in)
		if err != nil {
			return nil, nil, errors.Trace(err)
		}
	}
	return query, body, nil
}

// ReadOutputPayload reads the HTTP response into the output payload.
// It enables the HTTPResponseBody and HTTPStatusCode magic arguments.
func ReadOutputPayload(res *http.Response, out any) (err error) {
	_decodeTarget := any(out)
	v := reflect.ValueOf(out)
	if v.Kind() == reflect.Ptr && !v.IsNil() {
		if f := v.Elem().FieldByName("HTTPResponseBody"); f.IsValid() && f.CanAddr() {
			_decodeTarget = f.Addr().Interface()
		}
		if f := v.Elem().FieldByName("HTTPStatusCode"); f.IsValid() && f.CanSet() {
			f.SetInt(int64(res.StatusCode))
		}
	}
	if res.Body != nil {
		err = json.NewDecoder(res.Body).Decode(_decodeTarget)
		if err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

// WriteOutputPayload writes the output payload to the HTTP response as JSON.
// It enables the HTTPStatusCode and HTTPResponseBody magic arguments.
func WriteOutputPayload(w http.ResponseWriter, out any) (err error) {
	w.Header().Set("Content-Type", "application/json")
	v := reflect.ValueOf(out)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() == reflect.Struct {
		if f := v.FieldByName("HTTPStatusCode"); f.IsValid() {
			w.WriteHeader(int(f.Int()))
		}
	}
	encoder := json.NewEncoder(w)
	if v.Kind() == reflect.Struct {
		if f := v.FieldByName("HTTPResponseBody"); f.IsValid() {
			err = encoder.Encode(f.Interface())
		} else {
			err = encoder.Encode(out)
		}
	} else {
		err = encoder.Encode(out)
	}
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

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

package httpegress

import (
	"bufio"
	"bytes"
	"compress/flate"
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"strconv"

	"github.com/andybalholm/brotli"
	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/trc"
)

/*
Service implements the http.egress.core microservice.

The HTTP egress microservice relays HTTP requests to the internet.
*/
type Service struct {
	*Intermediate // IMPORTANT: Do not remove

	// HINT: Add member variables here
}

// OnStartup is called when the microservice is started up.
func (svc *Service) OnStartup(ctx context.Context) (err error) {
	return nil
}

// OnShutdown is called when the microservice is shut down.
func (svc *Service) OnShutdown(ctx context.Context) (err error) {
	return nil
}

/*
MakeRequest proxies a request to a URL and returns the HTTP response, respecting the timeout set in the context.
The proxied request is expected to be posted in the body of the request in binary form (RFC7231).
*/
func (svc *Service) MakeRequest(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: MakeRequest
	ctx := r.Context()
	req, err := http.ReadRequest(bufio.NewReaderSize(r.Body, 64))
	if err != nil {
		return errors.Trace(err)
	}
	if req.URL.Port() == "" {
		switch req.URL.Scheme {
		case "https":
			req.URL.Host += ":443"
		case "http":
			req.URL.Host += ":80"
		}
	}
	req.RequestURI = "" // Avoid "http: Request.RequestURI can't be set in client requests"
	req.Header.Set("Accept-Encoding", "br;q=1.0,deflate;q=0.8,gzip;q=0.6")

	// OpenTelemetry: create a child span
	spanOptions := []trc.Option{
		trc.Client(),
		// Do not record the request attributes yet because they take a lot of memory, they will be added if there's an error
	}
	if svc.Deployment() == connector.LOCAL {
		// Add the request attributes in LOCAL deployment to facilitate debugging
		spanOptions = append(spanOptions, trc.Request(r))
	}
	_, span := svc.StartSpan(ctx, req.URL.Hostname(), spanOptions...)
	spanEnded := false
	defer func() {
		if !spanEnded {
			span.End()
		}
	}()

	client := http.Client{}
	resp, err := client.Do(req)
	if err == nil {
		// Decompress as required
		var decompressed bytes.Buffer
		isCompressed := true
		switch resp.Header.Get("Content-Encoding") {
		case "br":
			rdr := brotli.NewReader(resp.Body)
			_, err = io.Copy(&decompressed, rdr)
			if err != nil {
				err = errors.Trace(err)
			}
		case "deflate":
			rdr := flate.NewReader(resp.Body)
			_, err = io.Copy(&decompressed, rdr)
			if err != nil {
				err = errors.Trace(err)
			}
			rdr.Close()
		case "gzip":
			var rdr *gzip.Reader
			rdr, err = gzip.NewReader(resp.Body)
			if err != nil {
				err = errors.Trace(err)
			} else {
				_, err = io.Copy(&decompressed, rdr)
				if err != nil {
					err = errors.Trace(err)
				}
				rdr.Close()
			}
		default:
			isCompressed = false
		}
		if err == nil {
			if isCompressed {
				resp.Header.Del("Content-Encoding")
				resp.Header.Set("Content-Length", strconv.Itoa(decompressed.Len()))
			}
			for k, vv := range resp.Header {
				for _, v := range vv {
					w.Header().Add(k, v)
				}
			}
			w.WriteHeader(resp.StatusCode)
			if isCompressed {
				_, err = io.Copy(w, &decompressed)
			} else {
				_, err = io.Copy(w, resp.Body)
			}
		}
	}
	if err != nil {
		// OpenTelemetry: record the error, adding the request attributes
		span.SetRequest(req)
		span.SetError(err)
		svc.ForceTrace(ctx)
	} else {
		span.SetOK(http.StatusOK)
	}
	span.End()
	spanEnded = true
	return errors.Trace(err)
}

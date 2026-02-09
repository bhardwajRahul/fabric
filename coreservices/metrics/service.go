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

package metrics

import (
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/coreservices/control/controlapi"
	"github.com/microbus-io/fabric/coreservices/metrics/metricsapi"
	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/fabric/pub"
)

var (
	_ errors.TracedError
	_ http.Request
	_ metricsapi.Client
)

/*
Service implements the metrics.core microservice.

The Metrics service is a core microservice that aggregates metrics from other microservices and makes them available for collection.
*/
type Service struct {
	*Intermediate // IMPORTANT: Do not remove
}

// OnStartup is called when the microservice is started up.
func (svc *Service) OnStartup(ctx context.Context) (err error) {
	return
}

// OnShutdown is called when the microservice is shut down.
func (svc *Service) OnShutdown(ctx context.Context) (err error) {
	return
}

/*
Collect returns the latest aggregated metrics.
*/
func (svc *Service) Collect(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Collect
	if svc.SecretKey() == "" && svc.Deployment() != connector.LOCAL && svc.Deployment() != connector.TESTING {
		return errors.New("secret key required")
	}

	secretKey := r.URL.Query().Get("secretKey")
	if secretKey == "" {
		secretKey = r.URL.Query().Get("secretkey")
	}
	if secretKey == "" {
		secretKey = r.URL.Query().Get("secret_key")
	}
	if secretKey != svc.SecretKey() && svc.SecretKey() != "" {
		return errors.New("incorrect secret key", http.StatusNotFound)
	}

	host := r.URL.Query().Get("service")
	if host == "" {
		host = "all"
	}
	err = httpx.ValidateHostname(host)
	if err != nil {
		return errors.Trace(err)
	}

	// Compress, except on local to avoid special characters when running NATS is debug mode
	var writer io.Writer
	var wCloser io.Closer
	writer = w
	if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") && svc.Deployment() != connector.LOCAL {
		zipper, _ := gzip.NewWriterLevel(w, gzip.BestSpeed)
		if zipper != nil {
			w.Header().Set("Content-Encoding", "gzip")
			writer = zipper
			wCloser = zipper // Gzip writer must be closed to flush buffer
		}
	}

	// Timeout
	ctx := r.Context()
	timeout := pub.Noop()
	secs := r.Header.Get("X-Prometheus-Scrape-Timeout-Seconds")
	if secs != "" {
		if s, err := strconv.Atoi(secs); err == nil {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(r.Context(), time.Duration(s)*time.Second)
			defer cancel()
		}
	}

	w.Header().Set("Content-Type", "text/plain")

	var delay time.Duration
	var mux sync.Mutex
	var wg sync.WaitGroup
	for serviceInfo := range controlapi.NewMulticastClient(svc).ForHost(host).PingServices(ctx) {
		wg.Add(1)
		go func(s string, delay time.Duration) {
			defer wg.Done()
			time.Sleep(delay) // Stagger requests to avoid all of them coming back at the same time
			u := "https://" + s + ":888/metrics"
			ch := svc.Publish(
				ctx,
				pub.GET(u),
				pub.Header("Accept-Encoding", "gzip"),
				timeout,
			)
			for i := range ch {
				res, err := i.Get()
				if err != nil {
					// Error 501 Status Not Implemented indicates that Prometheus metric collection is disabled.
					// Set the PROMETHEUS_EXPORTER_ENABLED environment variable to enable.
					svc.LogWarn(ctx, "Fetching metrics",
						"error", err,
						"targetService", s,
					)
					continue
				}
				if res.StatusCode != http.StatusOK {
					// Error 501 Status Not Implemented indicates that Prometheus metric collection is disabled.
					// Set the PROMETHEUS_EXPORTER_ENABLED environment variable to enable.
					svc.LogWarn(ctx, "Fetching metrics",
						"statusCode", res.StatusCode,
						"targetService", s,
					)
					continue
				}

				var reader io.Reader
				var rCloser io.Closer
				reader = res.Body
				rCloser = res.Body
				if res.Header.Get("Content-Encoding") == "gzip" {
					unzipper, err := gzip.NewReader(res.Body)
					if err != nil {
						svc.LogWarn(ctx, "Unzippping metrics",
							"error", err,
							"targetService", s,
						)
						continue
					}
					reader = unzipper
					rCloser = unzipper
				}

				mux.Lock()
				_, err = io.Copy(writer, reader)
				mux.Unlock()
				if err != nil {
					svc.LogWarn(ctx, "Copying metrics",
						"error", err,
						"targetService", s,
					)
				}
				rCloser.Close()
			}
		}(serviceInfo.Hostname, delay)
		delay += time.Millisecond
	}
	wg.Wait()
	writer.Write([]byte(""))
	if wCloser != nil {
		wCloser.Close()
	}

	return nil
}

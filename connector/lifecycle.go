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

package connector

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/dlru"
	"github.com/microbus-io/fabric/env"
	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/fabric/service"
	"github.com/microbus-io/fabric/trc"
	"github.com/microbus-io/fabric/utils"
	"go.opentelemetry.io/otel/trace"
)

const (
	shutDown = iota
	startingUp
	startedUp
	shuttingDown
)

// SetOnStartup adds a function to be called during the starting up of the microservice.
// Startup callbacks are called in the order they were added.
func (c *Connector) SetOnStartup(handler service.StartupHandler) error {
	if !c.isPhase(shutDown) {
		return c.captureInitErr(errors.New("already started"))
	}
	c.onStartup = append(c.onStartup, handler)
	return nil
}

// SetOnShutdown adds a function to be called during the shutting down of the microservice.
// Shutdown callbacks are called in the reverse order they were added.
func (c *Connector) SetOnShutdown(handler service.ShutdownHandler) error {
	if !c.isPhase(shutDown) {
		return c.captureInitErr(errors.New("already started"))
	}
	c.onShutdown = append(c.onShutdown, handler)
	return nil
}

// Startup the microservice by connecting to the transport and activating the subscriptions.
// The context deadline is used to limit the time allotted to the operation.
func (c *Connector) Startup(ctx context.Context) (err error) {
	if !c.phase.CompareAndSwap(shutDown, startingUp) {
		return errors.New("not shut down")
	}
	defer func() { c.phase.CompareAndSwap(startingUp, shutDown) }()
	if c.hostname == "" {
		return errors.New("hostname is not set")
	}
	defer func() { c.initErr = nil }()
	if c.initErr != nil {
		return c.initErr
	}

	// Determine the communication plane
	if c.plane == "" {
		if plane := env.Get("MICROBUS_PLANE"); plane != "" {
			err := c.SetPlane(plane)
			if err != nil {
				return errors.Trace(err)
			}
		}
		if c.plane == "" {
			testingFuncName, underTest := utils.Testing()
			if underTest {
				// Generate a unique plane from the test name
				h := sha256.New()
				h.Write([]byte(testingFuncName))
				c.plane = strings.ToLower(hex.EncodeToString(h.Sum(nil)[:8]))
			}
		}
		if c.plane == "" {
			c.plane = "microbus"
		}
	}

	// Determine the geographic locality
	if c.locality == "" {
		if locality := env.Get("MICROBUS_LOCALITY"); locality != "" {
			err := c.SetLocality(locality)
			if err != nil {
				return errors.Trace(err)
			}
		}
	}
	c.locality, err = determineCloudLocality(c.locality)
	if err != nil {
		return errors.Trace(err)
	}

	// Identify the environment deployment
	if c.deployment == "" {
		if deployment := env.Get("MICROBUS_DEPLOYMENT"); deployment != "" {
			err := c.SetDeployment(deployment)
			if err != nil {
				return errors.Trace(err)
			}
		}
		if c.deployment == "" {
			_, underTest := utils.Testing()
			if underTest {
				c.deployment = TESTING
			}
		}
		if c.deployment == "" {
			c.deployment = LOCAL
			if natsURL := env.Get("MICROBUS_NATS"); natsURL != "" {
				if !strings.Contains(natsURL, "/127.0.0.1:") &&
					!strings.Contains(natsURL, "/0.0.0.0:") &&
					!strings.Contains(natsURL, "/localhost:") {
					c.deployment = PROD
				}
			}
		}
	}

	// Call shutdown to clean up, if there's an error.
	// All errors must be assigned to err.
	span := trc.NewSpan(nil)
	startTime := time.Now()
	defer func() {
		if err != nil {
			c.LogError(ctx, "Starting up", "error", err)
			// OpenTelemetry: record the error
			span.SetError(err)
			c.ForceTrace(ctx)
		} else {
			span.SetOK(http.StatusOK)
		}
		span.End()
		_ = c.RecordHistogram(
			ctx,
			"microbus_callback_duration_seconds",
			time.Since(startTime).Seconds(),
			"handler", "startup",
			"error", func() string {
				if err != nil {
					return "ERROR"
				}
				return "OK"
			}(),
		)
		if err != nil {
			c.Shutdown(ctx)
		}
	}()
	c.onStartupCalled = false

	// OpenTelemetry: init
	err = c.initMeter(ctx)
	if err != nil {
		err = errors.Trace(err)
		return err
	}
	err = c.initTracer(ctx)
	if err != nil {
		err = errors.Trace(err)
		return err
	}

	// OpenTelemetry: create a span for the callback
	ctx, span = c.StartSpan(ctx, "startup", trc.Internal())

	// Initialize logger
	err = c.initLogger()
	if err != nil {
		err = errors.Trace(err)
		return err
	}
	c.LogInfo(ctx, "Startup")

	// Connect to the transport
	err = c.transportConn.Open(context.Background(), c)
	if err != nil {
		err = errors.Trace(err)
		return err
	}
	c.maxFragmentSize = c.transportConn.MaxPayload() - 64<<10 // Up to 64K for headers
	if c.maxFragmentSize < 64<<10 {
		err = errors.New("message size limit is too restrictive")
		return err
	}
	c.networkRoundtrip = c.transportConn.Latency()
	c.ackTimeout = c.networkRoundtrip
	c.LogInfo(ctx, "Transport latency", "latency", c.networkRoundtrip)

	// Subscribe to the response subject
	c.responseSub, err = c.transportConn.QueueSubscribe(subjectOfResponses(c.plane, c.hostname, c.id), c.id, c.onResponse)
	if err != nil {
		err = errors.Trace(err)
		return err
	}

	// Fetch configs
	err = c.refreshConfig(ctx, false)
	if err != nil {
		err = errors.Trace(err)
		return err
	}
	c.logConfigs(ctx)

	// Set up the distributed cache (before the callbacks)
	c.distribCache, err = dlru.NewCache(ctx, c, ":888/dcache")
	if err != nil {
		err = errors.Trace(err)
		return err
	}

	// Call the callback functions in order
	c.onStartupCalled = true
	for i := range c.onStartup {
		err = errors.CatchPanic(func() error {
			return c.onStartup[i](ctx)
		})
		if err != nil {
			err = errors.Trace(err)
			return err
		}
	}

	// Prepare the connector's root context
	c.lifetimeCtx, c.ctxCancel = context.WithCancel(context.Background())

	// Subscribe to :888 control messages
	err = c.subscribeControl()
	if err != nil {
		err = errors.Trace(err)
		return err
	}

	// Activate subscriptions
	err = c.activateSubs()
	if err != nil {
		err = errors.Trace(err)
		return err
	}

	// Run all tickers
	c.runTickers()

	c.startupTime = time.Now().UTC()
	c.phase.Store(startedUp)

	return nil
}

// Shutdown the microservice by deactivating subscriptions and disconnecting from the transport.
// The context deadline is used to limit the time allotted to the operation.
func (c *Connector) Shutdown(ctx context.Context) (err error) {
	if !c.phase.CompareAndSwap(startedUp, shuttingDown) && !c.phase.CompareAndSwap(startingUp, shuttingDown) {
		return errors.New("not started")
	}

	// OpenTelemetry: create a span for the callback
	ctx, span := c.StartSpan(context.Background(), "shutdown", trc.Internal())

	startTime := time.Now()
	var lastErr error

	// Stop all tickers
	err = c.stopTickers()
	if err != nil {
		lastErr = errors.Trace(err)
	}

	// Deactivate subscriptions
	err = c.deactivateSubs()
	if err != nil {
		lastErr = errors.Trace(err)
	}

	// Drain pending operations (incoming requests, running tickers, goroutines)
	totalDrainTime := time.Duration(0)
	for c.pendingOps.Load() > 0 && totalDrainTime < 8*time.Second { // 8 seconds
		time.Sleep(20 * time.Millisecond)
		totalDrainTime += 20 * time.Millisecond
	}
	undrained := c.pendingOps.Load()
	if undrained > 0 {
		c.LogInfo(ctx, "Stubborn pending operations",
			"ops", int(undrained),
		)
	}

	// Cancel the root context
	if c.ctxCancel != nil {
		c.ctxCancel()
		c.ctxCancel = nil
	}

	// Drain pending operations again after cancelling the context
	totalDrainTime = time.Duration(0)
	for c.pendingOps.Load() > 0 && totalDrainTime < 4*time.Second { // 4 seconds
		time.Sleep(20 * time.Millisecond)
		totalDrainTime += 20 * time.Millisecond
	}
	undrained = c.pendingOps.Load()
	if undrained > 0 {
		c.LogWarn(ctx, "Unable to drain pending operations",
			"ops", int(undrained),
		)
	}

	// Call the callback functions in reverse order
	if c.onStartupCalled {
		for i := len(c.onShutdown) - 1; i >= 0; i-- {
			err = errors.CatchPanic(func() error {
				return c.onShutdown[i](ctx)
			})
			if err != nil {
				lastErr = errors.Trace(err)
			}
		}
	}

	// Close the distributed cache
	if c.distribCache != nil {
		err = c.distribCache.Close(ctx)
		if err != nil {
			lastErr = errors.Trace(err)
		}
		c.distribCache = nil
	}

	// Unsubscribe from the response subject
	if c.responseSub != nil {
		err = c.responseSub.Unsubscribe()
		if err != nil {
			lastErr = errors.Trace(err)
		}
		c.responseSub = nil
	}

	// Disconnect the transport
	c.transportConn.Close()

	// Last chance to log an error
	if lastErr != nil {
		c.LogError(ctx, "Shutting down", "error", lastErr)
		// OpenTelemetry: record the error
		span.SetError(lastErr)
		c.ForceTrace(ctx)
	}
	_ = c.RecordHistogram(
		ctx,
		"microbus_callback_duration_seconds",
		time.Since(startTime).Seconds(),
		"handler", "shutdown",
		"error", func() string {
			if lastErr != nil {
				return "ERROR"
			}
			return "OK"
		}(),
	)

	// OpenTelemetry: terminate
	span.End()
	_ = c.termTracer(ctx)
	_ = c.termMeter(ctx)

	c.LogInfo(ctx, "Shutdown")
	c.phase.Store(shutDown)

	return lastErr
}

// IsStarted indicates if the microservice has been successfully started up.
func (c *Connector) IsStarted() bool {
	return c.isPhase(startedUp)
}

// isPhase checks if the microservice is in any of the indicates lifecycle phases.
func (c *Connector) isPhase(phase ...int) bool {
	actual := int(c.phase.Load())
	for _, p := range phase {
		if p == actual {
			return true
		}
	}
	return false
}

// Lifetime returns a context that gets cancelled when the microservice is shutdown.
// The Done() channel can be used to detect when the microservice is shutting down.
// In most cases the lifetime context should be used instead of the background context.
func (c *Connector) Lifetime() context.Context {
	return c.lifetimeCtx
}

// captureInitErr captures errors during the pre-start phase of the connector.
// If such an error occurs, the connector fails to start.
// This is useful since errors can be ignored during initialization.
func (c *Connector) captureInitErr(err error) error {
	if err != nil && c.initErr == nil && c.isPhase(shutDown) {
		c.initErr = err
	}
	return err
}

// Go launches a goroutine in the lifetime context of the microservice.
// Errors and panics are automatically captured and logged.
// On shutdown, the microservice will attempt to gracefully end a pending goroutine before termination.
func (c *Connector) Go(ctx context.Context, f func(ctx context.Context) (err error)) error {
	if !c.isPhase(startedUp, startingUp) {
		return errors.New("not started")
	}
	c.pendingOps.Add(1)
	subCtx := frame.ContextWithClonedFrameOf(c.Lifetime(), ctx)        // Copy the original frame headers
	subCtx = trace.ContextWithSpan(subCtx, trace.SpanFromContext(ctx)) // Copy the tracing context
	subCtx, span := c.StartSpan(subCtx, "Goroutine", trc.Consumer())

	go func() {
		err := errors.CatchPanic(func() error {
			return errors.Trace(f(subCtx))
		})
		if err != nil {
			c.LogError(subCtx, "Goroutine", "error", err)
		}
		c.pendingOps.Add(-1)
		span.End()
	}()
	return nil
}

// Parallel executes multiple jobs in parallel and returns the first error it encounters.
// It is a convenient pattern for calling multiple other microservices and thus amortize the network latency.
// There is no mechanism to identify the failed jobs so this pattern isn't suited for jobs that
// update data and require to be rolled back on failure.
func (c *Connector) Parallel(jobs ...func() (err error)) error {
	n := len(jobs)
	errChan := make(chan error, n)
	var wg sync.WaitGroup
	wg.Add(n)
	c.pendingOps.Add(int32(n))
	for _, j := range jobs {
		j := j
		go func() {
			defer c.pendingOps.Add(-1)
			defer wg.Done()
			errChan <- errors.CatchPanic(j)
		}()
	}
	wg.Wait()
	close(errChan)
	for e := range errChan {
		if e != nil {
			return e // NoTrace
		}
	}
	return nil
}

// determineCloudLocality determines the locality from the instance meta-data when hosted on AWS or GCP.
func determineCloudLocality(cloudProvider string) (locality string, err error) {
	var httpReq *http.Request
	switch strings.ToUpper(cloudProvider) {
	case "AWS":
		// https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/instancedata-data-retrieval.html
		// https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/instancedata-data-categories.html
		httpReq, _ = http.NewRequest("GET", "http://169.254.169.254/latest/meta-data/placement/availability-zone", nil)
	case "GCP":
		// https://cloud.google.com/compute/docs/metadata/querying-metadata
		// https://cloud.google.com/compute/docs/metadata/predefined-metadata-keys
		httpReq, _ = http.NewRequest("GET", "http://metadata.google.internal/computeMetadata/v1/instance/zone", nil)
		httpReq.Header.Set("Metadata-Flavor", "Google")
	default:
		return cloudProvider, nil
	}

	client := http.Client{
		Timeout: 2 * time.Second,
	}
	res, err := client.Do(httpReq)
	if err != nil {
		return "", errors.Trace(err)
	}
	if res.StatusCode != http.StatusOK {
		return "", errors.New("determining %s AZ", strings.ToUpper(cloudProvider))
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", errors.Trace(err)
	}
	az := string(body)

	if cloudProvider == "AWS" {
		// az == us-east-1a
		az = az[:len(az)-1] + "-" + az[len(az)-1:] // us-east-1-a
	}

	if cloudProvider == "GCP" {
		// az == projects/415104041262/zones/us-east1-a
		_, az, _ = strings.Cut(az, "/zones/") // us-east1-a
		for i := range az {
			if az[i] >= '0' && az[i] <= '9' {
				az = az[:i] + "-" + az[i:] // us-east-1-a
				break
			}
		}
	}

	parts := strings.Split(az, "-") // [us, east, 1, a]
	for i := range len(parts) / 2 {
		parts[i], parts[len(parts)-1-i] = parts[len(parts)-1-i], parts[i]
	} // [a, 1, east, us]
	return strings.Join(parts, "."), nil // a.1.east.us
}

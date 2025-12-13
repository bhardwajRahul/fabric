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
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/testarossa"
)

func TestConnector_ClockOffset(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	// Create the microservices
	alpha := New("alpha.clock.offset.connector")

	var betaTime time.Time
	var betaShift time.Duration
	beta := New("beta.clock.offset.connector")
	beta.Subscribe("GET", "shift", func(w http.ResponseWriter, r *http.Request) error {
		ctx := r.Context()
		betaTime = beta.Now(ctx)
		betaShift = frame.Of(ctx).ClockShift()
		beta.GET(r.Context(), "https://gamma.clock.offset.connector/shift")
		return nil
	})

	var gammaTime time.Time
	var gammaShift time.Duration
	gamma := New("gamma.clock.offset.connector")
	gamma.Subscribe("GET", "shift", func(w http.ResponseWriter, r *http.Request) error {
		ctx := r.Context()
		gammaTime = beta.Now(ctx)
		gammaShift = frame.Of(ctx).ClockShift()
		return nil
	})

	// Startup the microservices
	err := alpha.Startup()
	assert.NoError(err)
	defer alpha.Shutdown()
	err = beta.Startup()
	assert.NoError(err)
	defer beta.Shutdown()
	err = gamma.Startup()
	assert.NoError(err)
	defer gamma.Shutdown()

	// Shift the time in the context one minute in the past
	ctx := frame.CloneContext(context.Background())
	f := frame.Of(ctx)
	f.SetClockShift(-time.Minute)
	assert.Equal(-time.Minute, f.ClockShift())

	// Send message and validate that beta receives the offset time
	realTime := time.Now().UTC()
	time.Sleep(10 * time.Millisecond)
	alphaTime := alpha.Now(ctx) // Offset by -1m
	assert.True(alphaTime.Before(realTime))
	_, err = alpha.GET(ctx, "https://beta.clock.offset.connector/shift")
	assert.NoError(err)
	assert.True(betaTime.Before(realTime))
	assert.True(gammaTime.Before(realTime))
	assert.Equal(-time.Minute, betaShift)
	assert.Equal(-time.Minute, gammaShift)

	// Shift the time in the context one hour in the future
	ctx = frame.CloneContext(context.Background())
	f = frame.Of(ctx)
	f.SetClockShift(15 * time.Minute)
	f.IncrementClockShift(45 * time.Minute)
	f.SetClockShift(time.Hour)

	// Send message and validate that beta receives the offset time
	realTime = time.Now().UTC()
	alphaTime = alpha.Now(ctx) // Offset by +1h
	assert.True(alphaTime.After(realTime.Add(time.Minute)))
	_, err = alpha.GET(ctx, "https://beta.clock.offset.connector/shift")
	assert.NoError(err)
	assert.True(betaTime.After(realTime.Add(59 * time.Minute)))
	assert.True(gammaTime.After(realTime.Add(59 * time.Minute)))
	assert.Equal(time.Hour, betaShift)
	assert.Equal(time.Hour, gammaShift)
}

func TestConnector_Ticker(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	con := New("ticker.connector")
	con.SetDeployment(LAB) // Tickers are disabled in TESTING

	interval := 200 * time.Millisecond
	var count atomic.Int32
	step := make(chan bool)
	con.StartTicker("myticker", interval, func(ctx context.Context) error {
		count.Add(1)
		step <- true
		return nil
	})

	assert.Zero(count.Load())

	err := con.Startup()
	assert.NoError(err)
	defer con.Shutdown()

	<-step // at 1 intervals
	assert.Equal(int32(1), count.Load())
	time.Sleep(interval / 2) // at 1.5 intervals
	assert.Equal(int32(1), count.Load())
	<-step // at 2 intervals
	assert.Equal(int32(2), count.Load())
	<-step // at 3 intervals
	assert.Equal(int32(3), count.Load())
}

func TestConnector_TickerSkippingBeats(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	con := New("ticker.skipping.beats.connector")
	con.SetDeployment(LAB) // Tickers are disabled in TESTING

	interval := 200 * time.Millisecond
	var count atomic.Int32
	step := make(chan bool)
	con.StartTicker("myticker", interval, func(ctx context.Context) error {
		count.Add(1)
		step <- true
		time.Sleep(2*interval + interval/4) // 2.25 intervals
		return nil
	})

	assert.Zero(count.Load())

	err := con.Startup()
	assert.NoError(err)
	defer con.Shutdown()

	<-step // at 1 intervals
	assert.Equal(int32(1), count.Load())
	time.Sleep(interval + interval/2) // at 2.5 intervals
	assert.Equal(int32(1), count.Load())
	time.Sleep(interval) // at 3.5 intervals
	assert.Equal(int32(1), count.Load())

	<-step // at 4 intervals
	assert.Equal(int32(2), count.Load())
}

func TestConnector_TickerPendingOps(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	con := New("ticker.pending.ops.connector")
	con.SetDeployment(LAB) // Tickers are disabled in TESTING

	interval := 200 * time.Millisecond
	step1 := make(chan bool)
	hold1 := make(chan bool)
	step2 := make(chan bool)
	hold2 := make(chan bool)
	con.StartTicker("myticker1", interval, func(ctx context.Context) error {
		step1 <- true
		hold1 <- true
		return nil
	})
	con.StartTicker("myticker2", interval, func(ctx context.Context) error {
		step2 <- true
		hold2 <- true
		return nil
	})

	assert.Zero(con.pendingOps.Load())

	err := con.Startup()
	assert.NoError(err)
	defer con.Shutdown()

	<-step1 // at 1 intervals
	<-step2 // at 1 intervals
	assert.Equal(int32(2), con.pendingOps.Load())
	<-hold1
	time.Sleep(interval / 4) // at 1.25 intervals
	assert.Equal(int32(1), con.pendingOps.Load())
	<-hold2 // at 1.5 intervals
	time.Sleep(interval / 4)
	assert.Zero(con.pendingOps.Load())
}

func TestConnector_TickerTimeout(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	con := New("ticker.timeout.connector")
	con.SetDeployment(LAB) // Tickers are disabled in TESTING

	interval := 400 * time.Millisecond
	start := make(chan bool)
	end := make(chan bool)
	con.StartTicker("ticker", interval, func(ctx context.Context) error {
		start <- true
		ctx, cancel := context.WithTimeout(ctx, interval/4)
		defer cancel()
		<-ctx.Done()
		end <- true
		return nil
	})

	err := con.Startup()
	assert.NoError(err)
	defer con.Shutdown()

	<-start
	t0 := time.Now()
	<-end
	dur := time.Since(t0)
	assert.True(dur >= interval/4-interval/20, dur) // 5% margin of error
	assert.True(dur < interval/2, dur)
}

func TestConnector_TickerLifetimeCancellation(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	con := New("ticker.lifetime.cancellation.connector")
	con.SetDeployment(LAB) // Tickers are disabled in TESTING

	interval := 200 * time.Millisecond
	start := make(chan bool)
	end := make(chan bool)
	con.StartTicker("ticker", interval, func(ctx context.Context) error {
		start <- true
		ctx, cancel := context.WithTimeout(ctx, time.Minute)
		defer cancel()
		<-ctx.Done()
		end <- true
		return nil
	})

	err := con.Startup()
	assert.NoError(err)
	defer con.Shutdown()

	<-start
	t0 := time.Now()
	con.ctxCancel() // Cancel the lifetime context
	<-end
	dur := time.Since(t0)
	assert.True(dur < interval)
}

func TestConnector_TickersDisabledInTestingApp(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	con := New("tickers.disabled.in.testing.app.connector")

	interval := 200 * time.Millisecond
	var count atomic.Int32
	con.StartTicker("myticker", interval, func(ctx context.Context) error {
		count.Add(1)
		return nil
	})

	assert.Zero(count.Load())

	err := con.Startup()
	assert.NoError(err)
	defer con.Shutdown()

	time.Sleep(5 * interval)
	assert.Zero(count.Load())
}

func TestConnector_TickerStop(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	con := New("ticker.stop.connector")
	con.SetDeployment(LAB) // Tickers are disabled in TESTING

	interval := 200 * time.Millisecond
	var count atomic.Int32
	enter := make(chan bool)
	exit := make(chan bool)
	con.StartTicker("my-ticker_123", interval, func(ctx context.Context) error {
		count.Add(1)
		enter <- true
		exit <- true
		return nil
	})

	assert.Zero(count.Load())

	err := con.Startup()
	assert.NoError(err)
	defer con.Shutdown()

	<-enter
	assert.Equal(int32(1), count.Load())
	con.StopTicker("my-ticker_123")
	<-exit

	time.Sleep(2 * interval)
	assert.Equal(int32(1), count.Load())

	// Restart
	con.StartTicker("my-ticker_123", interval, func(ctx context.Context) error {
		count.Add(1)
		enter <- true
		exit <- true
		return nil
	})

	<-enter
	assert.Equal(int32(2), count.Load())
	<-exit
}

func TestConnector_Sleep(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	con := New("sleep.connector")
	err := con.Startup()
	assert.NoError(err)
	defer con.Shutdown()

	// Natural expiration
	ctx := context.Background()
	t0 := time.Now()
	v := con.Sleep(ctx, time.Millisecond*100)
	dur := time.Since(t0)
	assert.True(dur > time.Millisecond*100 && dur <= time.Millisecond*200)
	assert.True(v)

	// Context timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*100)
	t0 = time.Now()
	v = con.Sleep(ctx, time.Millisecond*1000)
	dur = time.Since(t0)
	assert.True(dur > time.Millisecond*100 && dur <= time.Millisecond*200)
	assert.False(v)
	cancel()

	// Context cancellation
	ctx, cancel = context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()
	t0 = time.Now()
	v = con.Sleep(ctx, time.Millisecond*1000)
	dur = time.Since(t0)
	assert.True(dur > time.Millisecond*100 && dur <= time.Millisecond*200)
	assert.False(v)

	// Lifetime cancellation
	ctx = context.Background()
	go func() {
		time.Sleep(100 * time.Millisecond)
		con.Shutdown()
	}()
	t0 = time.Now()
	v = con.Sleep(ctx, time.Millisecond*1000)
	dur = time.Since(t0)
	assert.True(dur > time.Millisecond*100 && dur <= time.Millisecond*200)
	assert.False(v)
}

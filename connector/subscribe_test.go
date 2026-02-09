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
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/fabric/pub"
	"github.com/microbus-io/fabric/sub"
	"github.com/microbus-io/fabric/transport"
	"github.com/microbus-io/fabric/utils"
	"github.com/microbus-io/testarossa"
)

func TestConnector_DirectorySubscription(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	ctx := t.Context()

	// Create the microservice
	var count int
	var appendix string
	con := New("directory.subscription.connector")
	con.Subscribe("GET", "directory/{appendix...}", func(w http.ResponseWriter, r *http.Request) error {
		count++
		_, appendix, _ = strings.Cut(r.URL.Path, "/directory/")
		return nil
	})

	// Startup the microservices
	err := con.Startup(ctx)
	assert.NoError(err)
	defer con.Shutdown(ctx)

	// Send messages to various locations under the directory
	_, err = con.GET(ctx, "https://directory.subscription.connector/directory/1.html")
	assert.NoError(err)
	assert.Equal("1.html", appendix)
	_, err = con.GET(ctx, "https://directory.subscription.connector/directory/2.html")
	assert.NoError(err)
	assert.Equal("2.html", appendix)
	_, err = con.GET(ctx, "https://directory.subscription.connector/directory/sub/3.html")
	assert.NoError(err)
	assert.Equal("sub/3.html", appendix)
	_, err = con.GET(ctx, "https://directory.subscription.connector/directory/")
	assert.NoError(err)
	assert.Equal("", appendix)

	assert.Equal(4, count)

	// The path of the directory should not be captured
	_, err = con.GET(ctx, "https://directory.subscription.connector/directory")
	assert.Error(err)
}

func TestConnector_HyphenInHostname(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	ctx := t.Context()

	// Create the microservice
	entered := false
	con := New("hyphen-in-host_name.connector")
	con.Subscribe("GET", "path", func(w http.ResponseWriter, r *http.Request) error {
		entered = true
		return nil
	})

	// Startup the microservices
	err := con.Startup(ctx)
	assert.NoError(err)
	defer con.Shutdown(ctx)

	_, err = con.GET(ctx, "https://hyphen-in-host_name.connector/path")
	assert.NoError(err)
	assert.True(entered)
}

func TestConnector_PathArgumentsInSubscription(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	ctx := t.Context()

	// Create the microservice
	alphaCount := 0
	betaCount := 0
	rootCount := 0
	parentCount := 0
	detected := map[string]string{}
	con := New("path.arguments.in.subscription.connector")
	con.Subscribe("GET", "/obj/{id}/alpha", func(w http.ResponseWriter, r *http.Request) error {
		alphaCount++
		parts := strings.Split(r.URL.Path, "/")
		detected[r.URL.Path] = parts[2]
		return nil
	})
	con.Subscribe("GET", "/obj/{id}/beta", func(w http.ResponseWriter, r *http.Request) error {
		betaCount++
		parts := strings.Split(r.URL.Path, "/")
		detected[r.URL.Path] = parts[2]
		return nil
	})
	con.Subscribe("GET", "/obj/{id}", func(w http.ResponseWriter, r *http.Request) error {
		rootCount++
		parts := strings.Split(r.URL.Path, "/")
		detected[r.URL.Path] = parts[2]
		return nil
	})
	con.Subscribe("GET", "/obj", func(w http.ResponseWriter, r *http.Request) error {
		parentCount++
		detected[r.URL.Path] = ""
		return nil
	})

	// Startup the microservices
	err := con.Startup(ctx)
	assert.NoError(err)
	defer con.Shutdown(ctx)

	// Send messages
	_, err = con.GET(ctx, "https://path.arguments.in.subscription.connector/obj/1234/alpha")
	assert.NoError(err)
	assert.Equal(1, alphaCount)
	_, err = con.GET(ctx, "https://path.arguments.in.subscription.connector/obj/2345/alpha")
	assert.NoError(err)
	assert.Equal(2, alphaCount)
	_, err = con.GET(ctx, "https://path.arguments.in.subscription.connector/obj/1111/beta")
	assert.NoError(err)
	assert.Equal(1, betaCount)
	_, err = con.GET(ctx, "https://path.arguments.in.subscription.connector/obj/2222/beta")
	assert.NoError(err)
	assert.Equal(2, betaCount)
	_, err = con.GET(ctx, "https://path.arguments.in.subscription.connector/obj/8000")
	assert.NoError(err)
	assert.Equal(1, rootCount)
	_, err = con.GET(ctx, "https://path.arguments.in.subscription.connector/obj")
	assert.NoError(err)
	assert.Equal(1, parentCount)

	assert.Len(detected, 6)
	assert.Equal("1234", detected["/obj/1234/alpha"])
	assert.Equal("2345", detected["/obj/2345/alpha"])
	assert.Equal("1111", detected["/obj/1111/beta"])
	assert.Equal("2222", detected["/obj/2222/beta"])
	assert.Equal("8000", detected["/obj/8000"])
	assert.Equal("", detected["/obj"])
}

func TestConnector_MixedAsteriskSubscription(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	ctx := t.Context()

	// Create the microservice
	detected := map[string]bool{}
	con := New("mixed.asterisk.subscription.connector")
	con.Subscribe("GET", "/obj/x*x/gamma", func(w http.ResponseWriter, r *http.Request) error {
		detected[r.URL.Path] = true
		return nil
	})

	// Startup the microservices
	err := con.Startup(ctx)
	assert.NoError(err)
	defer con.Shutdown(ctx)

	_, err = con.GET(ctx, "https://mixed.asterisk.subscription.connector/obj/2222/gamma")
	assert.Error(err)
	_, err = con.GET(ctx, "https://mixed.asterisk.subscription.connector/obj/x2x/gamma")
	assert.Error(err)
	_, err = con.GET(ctx, "https://mixed.asterisk.subscription.connector/obj/x*x/gamma")
	assert.NoError(err)
}

func TestConnector_ErrorAndPanic(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	ctx := t.Context()

	// Create the microservice
	con := New("error.and.panic.connector")
	con.Subscribe("GET", "usererr", func(w http.ResponseWriter, r *http.Request) error {
		return errors.New("bad input", http.StatusBadRequest)
	})
	con.Subscribe("GET", "err", func(w http.ResponseWriter, r *http.Request) error {
		return errors.New("it's bad")
	})
	con.Subscribe("GET", "panic", func(w http.ResponseWriter, r *http.Request) error {
		panic("it's really bad")
	})
	con.Subscribe("GET", "oserr", func(w http.ResponseWriter, r *http.Request) error {
		err := errors.Trace(os.ErrNotExist)
		assert.True(errors.Is(err, os.ErrNotExist))
		return err
	})
	con.Subscribe("GET", "stillalive", func(w http.ResponseWriter, r *http.Request) error {
		return nil
	})

	// Startup the microservices
	err := con.Startup(ctx)
	assert.NoError(err)
	defer con.Shutdown(ctx)

	// Send messages
	_, err = con.GET(ctx, "https://error.and.panic.connector/usererr")
	assert.Error(err)
	assert.Equal("bad input", err.Error())
	assert.Equal(http.StatusBadRequest, errors.Convert(err).StatusCode)

	_, err = con.GET(ctx, "https://error.and.panic.connector/err")
	assert.Error(err)
	assert.Equal("it's bad", err.Error())
	assert.Equal(http.StatusInternalServerError, errors.Convert(err).StatusCode)

	_, err = con.GET(ctx, "https://error.and.panic.connector/panic")
	assert.Error(err)
	assert.Equal("it's really bad", err.Error())

	_, err = con.GET(ctx, "https://error.and.panic.connector/oserr")
	assert.Error(err)
	assert.Equal("file does not exist", err.Error())
	assert.False(errors.Is(err, os.ErrNotExist)) // Cannot reconstitute error type

	_, err = con.GET(ctx, "https://error.and.panic.connector/stillalive")
	assert.NoError(err)
}

func TestConnector_DifferentPlanes(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	ctx := t.Context()

	// Create the microservices
	alpha := New("different.planes.connector")
	alpha.SetPlane("alpha")
	alpha.Subscribe("GET", "id", func(w http.ResponseWriter, r *http.Request) error {
		w.Write([]byte("alpha"))
		return nil
	})

	beta := New("different.planes.connector")
	beta.SetPlane("beta")
	beta.Subscribe("GET", "id", func(w http.ResponseWriter, r *http.Request) error {
		w.Write([]byte("beta"))
		return nil
	})

	// Startup the microservices
	err := alpha.Startup(ctx)
	assert.NoError(err)
	defer alpha.Shutdown(ctx)
	err = beta.Startup(ctx)
	assert.NoError(err)
	defer beta.Shutdown(ctx)

	// Alpha should never see beta
	for range 32 {
		response, err := alpha.GET(ctx, "https://different.planes.connector/id")
		if assert.NoError(err) {
			body, err := io.ReadAll(response.Body)
			if assert.NoError(err) {
				assert.Equal("alpha", string(body))
			}
		}
	}

	// Beta should never see alpha
	for range 32 {
		response, err := beta.GET(ctx, "https://different.planes.connector/id")
		if assert.NoError(err) {
			body, err := io.ReadAll(response.Body)
			if assert.NoError(err) {
				assert.Equal("beta", string(body))
			}
		}
	}
}

func TestConnector_SubscribeBeforeAndAfterStartup(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	ctx := t.Context()

	// Create the microservices
	var beforeCalled, afterCalled bool
	con := New("subscribe.before.and.after.startup.connector")

	// Subscribe before beta is started
	con.Subscribe("GET", "before", func(w http.ResponseWriter, r *http.Request) error {
		beforeCalled = true
		return nil
	})

	// Startup the microservice
	err := con.Startup(ctx)
	assert.NoError(err)
	defer con.Shutdown(ctx)

	// Subscribe after beta is started
	con.Subscribe("GET", "after", func(w http.ResponseWriter, r *http.Request) error {
		afterCalled = true
		return nil
	})

	// Send requests to both handlers
	_, err = con.GET(ctx, "https://subscribe.before.and.after.startup.connector/before")
	assert.NoError(err)
	_, err = con.GET(ctx, "https://subscribe.before.and.after.startup.connector/after")
	assert.NoError(err)

	assert.True(beforeCalled)
	assert.True(afterCalled)
}

func TestConnector_Unsubscribe(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	ctx := t.Context()

	// Create the microservice
	con := New("unsubscribe.connector")

	// Subscribe
	unsub1, _ := con.Subscribe("GET", "sub1", func(w http.ResponseWriter, r *http.Request) error {
		return nil
	})
	con.Subscribe("GET", "sub2", func(w http.ResponseWriter, r *http.Request) error {
		return nil
	})

	// Startup the microservices
	err := con.Startup(ctx)
	assert.NoError(err)
	defer con.Shutdown(ctx)

	// Send requests
	_, err = con.GET(ctx, "https://unsubscribe.connector/sub1")
	assert.NoError(err)
	_, err = con.GET(ctx, "https://unsubscribe.connector/sub2")
	assert.NoError(err)

	// Unsubscribe sub1
	err = unsub1()
	assert.NoError(err)

	// Send requests
	_, err = con.GET(ctx, "https://unsubscribe.connector/sub1")
	assert.Error(err)
	_, err = con.GET(ctx, "https://unsubscribe.connector/sub2")
	assert.NoError(err)

	// Deactivate all
	err = con.deactivateSubs()
	assert.NoError(err)

	// Send requests
	_, err = con.GET(ctx, "https://unsubscribe.connector/sub1")
	assert.Error(err)
	_, err = con.GET(ctx, "https://unsubscribe.connector/sub2")
	assert.Error(err)
}

func TestConnector_AnotherHost(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	ctx := t.Context()

	// Create the microservices
	alpha := New("alpha.another.host.connector")
	alpha.Subscribe("GET", "https://alternative.host.connector/empty", func(w http.ResponseWriter, r *http.Request) error {
		return nil
	})

	beta1 := New("beta.another.host.connector")
	beta1.Subscribe("GET", "https://alternative.host.connector/empty", func(w http.ResponseWriter, r *http.Request) error {
		return nil
	})

	beta2 := New("beta.another.host.connector")
	beta2.Subscribe("GET", "https://alternative.host.connector/empty", func(w http.ResponseWriter, r *http.Request) error {
		return nil
	})

	// Startup the microservices
	err := alpha.Startup(ctx)
	assert.NoError(err)
	defer alpha.Shutdown(ctx)
	err = beta1.Startup(ctx)
	assert.NoError(err)
	defer beta1.Shutdown(ctx)
	err = beta2.Startup(ctx)
	assert.NoError(err)
	defer beta2.Shutdown(ctx)

	// Send message
	responded := 0
	ch := alpha.Publish(ctx, pub.GET("https://alternative.host.connector/empty"), pub.Multicast())
	for i := range ch {
		_, err := i.Get()
		assert.NoError(err)
		responded++
	}
	// Even though the microservices subscribe to the same alternative host, their queues should be different
	assert.Equal(2, responded)
}

func TestConnector_DirectAddressing(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	ctx := t.Context()

	// Create the microservice
	con := New("direct.addressing.connector")
	unsub, _ := con.Subscribe("GET", "/hello", func(w http.ResponseWriter, r *http.Request) error {
		w.Write([]byte("Hello"))
		return nil
	})

	// Startup the microservice
	err := con.Startup(ctx)
	assert.NoError(err)
	defer con.Shutdown(ctx)

	// Send messages
	_, err = con.GET(ctx, "https://direct.addressing.connector/hello")
	assert.NoError(err)
	_, err = con.GET(ctx, "https://"+con.id+".direct.addressing.connector/hello")
	assert.NoError(err)

	err = unsub()
	assert.NoError(err)

	// Both subscriptions should be deactivated
	_, err = con.GET(ctx, "https://direct.addressing.connector/hello")
	assert.Error(err)
	_, err = con.GET(ctx, "https://"+con.id+".direct.addressing.connector/hello")
	assert.Error(err)
}

func TestConnector_SubPendingOps(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	assert := testarossa.For(t)

	con := New("sub.pending.ops.connector")

	start := make(chan bool)
	hold := make(chan bool)
	end := make(chan bool)
	con.Subscribe("GET", "/op", func(w http.ResponseWriter, r *http.Request) error {
		start <- true
		hold <- true
		return nil
	})

	err := con.Startup(ctx)
	assert.NoError(err)
	defer con.Shutdown(ctx)

	assert.Zero(con.pendingOps.Load())

	// First call
	go func() {
		con.GET(con.Lifetime(), "https://sub.pending.ops.connector/op")
		end <- true
	}()
	<-start
	assert.Equal(int32(1), con.pendingOps.Load())

	// Second call
	go func() {
		con.GET(con.Lifetime(), "https://sub.pending.ops.connector/op")
		end <- true
	}()
	<-start
	assert.Equal(int32(2), con.pendingOps.Load())

	<-hold
	<-end
	assert.Equal(int32(1), con.pendingOps.Load())
	<-hold
	<-end
	assert.Zero(con.pendingOps.Load())
}

func TestConnector_SubscriptionMethods(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	ctx := t.Context()

	// Create the microservice
	var get int
	var post int
	var star int
	con := New("subscription.methods.connector")
	con.Subscribe("GET", "single", func(w http.ResponseWriter, r *http.Request) error {
		get++
		return nil
	})
	con.Subscribe("POST", "single", func(w http.ResponseWriter, r *http.Request) error {
		post++
		return nil
	})
	con.Subscribe("ANY", "star", func(w http.ResponseWriter, r *http.Request) error {
		star++
		return nil
	})

	// Startup the microservices
	err := con.Startup(ctx)
	assert.NoError(err)
	defer con.Shutdown(ctx)

	// Send messages to various locations under the directory
	_, err = con.Request(ctx, pub.GET("https://subscription.methods.connector/single"))
	assert.NoError(err)
	assert.Equal(1, get)
	assert.Zero(post)

	_, err = con.Request(ctx, pub.POST("https://subscription.methods.connector/single"))
	assert.NoError(err)
	assert.Equal(1, get)
	assert.Equal(1, post)

	_, err = con.Request(ctx, pub.PATCH("https://subscription.methods.connector/single"))
	assert.Error(err)
	assert.Equal(http.StatusNotFound, errors.Convert(err).StatusCode)
	assert.Equal(1, get)
	assert.Equal(1, post)

	_, err = con.Request(ctx, pub.PATCH("https://subscription.methods.connector/star"))
	assert.NoError(err)
	assert.Equal(1, get)
	assert.Equal(1, post)
	assert.Equal(1, star)

	_, err = con.Request(ctx, pub.GET("https://subscription.methods.connector/star"))
	assert.NoError(err)
	assert.Equal(1, get)
	assert.Equal(1, post)
	assert.Equal(2, star)
}

func TestConnector_SubscriptionPorts(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	ctx := t.Context()

	// Create the microservice
	var p123 int
	var p234 int
	var star int
	con := New("subscription.ports.connector")
	con.Subscribe("GET", ":123/single", func(w http.ResponseWriter, r *http.Request) error {
		p123++
		return nil
	})
	con.Subscribe("GET", ":234/single", func(w http.ResponseWriter, r *http.Request) error {
		p234++
		return nil
	})
	con.Subscribe("GET", ":0/any", func(w http.ResponseWriter, r *http.Request) error {
		star++
		return nil
	})

	// Startup the microservices
	err := con.Startup(ctx)
	assert.NoError(err)
	defer con.Shutdown(ctx)

	// Send messages to various locations under the directory
	_, err = con.Request(ctx, pub.GET("https://subscription.ports.connector:123/single"))
	assert.NoError(err)
	assert.Equal(1, p123)
	assert.Zero(p234)

	_, err = con.Request(ctx, pub.GET("https://subscription.ports.connector:234/single"))
	assert.NoError(err)
	assert.Equal(1, p123)
	assert.Equal(1, p234)

	_, err = con.Request(ctx, pub.GET("https://subscription.ports.connector:999/single"))
	assert.Error(err)
	assert.Equal(http.StatusNotFound, errors.Convert(err).StatusCode)
	assert.Equal(1, p123)
	assert.Equal(1, p234)

	_, err = con.Request(ctx, pub.GET("https://subscription.ports.connector:999/any"))
	assert.NoError(err)
	assert.Equal(1, p123)
	assert.Equal(1, p234)
	assert.Equal(1, star)

	_, err = con.Request(ctx, pub.GET("https://subscription.ports.connector:10000/any"))
	assert.NoError(err)
	assert.Equal(1, p123)
	assert.Equal(1, p234)
	assert.Equal(2, star)
}

func TestConnector_FrameConsistency(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	ctx := t.Context()

	// Create the microservice
	con := New("frame.consistency.connector")
	con.Subscribe("GET", "/frame", func(w http.ResponseWriter, r *http.Request) error {
		f1 := frame.Of(r)
		f2 := frame.Of(r.Context())
		assert.Equal(&f1, &f2)
		f1.Set("ABC", "abc")
		assert.Equal(&f1, &f2)
		assert.Equal("abc", f2.Get("ABC"))
		f2.Set("ABC", "")
		assert.Equal(&f1, &f2)
		assert.Equal("", f1.Get("ABC"))
		return nil
	})

	// Startup the microservices
	err := con.Startup(ctx)
	assert.NoError(err)
	defer con.Shutdown(ctx)

	// Send messages to various locations under the directory
	_, err = con.Request(ctx, pub.GET("https://frame.consistency.connector/frame"))
	assert.NoError(err)
}

func BenchmarkConnection_AckRequest(b *testing.B) {
	ctx := context.Background()
	// Startup the microservices
	con := New("ack.request.connector")
	err := con.Startup(ctx)
	testarossa.NoError(b, err)
	defer con.Shutdown(ctx)

	req, _ := http.NewRequest("POST", "https://nowhere/", strings.NewReader(utils.RandomIdentifier(16*1024)))
	f := frame.Of(req)
	f.SetFromHost("someone")
	f.SetFromID("me")
	f.SetMessageID("123456")

	var buf bytes.Buffer
	req.Write(&buf)
	msgData := buf.Bytes()

	b.ResetTimer()
	for b.Loop() {
		con.ackRequest(&transport.Msg{
			Data: msgData,
		}, &sub.Subscription{})
	}

	// goos: darwin
	// goarch: arm64
	// pkg: github.com/microbus-io/fabric/connector
	// cpu: Apple M1 Pro
	// BenchmarkConnection_AckRequest-10    	  202579	      5860 ns/op	    2508 B/op	      46 allocs/op
}

func TestConnector_PathArguments(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	ctx := t.Context()

	// Create the microservice
	var foo string
	var bar string
	con := New("path.arguments.connector")
	con.Subscribe("ANY", "/foo/{foo}/bar/{bar}", func(w http.ResponseWriter, r *http.Request) error {
		parts := strings.Split(r.URL.Path, "/")
		foo = parts[2]
		bar = parts[4]
		return nil
	})

	// Startup the microservices
	err := con.Startup(ctx)
	assert.NoError(err)
	defer con.Shutdown(ctx)

	// Values provided in path should be delivered
	_, err = con.Request(ctx, pub.GET("https://path.arguments.connector/foo/FOO1/bar/BAR1"))
	if assert.NoError(err) {
		assert.Equal("FOO1", foo)
		assert.Equal("BAR1", bar)
	}
	_, err = con.Request(ctx, pub.GET("https://path.arguments.connector/foo/{foo}/bar/{bar}"))
	if assert.NoError(err) {
		assert.Equal("", foo)
		assert.Equal("", bar)
	}
	_, err = con.Request(ctx, pub.GET("https://path.arguments.connector/foo//bar/BAR2"))
	if assert.NoError(err) {
		assert.Equal("", foo)
		assert.Equal("BAR2", bar)
	}
}

func TestConnector_InvalidPathArguments(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	assert := testarossa.For(t)

	for _, path := range []string{
		"/1/x{mmm}x", "/2/{}x", "/3/x{}", "/4/x{...}", "/}{", "/{/x",
	} {
		con := New("invalid.path.arguments.connector")
		con.Subscribe("GET", path, func(w http.ResponseWriter, r *http.Request) error {
			return nil
		})
		err := con.Startup(ctx)
		if !assert.Error(err, "%", path) {
			con.Shutdown(ctx)
		}
	}
}

func TestConnector_SubscriptionLocality(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	ctx := t.Context()

	// Create the microservices
	alpha := New("alpha.subscription.locality.connector")
	alpha.SetLocality("az1.dC2.weSt.Us")

	beta1 := New("beta.subscription.locality.connector")
	beta1.SetLocality("az2.dc2.WEST.us")

	beta2 := New("beta.subscription.locality.connector")
	beta2.SetLocality("az1.DC3.west.us")

	beta3 := New("beta.subscription.locality.connector")
	beta3.SetLocality("az1.dc2.east.US")

	beta4 := New("beta.subscription.locality.connector")
	beta4.SetLocality("az4.dc5.north.eu")

	beta5 := New("beta.subscription.locality.connector")
	beta5.SetLocality("az1.dc1.southwest.ap")

	beta6 := New("beta.subscription.locality.connector")
	beta6.SetLocality("az4.dc2.south.ap")

	// Startup the microservices
	for _, con := range []*Connector{alpha, beta1, beta2, beta3, beta4, beta5, beta6} {
		con.Subscribe("GET", "ok", func(w http.ResponseWriter, r *http.Request) error {
			return nil
		})
		err := con.Startup(ctx)
		assert.NoError(err)
		defer con.Shutdown(ctx)
	}

	// Requests should converge to beta1 that is in the same DC as alpha
	for repeat := 0; repeat < 16; repeat++ {
		beta1Found := false
		for sticky := 0; sticky < 16; {
			localityBefore, _ := alpha.localResponder.Load("https://beta.subscription.locality.connector/ok")
			res, err := alpha.GET(ctx, "https://beta.subscription.locality.connector/ok")
			if !assert.NoError(err) {
				break
			}
			localityAfter, _ := alpha.localResponder.Load("https://beta.subscription.locality.connector/ok")
			assert.True(len(localityAfter) >= len(localityBefore))

			if beta1Found {
				// Once beta1 was found, all future requests should go there
				assert.Equal(beta1.id, frame.Of(res).FromID())
				sticky++
			}
			beta1Found = frame.Of(res).FromID() == beta1.id
		}
		alpha.localResponder.Clear() // Reset
	}

	// Shutting down beta1, requests should converge to beta2 that is in the same region as alpha
	beta1.Shutdown(ctx)

	for repeat := 0; repeat < 16; repeat++ {
		beta2Found := false
		for sticky := 0; sticky < 16; {
			res, err := alpha.GET(ctx, "https://beta.subscription.locality.connector/ok")
			if !assert.NoError(err) {
				break
			}
			assert.NotEqual(beta1.id, frame.Of(res).FromID()) // beta1 was shut down
			if beta2Found {
				// Once beta2 was found, all future requests should go there
				assert.Equal(beta2.id, frame.Of(res).FromID())
				sticky++
			}
			beta2Found = frame.Of(res).FromID() == beta2.id
		}
		alpha.localResponder.Clear() // Reset
	}
}

func TestConnector_SubscriptionNoLocalityWithID(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	ctx := t.Context()

	// Create the microservices
	alpha := New("alpha.subscription.no.locality.with.id.connector")
	alpha.SetLocality("az1.dc2.west.us")

	beta := New("beta.subscription.no.locality.with.id.connector")
	beta.SetLocality("az1.dc2.west.us")
	beta.Subscribe("GET", "byid", func(w http.ResponseWriter, r *http.Request) error {
		// When targeting a microservice by its ID, locality-aware optimization should not kick in
		assert.Equal(beta.id+".beta.subscription.no.locality.with.id.connector:443", r.Host)
		return nil
	})
	first := true
	beta.Subscribe("GET", "byhost", func(w http.ResponseWriter, r *http.Request) error {
		// When targeting by host, locality-aware optimization should kick in after the first request
		if first {
			assert.Equal("beta.subscription.no.locality.with.id.connector:443", r.Host)
			first = false
		} else {
			assert.Equal("az1.dc2.west.us.beta.subscription.no.locality.with.id.connector:443", r.Host)
		}
		return nil
	})

	err := alpha.Startup(ctx)
	assert.NoError(err)
	defer alpha.Shutdown(ctx)
	err = beta.Startup(ctx)
	assert.NoError(err)
	defer beta.Shutdown(ctx)

	for repeat := 0; repeat < 16; repeat++ {
		_, err := alpha.GET(ctx, "https://"+beta.ID()+".beta.subscription.no.locality.with.id.connector/byid")
		if !assert.NoError(err) {
			break
		}
	}
	for repeat := 0; repeat < 16; repeat++ {
		_, err := alpha.GET(ctx, "https://beta.subscription.no.locality.with.id.connector/byhost")
		if !assert.NoError(err) {
			break
		}
	}
}

func TestConnector_Actor(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	// Create the microservice
	entered := 0
	con := New("actor.connector")
	con.Subscribe("GET", "student",
		func(w http.ResponseWriter, r *http.Request) error {
			entered++
			return nil
		},
		sub.RequiredClaims(`iss=="hogwats.issuer" && (roles=~"student" || roles=~"professor")`),
	)
	con.Subscribe("GET", "professor",
		func(w http.ResponseWriter, r *http.Request) error {
			entered++
			return nil
		},
		sub.RequiredClaims(`iss=="hogwats.issuer" && roles=~"professor"`),
	)

	// Startup the microservice
	ctx := t.Context()
	err := con.Startup(ctx)
	assert.NoError(err)
	defer con.Shutdown(ctx)

	// Without a token
	_, err = con.GET(ctx, "https://actor.connector/student")
	assert.Error(err)
	assert.Equal(http.StatusUnauthorized, errors.StatusCode(err))
	assert.Equal(0, entered)
	_, err = con.GET(ctx, "https://actor.connector/professor")
	assert.Error(err)
	assert.Equal(http.StatusUnauthorized, errors.StatusCode(err))
	assert.Equal(0, entered)

	// Create token for wizard role
	harry := map[string]any{
		"iss":   "hogwats.issuer",
		"sub":   "harry@hogwarts.edu",
		"roles": "wizard student",
	}
	_, err = con.Request(ctx, pub.GET("https://actor.connector/student"), pub.Actor(harry))
	assert.NoError(err)
	assert.Equal(1, entered)
	_, err = con.Request(ctx, pub.GET("https://actor.connector/professor"), pub.Actor(harry))
	assert.Error(err)
	assert.Equal(http.StatusForbidden, errors.StatusCode(err))
	assert.Equal(1, entered)

	// Create token for professor role
	dumbledore := map[string]any{
		"iss":   "hogwats.issuer",
		"sub":   "dumbledore@hogwarts.edu",
		"roles": "wizard professor headmaster",
	}
	_, err = con.Request(ctx, pub.GET("https://actor.connector/student"), pub.Actor(dumbledore))
	assert.NoError(err)
	assert.Equal(2, entered)
	_, err = con.Request(ctx, pub.GET("https://actor.connector/professor"), pub.Actor(dumbledore))
	assert.NoError(err)
	assert.Equal(3, entered)
}

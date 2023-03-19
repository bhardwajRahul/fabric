/*
Copyright 2023 Microbus LLC and various contributors

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
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/microbus-io/fabric/errors"
	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/fabric/log"
	"github.com/microbus-io/fabric/sub"
	"github.com/microbus-io/fabric/utils"
	"github.com/nats-io/nats.go"
)

// HTTPHandler extends the standard http.Handler to also return an error
type HTTPHandler = sub.HTTPHandler

/*
Subscribe assigns a function to handle HTTP requests to the given path.
If the path ends with a / all sub-paths under the path are capture by the subscription

If the path does not include a host name, the default host is used.
If a port is not specified, 443 is used by default.

Examples of valid paths:

	(empty)
	/
	:1080
	:1080/
	:1080/path
	/path/with/slash
	path/with/no/slash
	https://www.example.com/path
	https://www.example.com:1080/path
*/
func (c *Connector) Subscribe(path string, handler sub.HTTPHandler, options ...sub.Option) error {
	if c.hostName == "" {
		return c.captureInitErr(errors.New("host name is not set"))
	}
	newSub, err := sub.NewSub(c.hostName, path, handler, options...)
	if err != nil {
		return c.captureInitErr(errors.Trace(err))
	}
	if c.IsStarted() {
		err := c.activateSub(newSub)
		if err != nil {
			return c.captureInitErr(errors.Trace(err))
		}
		time.Sleep(20 * time.Millisecond) // Give time for subscription activation by NATS
	}
	key := newSub.Canonical()
	c.subsLock.Lock()
	c.subs[key] = newSub
	c.subsLock.Unlock()
	return nil
}

// Unsubscribe removes the handler for the specified path
func (c *Connector) Unsubscribe(path string) error {
	newSub, err := sub.NewSub(c.hostName, path, nil)
	if err != nil {
		return errors.Trace(err)
	}
	key := newSub.Canonical()
	c.subsLock.Lock()
	if sub, ok := c.subs[key]; ok {
		err = c.deactivateSub(sub)
		if err == nil {
			delete(c.subs, key)
		}
	}
	c.subsLock.Unlock()
	if c.IsStarted() {
		time.Sleep(20 * time.Millisecond) // Give time for subscription deactivation by NATS
	}
	return errors.Trace(err)
}

// activateSub will subscribe to NATS
func (c *Connector) activateSub(s *sub.Subscription) error {
	handler := func(msg *nats.Msg) {
		err := c.ackRequest(msg, s)
		if err != nil {
			err = errors.Trace(err)
			c.LogError(c.lifetimeCtx, "Acking request", log.Error(err))
			return
		}
		go func() {
			err := c.onRequest(msg, s)
			if err != nil {
				err = errors.Trace(err)
				c.LogError(c.lifetimeCtx, "Processing request", log.Error(err))
			}
		}()
	}

	var err error
	if s.HostSub == nil {
		if s.Queue != "" {
			s.HostSub, err = c.natsConn.QueueSubscribe(subjectOfSubscription(c.plane, s.Host, s.Port, s.Path), s.Queue, handler)
		} else {
			s.HostSub, err = c.natsConn.Subscribe(subjectOfSubscription(c.plane, s.Host, s.Port, s.Path), handler)
		}
		if err != nil {
			return errors.Trace(err)
		}
	}
	if s.DirectSub == nil {
		if s.Queue != "" {
			s.DirectSub, err = c.natsConn.QueueSubscribe(subjectOfSubscription(c.plane, c.id+"."+s.Host, s.Port, s.Path), s.Queue, handler)
		} else {
			s.DirectSub, err = c.natsConn.Subscribe(subjectOfSubscription(c.plane, c.id+"."+s.Host, s.Port, s.Path), handler)
		}
		if err != nil {
			return errors.Trace(err)
		}
	}
	// c.LogDebug(c.lifetimeCtx, "Sub activated", log.String("name", s.Canonical()))
	return nil
}

// deactivateSubs unsubscribes from NATS.
func (c *Connector) deactivateSubs() error {
	c.subsLock.Lock()
	var lastErr error
	for _, sub := range c.subs {
		lastErr = c.deactivateSub(sub)
	}
	c.subsLock.Unlock()
	if c.IsStarted() {
		time.Sleep(20 * time.Millisecond) // Give time for subscription deactivation by NATS
	}
	return errors.Trace(lastErr)
}

// deactivateSub unsubscribes from NATS.
func (c *Connector) deactivateSub(s *sub.Subscription) error {
	var lastErr error
	if s.HostSub != nil {
		err := s.HostSub.Unsubscribe()
		if err != nil {
			lastErr = errors.Trace(err, s.Canonical())
			c.LogError(c.lifetimeCtx, "Unsubscribing host sub", log.Error(lastErr), log.String("sub", s.Canonical()))
		} else {
			s.HostSub = nil
		}
	}
	if s.DirectSub != nil {
		err := s.DirectSub.Unsubscribe()
		if err != nil {
			lastErr = errors.Trace(err, s.Canonical())
			c.LogError(c.lifetimeCtx, "Unsubscribing direct sub", log.Error(lastErr), log.String("sub", s.Canonical()))
		} else {
			s.DirectSub = nil
		}
	}
	// c.LogDebug(c.Lifetime(), "Sub deactivated", log.String("name", s.Canonical()))
	return lastErr
}

// ackRequest sends an ack response back to the caller.
// Acks are sent as soon as a request is received to let the caller know it is
// being processed
func (c *Connector) ackRequest(msg *nats.Msg, s *sub.Subscription) error {
	// Parse only the headers of the request
	headerData := msg.Data
	eoh := bytes.Index(headerData, []byte("\r\n\r\n"))
	if eoh >= 0 {
		headerData = headerData[:eoh+4]
	}
	httpReq, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(headerData)))
	if err != nil {
		return errors.Trace(err)
	}

	// Ack only the first fragment which will have index 1
	fragmentIndex, fragmentMax := frame.Of(httpReq).Fragment()
	if fragmentIndex > 1 {
		return nil
	}

	// Get return address
	fromHost := frame.Of(httpReq).FromHost()
	if fromHost == "" {
		return errors.New("empty " + frame.HeaderFromHost + " header")
	}
	fromID := frame.Of(httpReq).FromID()
	if fromID == "" {
		return errors.New("empty " + frame.HeaderFromId + " header")
	}
	msgID := frame.Of(httpReq).MessageID()
	if msgID == "" {
		return errors.New("empty " + frame.HeaderMsgId + " header")
	}
	queue := s.Queue
	if queue == "" {
		queue = c.id + "." + c.hostName
	}

	// Prepare and send the ack
	var buf bytes.Buffer
	buf.WriteString("HTTP/1.1 ")
	if fragmentMax > 1 {
		buf.WriteString("100 Continue")
	} else {
		buf.WriteString("202 Accepted")
	}
	buf.WriteString("\r\nConnection: close")
	header := map[string]string{
		frame.HeaderOpCode:   frame.OpCodeAck,
		frame.HeaderFromHost: c.hostName,
		frame.HeaderFromId:   c.id,
		frame.HeaderMsgId:    msgID,
		frame.HeaderQueue:    queue,
	}
	for k, v := range header {
		buf.WriteString("\r\n")
		buf.WriteString(k)
		buf.WriteString(": ")
		buf.WriteString(v)
	}
	buf.WriteString("\r\n\r\n")

	err = c.natsConn.Publish(subjectOfResponses(c.plane, fromHost, fromID), buf.Bytes())
	if err != nil {
		return errors.Trace(err)
	}

	return nil
}

// onRequest is called when an incoming HTTP request is received.
// The message is dispatched to the appropriate web handler and the response is serialized and sent back to the response channel of the sender
func (c *Connector) onRequest(msg *nats.Msg, s *sub.Subscription) error {
	atomic.AddInt32(&c.pendingOps, 1)
	defer atomic.AddInt32(&c.pendingOps, -1)

	// Parse the request
	httpReq, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(msg.Data)))
	if err != nil {
		return errors.Trace(err)
	}

	// Fill in the gaps
	httpReq.URL.Host = fmt.Sprintf("%s:%d", s.Host, s.Port)
	httpReq.URL.Scheme = "https"

	// Get the sender host name and message ID
	fromHost := frame.Of(httpReq).FromHost()
	fromId := frame.Of(httpReq).FromID()
	msgID := frame.Of(httpReq).MessageID()
	queue := s.Queue
	if queue == "" {
		queue = c.id + "." + c.hostName
	}

	c.LogDebug(c.lifetimeCtx, "Handling", log.String("msg", msgID), log.String("sub", s.Canonical()))

	// Time budget
	budget := frame.Of(httpReq).TimeBudget()
	if budget > 0 && budget <= c.networkHop {
		return errors.Newc(http.StatusRequestTimeout, "timeout")
	}

	// Integrate fragments together
	httpReq, err = c.defragRequest(httpReq)
	if err != nil {
		return errors.Trace(err)
	}
	if httpReq == nil {
		// Not all fragments arrived yet
		return nil
	}

	// Execute the request
	handlerStartTime := time.Now()
	httpRecorder := httpx.NewResponseRecorder()
	var handlerErr error
	corsPreflight := (httpReq.Method == "OPTIONS" && httpReq.Header.Get("Origin") != "")
	if corsPreflight {
		// CORS preflight requests are returned empty
		// https://developer.mozilla.org/en-US/docs/Web/HTTP/CORS
		httpRecorder.WriteHeader(http.StatusNoContent)
	} else {
		// Prepare the context
		frameCtx := context.WithValue(c.lifetimeCtx, frame.ContextKey, httpReq.Header)
		ctx := frameCtx
		cancel := func() {}
		if budget > 0 {
			// Set the context's timeout to the time budget reduced by a network hop
			ctx, cancel = context.WithTimeout(frameCtx, budget-c.networkHop)
		}
		httpReq = httpReq.WithContext(ctx)

		// Call the handler
		handlerErr = utils.CatchPanic(func() error {
			return s.Handler.(HTTPHandler)(httpRecorder, httpReq)
		})
		cancel()

		if handlerErr != nil {
			handlerErr = errors.Trace(handlerErr, httpx.JoinHostAndPath(c.hostName, s.Path))
			if handlerErr.Error() == "http: request body too large" || // https://go.dev/src/net/http/request.go#L1150
				handlerErr.Error() == "http: POST too large" { // https://go.dev/src/net/http/request.go#L1240
				errors.Convert(handlerErr).StatusCode = http.StatusRequestEntityTooLarge
			}
			c.LogError(frameCtx, "Handling request", log.Error(handlerErr), log.String("path", s.Path))

			// Prepare an error response instead
			httpRecorder = httpx.NewResponseRecorder()
			httpRecorder.Header().Set("Content-Type", "application/json")
			body, err := json.MarshalIndent(handlerErr, "", "\t")
			if err != nil {
				return errors.Trace(err)
			}
			statusCode := errors.Convert(handlerErr).StatusCode
			if statusCode == 0 {
				statusCode = http.StatusInternalServerError
			}
			httpRecorder.WriteHeader(statusCode)
			httpRecorder.Write(body)
		}

		// Meter
		_ = c.ObserveMetric(
			"microbus_response_duration_seconds",
			time.Since(handlerStartTime).Seconds(),
			s.Canonical(),
			strconv.Itoa(s.Port),
			httpReq.Method,
			strconv.Itoa(httpRecorder.StatusCode()),
			func() string {
				if handlerErr != nil {
					return "ERROR"
				}
				return "OK"
			}(),
		)
		_ = c.ObserveMetric(
			"microbus_response_size_bytes",
			float64(httpRecorder.ContentLength()),
			s.Canonical(),
			strconv.Itoa(s.Port),
			httpReq.Method,
			strconv.Itoa(httpRecorder.StatusCode()),
			func() string {
				if handlerErr != nil {
					return "ERROR"
				}
				return "OK"
			}(),
		)
	}

	// Set control headers on the response
	httpResponse := httpRecorder.Result()
	frame.Of(httpResponse).SetMessageID(msgID)
	frame.Of(httpResponse).SetFromHost(c.hostName)
	frame.Of(httpResponse).SetFromID(c.id)
	frame.Of(httpResponse).SetFromVersion(c.version)
	frame.Of(httpResponse).SetQueue(queue)
	frame.Of(httpResponse).SetOpCode(frame.OpCodeResponse)
	if handlerErr != nil {
		frame.Of(httpResponse).SetOpCode(frame.OpCodeError)
	}

	// Send back the response, in fragments if needed
	fragger, err := httpx.NewFragResponse(httpResponse, c.maxFragmentSize)
	if err != nil {
		return errors.Trace(err)
	}
	for f := 1; f <= fragger.N(); f++ {
		fragment, err := fragger.Fragment(f)
		if err != nil {
			return errors.Trace(err)
		}
		var buf bytes.Buffer
		err = fragment.Write(&buf)
		if err != nil {
			return errors.Trace(err)
		}
		err = c.natsConn.Publish(subjectOfResponses(c.plane, fromHost, fromId), buf.Bytes())
		if err != nil {
			return errors.Trace(err)
		}
	}

	return nil
}
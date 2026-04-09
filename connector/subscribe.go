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
	"bufio"
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/microbus-io/boolexp"
	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/fabric/pub"
	"github.com/microbus-io/fabric/sub"
	"github.com/microbus-io/fabric/transport"
	"github.com/microbus-io/fabric/trc"
	"github.com/microbus-io/fabric/utils"

	"go.opentelemetry.io/otel/propagation"
)

// HTTPHandler extends the standard http.Handler to also return an error
type HTTPHandler = sub.HTTPHandler

/*
Listen subscribes a handler under a typed name. The name must be a Go-style upper-case
identifier (e.g. "MyEndpoint2") and unique within the connector. Exactly one feature option
([sub.Function], [sub.Web], [sub.InboundEvent], [sub.Task], [sub.Workflow]) must be supplied.

Defaults filled in after options are applied:
  - Method defaults to "ANY".
  - Route defaults to ":443/my-subscription" with the port determined by the feature type
    (443 for function/web, 417 for inbound events, 428 for tasks/graphs).
  - Queue defaults to the connector's hostname.

Returns an error if the name is invalid, already registered, or if the option set is malformed.
*/
func (c *Connector) Subscribe(name string, handler sub.HTTPHandler, options ...sub.Option) error {
	if c.hostname == "" {
		return c.captureInitErr(errors.New("hostname is not set"))
	}
	newSub, err := sub.NewSubscription(name, c.hostname, handler, options...)
	if err != nil {
		return c.captureInitErr(errors.Trace(err))
	}
	if _, loaded := c.subs.LoadOrStore(name, newSub); loaded {
		return c.captureInitErr(errors.New("duplicate subscription name '%s'", name))
	}
	if c.isPhase(startedUp) {
		if err := c.activateSub(newSub); err != nil {
			c.subs.Delete(name)
			return c.captureInitErr(errors.Trace(err))
		}
		c.notifyOnNewSubs(newSub)
		c.transportConn.WaitForSub()
	}
	return nil
}

// Unsubscribe removes the subscription registered with [Connector.Subscribe] under the given name.
// Returns an error if no subscription with that name exists.
func (c *Connector) Unsubscribe(name string) error {
	s, ok := c.subs.Delete(name)
	if !ok {
		return errors.New("unknown subscription name '%s'", name)
	}
	if err := c.deactivateSub(s); err != nil {
		return errors.Trace(err)
	}
	if c.isPhase(startedUp) {
		c.transportConn.WaitForSub()
	}
	return nil
}

// onRequest handles an incoming request. It acks it, then calls the handler to process it and responds to the caller.
func (c *Connector) onRequest(msg *transport.Msg, s *sub.Subscription) {
	c.pendingOps.Add(1)

	if msg.Request == nil {
		// Parse the request
		httpReq, err := http.ReadRequest(bufio.NewReaderSize(bytes.NewReader(msg.Data), 64))
		if err != nil {
			c.pendingOps.Add(-1)
			err = errors.Trace(err)
			c.LogError(c.Lifetime(), "Parsing request", "error", err)
			return
		}
		msg.Request = httpReq
	}
	if msg.Request.Body == nil {
		msg.Request.Body = http.NoBody
	}

	err := c.ackRequest(msg, s)
	if err != nil {
		c.pendingOps.Add(-1)
		err = errors.Trace(err)
		c.LogError(c.Lifetime(), "Acking request", "error", err)
		return
	}
	go func() {
		defer c.pendingOps.Add(-1)
		err := c.handleRequest(msg, s)
		if err != nil {
			err = errors.Trace(err)
			c.LogError(c.Lifetime(), "Handling request", "error", err)
		}
	}()
}

// activateSubs subscribes all subscriptions with the transport.
// It is called during startup. Subscriptions already activated by
// [Connector.activateInfraSubs] are skipped via the idempotent guard in [Connector.activateSub].
func (c *Connector) activateSubs() (err error) {
	subs := c.subs.Values()
	for _, sub := range subs {
		err = c.activateSub(sub)
		if err != nil {
			return errors.Trace(err)
		}
	}
	if c.isPhase(startingUp) {
		c.notifyOnNewSubs(subs...)
	}
	c.transportConn.WaitForSub()
	return nil
}

// activateInfraSubs activates only subscriptions tagged with [sub.Infra].
// It is called during startup before the user's OnStartup callback runs, so
// framework-internal subscriptions (e.g. the distributed cache) are reachable
// from inside OnStartup. Peer notification is deferred to [Connector.activateSubs].
func (c *Connector) activateInfraSubs() error {
	activated := false
	for _, s := range c.subs.Values() {
		if !s.Infra {
			continue
		}
		if err := c.activateSub(s); err != nil {
			return errors.Trace(err)
		}
		activated = true
	}
	if activated {
		c.transportConn.WaitForSub()
	}
	return nil
}

// activateSub subscribes a single subscription with the transport.
func (c *Connector) activateSub(s *sub.Subscription) (err error) {
	if len(s.Subs) > 0 {
		return nil
	}
	// Recalculate the subscription path in case the hostname changed since it was created
	err = s.RefreshHostname(c.hostname)
	if err != nil {
		return errors.Trace(err)
	}
	// Create the subscriptions
	handler := func(msg *transport.Msg) {
		c.onRequest(msg, s)
	}
	prefixes := []string{
		"",
		c.id,
	}
	if c.locality != "" {
		loc := strings.Split(c.locality, ".")
		for i := len(loc) - 1; i >= 0; i-- {
			prefixes = append(prefixes, strings.Join(loc[i:], "."))
		}
	}
	for _, prefix := range prefixes {
		var transportSub *transport.Subscription
		if prefix != "" {
			prefix += "."
		}
		if s.Queue != "" {
			transportSub, err = c.transportConn.QueueSubscribe(subjectOfSubscription(c.plane, s.Method, prefix+s.Host, s.Port, s.Route), s.Queue, handler)
		} else {
			transportSub, err = c.transportConn.Subscribe(subjectOfSubscription(c.plane, s.Method, prefix+s.Host, s.Port, s.Route), handler)
		}
		if err != nil {
			break
		}
		s.Subs = append(s.Subs, transportSub)
	}
	if err != nil {
		c.LogError(c.Lifetime(), "Activating sub",
			"error", err,
			"url", s.Canonical(),
			"method", s.Method,
		)
		c.deactivateSub(s)
		return errors.Trace(err)
	}
	// c.LogDebug(c.Lifetime(), "Sub activated",
	// 	"url", s.Canonical(),
	// 	"method", req.Method,
	// )
	return nil
}

// deactivateSubs unsubscribes all subscriptions with the transport.
// It is called during shutdown.
func (c *Connector) deactivateSubs() error {
	var lastErr error
	for _, sub := range c.subs.Values() {
		err := c.deactivateSub(sub)
		if err != nil {
			lastErr = errors.Trace(err)
		}
	}
	c.transportConn.WaitForSub()
	return lastErr // No trace
}

// deactivateNonInfraSubs unsubscribes all subscriptions except those tagged with [sub.Infra].
// It is called during shutdown before OnShutdown runs, so user code stops receiving requests
// while infra subscriptions (e.g. the distributed cache) remain reachable to OnShutdown.
func (c *Connector) deactivateNonInfraSubs() error {
	var lastErr error
	deactivated := false
	for _, s := range c.subs.Values() {
		if s.Infra {
			continue
		}
		if err := c.deactivateSub(s); err != nil {
			lastErr = errors.Trace(err)
		}
		deactivated = true
	}
	if deactivated {
		c.transportConn.WaitForSub()
	}
	return lastErr // No trace
}

// deactivateInfraSubs unsubscribes only subscriptions tagged with [sub.Infra].
// It is called during shutdown after OnShutdown returns, so framework-internal subscriptions
// (e.g. the distributed cache) remain reachable while user shutdown code runs.
func (c *Connector) deactivateInfraSubs() error {
	var lastErr error
	deactivated := false
	for _, s := range c.subs.Values() {
		if !s.Infra {
			continue
		}
		if err := c.deactivateSub(s); err != nil {
			lastErr = errors.Trace(err)
		}
		deactivated = true
	}
	if deactivated {
		c.transportConn.WaitForSub()
	}
	return lastErr // No trace
}

// deactivateSub unsubscribes a single subscription with the transport.
func (c *Connector) deactivateSub(s *sub.Subscription) error {
	var lastErr error
	for _, sub := range s.Subs {
		err := sub.Unsubscribe()
		if err != nil {
			lastErr = errors.Trace(err)
		}
	}
	if lastErr != nil {
		c.LogError(c.Lifetime(), "Deactivating sub",
			"error", lastErr,
			"url", s.Canonical(),
			"method", s.Method,
		)
	}
	s.Subs = nil
	// c.LogDebug(c.Lifetime(), "Sub deactivated",
	// 	"url", s.Canonical(),
	// 	"method", s.Method,
	// )
	return lastErr
}

// ackRequest sends an ack response back to the caller.
// Acks are sent as soon as a request is received to let the caller know it is being processed.
func (c *Connector) ackRequest(msg *transport.Msg, s *sub.Subscription) (err error) {
	httpReq := msg.Request
	if httpReq == nil {
		// Parse only the headers of the request
		headerData := msg.Data
		eoh := bytes.Index(headerData, []byte("\r\n\r\n"))
		if eoh >= 0 {
			headerData = headerData[:eoh+4]
		}
		httpReq, err = http.ReadRequest(bufio.NewReaderSize(bytes.NewReader(headerData), 64))
		if err != nil {
			return errors.Trace(err)
		}
	}

	// Get return address
	frm := frame.Of(httpReq)
	fromHost := frm.FromHost()
	if fromHost == "" {
		return errors.New("empty " + frame.HeaderFromHost + " header")
	}
	fromID := frm.FromID()
	if fromID == "" {
		return errors.New("empty " + frame.HeaderFromId + " header")
	}
	msgID := frm.MessageID()
	if msgID == "" {
		return errors.New("empty " + frame.HeaderMsgId + " header")
	}
	queue := s.Queue
	_, fragmentMax := frm.Fragment()

	// Prepare and send the ack
	httpRes := &http.Response{
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header: http.Header{
			"Connection": []string{"close"},
		},
	}
	frm = frame.Of(httpRes)
	frm.SetOpCode(frame.OpCodeAck)
	frm.SetFromHost(c.hostname)
	frm.SetFromID(c.id)
	frm.SetMessageID(msgID)
	frm.SetQueue(queue)
	frm.SetLocality(c.locality)
	if fragmentMax > 1 {
		httpRes.StatusCode = http.StatusContinue
		httpRes.Status = "100 Continue"
	} else {
		httpRes.StatusCode = http.StatusAccepted
		httpRes.Status = "202 Accepted"
	}
	err = c.transportConn.Response(subjectOfResponses(c.plane, fromHost, fromID), httpRes)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// handleRequest is called when an incoming HTTP request is received.
// The message is dispatched to the appropriate web handler and the response is serialized and sent back to the response channel of the sender.
func (c *Connector) handleRequest(msg *transport.Msg, s *sub.Subscription) (err error) {
	ctx := c.Lifetime()

	httpReq := msg.Request
	if httpReq == nil {
		// Parse the request
		httpReq, err = http.ReadRequest(bufio.NewReaderSize(bytes.NewReader(msg.Data), 64))
		if err != nil {
			return errors.Trace(err)
		}
	}
	if httpReq.Header.Get("User-Agent") == "Go-http-client/1.1" {
		// Remove the default user-agent set by the http package
		httpReq.Header.Del("User-Agent")
	}
	httpx.SetPathValues(httpReq, s.Route)
	if br, ok := httpReq.Body.(*httpx.BodyReader); ok {
		br.Reset()
	}

	// Get the sender hostname and message ID
	fromHost := frame.Of(httpReq).FromHost()
	fromId := frame.Of(httpReq).FromID()
	msgID := frame.Of(httpReq).MessageID()
	queue := s.Queue

	c.LogDebug(c.Lifetime(), "Handling",
		"msg", msgID,
		"url", s.Canonical(),
		"method", s.Method,
	)

	// Time budget
	budget := frame.Of(httpReq).TimeBudget()
	if budget <= 0 {
		budget = c.defaultTimeBudget
	}
	budget = min(budget, c.maxTimeBudget)
	frame.Of(httpReq).SetTimeBudget(budget)
	if budget <= c.networkRoundtrip {
		return errors.New("timeout", http.StatusRequestTimeout)
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

	// OpenTelemetry: create a child span
	spanOptions := []trc.Option{
		trc.Server(),
		// Do not record the request attributes yet because they take a lot of memory, they will be added if there's an error
	}
	if c.deployment == LOCAL {
		// Add the request attributes in LOCAL deployment to facilitate debugging
		spanOptions = append(spanOptions, trc.Request(httpReq), trc.String("http.route", s.Route))
	}
	ctx = propagation.TraceContext{}.Extract(ctx, propagation.HeaderCarrier(httpReq.Header))
	ctx, span := c.StartSpan(ctx, fmt.Sprintf(":%s%s", s.Port, s.Route), spanOptions...)
	spanEnded := false
	defer func() {
		if !spanEnded {
			span.End()
		}
	}()

	// Execute the request
	handlerStartTime := time.Now()
	httpRecorder := httpx.NewResponseRecorder()
	var handlerErr error

	// Prepare the context with a timeout set to the time budget reduced by a network hop
	ctx = frame.ContextWithClonedFrameOf(ctx, httpReq.Header)
	ctx, cancel := context.WithTimeout(ctx, budget-c.networkRoundtrip)
	httpReq = httpReq.WithContext(ctx)
	httpReq.Header = frame.Of(ctx).Header()

	// Check actor constraints
	if s.RequiredClaims != "" {
		actor := httpReq.Header.Get(frame.HeaderActor)
		if actor == "" || !utils.LooksLikeJWT(actor) {
			handlerErr = errors.New("", http.StatusUnauthorized)
		} else {
			var claims jwt.MapClaims
			claims, handlerErr = c.verifyToken(actor, s.RequiredClaims)
			if handlerErr == nil {
				ctx = logActorClaims(ctx, claims)
				httpReq = httpReq.WithContext(ctx)
			}
		}
	}

	// Call the handler
	if handlerErr == nil {
		handlerErr = errors.CatchPanic(func() error {
			return s.Handler.(HTTPHandler)(httpRecorder, httpReq)
		})
	}
	cancel()

	if handlerErr != nil {
		convertedErr := errors.Convert(handlerErr)
		handlerErr = convertedErr
		if convertedErr.Error() == "http: request body too large" || // https://go.dev/src/net/http/request.go#L1150
			convertedErr.Error() == "http: POST too large" { // https://go.dev/src/net/http/request.go#L1240
			convertedErr.StatusCode = http.StatusRequestEntityTooLarge
		}
		statusCode := convertedErr.StatusCode
		if statusCode == 0 {
			statusCode = http.StatusInternalServerError
		}
		c.LogError(ctx, "Handling request",
			"error", convertedErr,
			"path", s.Route,
			"depth", frame.Of(httpReq).CallDepth(),
			"code", statusCode,
		)

		// OpenTelemetry: record the error, adding the request attributes
		span.SetAttributes("http.route", s.Route)
		span.SetRequest(httpReq)
		span.SetError(convertedErr)
		c.ForceTrace(ctx)

		// Enrich error with trace ID
		convertedErr.Trace = span.TraceID()

		// Prepare an error response instead
		httpRecorder = httpx.NewResponseRecorder()
		httpRecorder.Header().Set("Content-Type", "application/json")
		httpRecorder.WriteHeader(statusCode)
		encoder := json.NewEncoder(httpRecorder)
		if c.Deployment() == LOCAL {
			encoder.SetIndent("", "  ")
		}
		serializedErr := struct {
			Err error `json:"err"`
		}{
			Err: convertedErr,
		}
		err = encoder.Encode(serializedErr)
		if err != nil {
			return errors.Trace(err)
		}
	}

	// Meter
	_ = c.RecordHistogram(
		ctx,
		"microbus_server_request_duration_seconds",
		time.Since(handlerStartTime).Seconds(),
		"handler", s.Canonical(),
		"port", s.Port,
		"method", httpReq.Method,
		"code", httpRecorder.StatusCode(),
		"error", func() string {
			if handlerErr != nil {
				return "ERROR"
			}
			return "OK"
		}(),
	)
	_ = c.RecordHistogram(
		ctx,
		"microbus_server_response_body_bytes",
		float64(httpRecorder.ContentLength()),
		"handler", s.Canonical(),
		"port", s.Port,
		"method", httpReq.Method,
		"code", httpRecorder.StatusCode(),
		"error", func() string {
			if handlerErr != nil {
				return "ERROR"
			}
			return "OK"
		}(),
	)

	// Set control headers on the response
	httpResponse := httpRecorder.Result()
	frame.Of(httpResponse).SetMessageID(msgID)
	frame.Of(httpResponse).SetFromHost(c.hostname)
	frame.Of(httpResponse).SetFromID(c.id)
	frame.Of(httpResponse).SetFromVersion(c.version)
	frame.Of(httpResponse).SetQueue(queue)
	if handlerErr != nil {
		frame.Of(httpResponse).SetOpCode(frame.OpCodeError)
	} else {
		frame.Of(httpResponse).SetOpCode(frame.OpCodeResponse)
	}
	frame.Of(httpResponse).SetLocality(c.locality)

	// OpenTelemetry: record the status code
	if handlerErr == nil {
		span.SetOK(httpResponse.StatusCode)
	}
	span.End()
	spanEnded = true

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
		err = c.transportConn.Response(subjectOfResponses(c.plane, fromHost, fromId), fragment)
		if err != nil {
			return errors.Trace(err)
		}
	}

	return nil
}

// notifyOnNewSubs notifies all microservices of this microservice's new subscriptions,
// allowing them to invalidate their known responders cache appropriately.
func (c *Connector) notifyOnNewSubs(subs ...*sub.Subscription) error {
	if len(subs) == 0 {
		return nil
	}
	hosts := make([]string, 0, len(subs))
	added := map[string]bool{}
	for _, s := range subs {
		if added[s.Host] {
			continue
		}
		hosts = append(hosts, s.Host)
		added[s.Host] = true
	}
	if len(hosts) == 0 {
		return nil
	}
	payload := struct {
		Hosts []string `json:"hosts"`
	}{
		Hosts: hosts,
	}
	ch := c.Publish(c.Lifetime(),
		pub.POST("https://all:888/on-new-subs"),
		pub.Body(payload),
		pub.Multicast(),
	)
	for range ch {
	}
	return nil
}

// verifyToken verifies the signature of an actor JWT and evaluates requiredClaims against
// its payload. On success it returns the verified claims, which the caller may cache for
// downstream non-security uses such as logging.
// If the kid is unknown, it fetches JWKS from the issuer host.
func (c *Connector) verifyToken(token string, requiredClaims string) (jwt.MapClaims, error) {
	// Get the alg and kid from the token's header
	parsed, _, err := jwt.NewParser().ParseUnverified(token, jwt.MapClaims{})
	if err != nil {
		return nil, errors.New("", http.StatusUnauthorized)
	}
	alg, _ := parsed.Header["alg"].(string)

	// Unsigned tokens (e.g. those minted by pub.Actor) are accepted only in TESTING,
	// where signing is impractical. Signature verification is skipped, but the claim
	// evaluation below still runs - an unsigned token's payload must satisfy
	// requiredClaims just like a signed one.
	if alg == "none" {
		if c.deployment != TESTING {
			return nil, errors.New("", http.StatusUnauthorized)
		}
		claims, ok := parsed.Claims.(jwt.MapClaims)
		if !ok {
			return nil, errors.New("", http.StatusUnauthorized)
		}
		satisfy, err := boolexp.Eval(requiredClaims, map[string]any(claims))
		if err != nil {
			return nil, errors.Trace(err)
		}
		if !satisfy {
			return nil, errors.New("", http.StatusForbidden)
		}
		return claims, nil
	}

	kid, _ := parsed.Header["kid"].(string)
	if kid == "" {
		return nil, errors.New("", http.StatusUnauthorized)
	}

	// Verify the token with the issuer
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("", http.StatusUnauthorized)
	}
	issStr, _ := claims["iss"].(string)
	_, ok = claims["microbus"].(string)
	if !ok && !strings.HasPrefix(issStr, "microbus://") { // microbus scheme for backward compatibility
		return nil, errors.New("", http.StatusUnauthorized)
	}
	_, issuerHost, ok := strings.Cut(issStr, "://")
	if !ok {
		issuerHost = issStr
	}

	// Look up the public key, refresh cache if needed
	key, found := c.lookupActorKey(kid)
	if !found {
		err = c.fetchActorKeys(issuerHost)
		if err != nil {
			return nil, errors.New("", http.StatusUnauthorized, err)
		}
		key, found = c.lookupActorKey(kid)
		if !found {
			return nil, errors.New("", http.StatusUnauthorized)
		}
	}

	// Verify the JWT signature
	_, err = jwt.Parse(token, func(t *jwt.Token) (any, error) {
		return key, nil
	})
	if err != nil {
		return nil, errors.New("", http.StatusUnauthorized)
	}

	// Check the required claims against the access token's claims
	satisfy, err := boolexp.Eval(requiredClaims, map[string]any(claims))
	if err != nil {
		return nil, errors.Trace(err)
	}
	if !satisfy {
		return nil, errors.New("", http.StatusForbidden)
	}

	return claims, nil
}

// lookupActorKey returns the cached Ed25519 public key for the given kid.
func (c *Connector) lookupActorKey(kid string) (ed25519.PublicKey, bool) {
	c.actorKeysLock.RLock()
	defer c.actorKeysLock.RUnlock()
	key, ok := c.actorKeys[kid]
	return key, ok
}

// fetchActorKeys fetches JWKS from the given host at :888/jwks and updates the key cache.
func (c *Connector) fetchActorKeys(host string) error {
	resp, err := c.Request(
		c.Lifetime(),
		pub.GET("https://"+host+":888/jwks"),
	)
	if err != nil {
		return errors.Trace(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return errors.Trace(err)
	}
	var jwksResp struct {
		Keys []struct {
			KID string `json:"kid"`
			X   string `json:"x"`
		} `json:"keys"`
	}
	err = json.Unmarshal(body, &jwksResp)
	if err != nil {
		return errors.Trace(err)
	}

	c.actorKeysLock.Lock()
	defer c.actorKeysLock.Unlock()
	if c.actorKeys == nil {
		c.actorKeys = make(map[string]ed25519.PublicKey)
	}
	for _, jwk := range jwksResp.Keys {
		pubBytes, err := base64.RawURLEncoding.DecodeString(jwk.X)
		if err != nil {
			continue
		}
		c.actorKeys[jwk.KID] = ed25519.PublicKey(pubBytes)
	}
	return nil
}

// invalidateKnownRespondersCache clears known responder cache for any of the indicated hosts.
// It is called in response to a notification from other microservices on new subscriptions.
func (c *Connector) invalidateKnownRespondersCache(hosts []string) error {
	if len(hosts) == 0 {
		return nil
	}
	c.knownResponders.DeletePredicate(func(key string) bool {
		for _, host := range hosts {
			revHost := "." + reverseHostname(host) + ".|"
			if strings.Contains(key, revHost) {
				c.LogDebug(c.Lifetime(), "Invalidating known responders cache", "host", host)
				return true
			}
		}
		return false
	})
	return nil
}

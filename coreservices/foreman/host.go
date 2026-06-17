package foreman

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/microbus-io/dwarf/workflow"
	"github.com/microbus-io/errors"

	"github.com/microbus-io/fabric/coreservices/accesstoken/accesstokenapi"
	"github.com/microbus-io/fabric/coreservices/foreman/foremanapi"
	"github.com/microbus-io/fabric/pub"
)

// This file implements the dwarf engine.Host interface for the Microbus transport. The engine owns the
// orchestration; these four methods are the only seam to the bus:
//   - LoadGraph   - GET the workflow graph over the bus.
//   - ExecuteTask - mint the actor token from baggage, POST the flow to the task, classify the error.
//   - FlowStopped - fire the OnFlowStopped outbound event to the notify host carried in the flow's baggage.
//   - SignalPeers - multicast the opaque (op, payload) to peer replicas, excluding self.

// breaker cause labels handed to the engine on a breaker trip. Opaque to the engine (used only as a
// metric label); defined here because error→cause classification is the host's responsibility.
const (
	breakerCauseAckTimeout  = "ack_timeout"
	breakerCauseUnavailable = "unavailable"
	breakerCauseOverloaded  = "overloaded"

	// statusOverloaded is the non-standard 529 "site overloaded" code (Cloudflare and some third-party
	// APIs); net/http has no constant for it.
	statusOverloaded = 529

	// baggageNotifyHost is the baggage key under which resolveOptions stamps the caller's host when the
	// caller set FlowOptions.NotifyOnStop, so FlowStopped can deliver the OnFlowStopped event back to it.
	// It is foreman bookkeeping, not an actor claim (mintActorToken strips it before minting).
	baggageNotifyHost = "notifyHost"
)

// LoadGraph fetches a workflow graph by issuing a GET to its URL over the bus and decoding the
// {"graph": ...} wrapper the workflow endpoint serves. Implements engine.Host.
func (svc *Service) LoadGraph(ctx context.Context, workflowURL string) (*workflow.Graph, error) {
	u := workflowURL
	if !strings.Contains(u, "://") {
		u = "https://" + u
	}
	httpRes, err := svc.Request(ctx, pub.Method("GET"), pub.URL(u))
	if err != nil {
		return nil, errors.Trace(err)
	}
	var wrapper struct {
		Graph workflow.Graph `json:"graph"`
	}
	err = json.NewDecoder(httpRes.Body).Decode(&wrapper)
	if err != nil {
		return nil, errors.Trace(err)
	}
	err = wrapper.Graph.Validate()
	if err != nil {
		return nil, errors.Trace(err, http.StatusBadRequest)
	}
	return &wrapper.Graph, nil
}

// ExecuteTask dispatches one task over the bus: it mints an actor token from the flow's baggage, POSTs
// the flow carrier to the task URL, and applies the task's returned flow back onto the carrier so the
// engine sees the changes and any control signals (goto/retry/sleep/interrupt/subgraph) the task armed.
// A transport error is classified into the engine's disposition wrappers (see classifyTaskError).
// Implements engine.Host.
func (svc *Service) ExecuteTask(ctx context.Context, taskURL string, flow *workflow.Flow) error {
	actorToken, err := svc.mintActorToken(ctx)
	if err != nil {
		return errors.Trace(err)
	}
	body, err := json.Marshal(flow)
	if err != nil {
		return errors.Trace(err)
	}
	u := taskURL
	if !strings.Contains(u, "://") {
		u = "https://" + u
	}
	opts := []pub.Option{
		pub.Method("POST"),
		pub.URL(u),
		pub.Body(body),
		pub.ContentType("application/json"),
	}
	if actorToken != "" {
		opts = append(opts, pub.Token(actorToken))
	}
	// The engine already bounded ctx with the step's time budget; svc.Request honors that deadline.
	httpRes, err := svc.Request(ctx, opts...)
	if err != nil {
		return classifyTaskError(err)
	}
	respBody, err := io.ReadAll(httpRes.Body)
	if err != nil {
		return errors.Trace(err)
	}
	// Unmarshal directly onto the carrier: Flow.UnmarshalJSON replaces its state, changes, and control
	// signals from the task's returned flow, in place, which is exactly what the engine reads back.
	err = json.Unmarshal(respBody, flow)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// classifyTaskError maps a bus transport error to the engine's disposition wrappers. The engine never
// inspects status codes itself; the host owns the mapping. A 429 engages backpressure (rate-cut + bounce);
// a 404 ack-timeout / 503 unavailable / 529 overloaded trips the breaker (park + exponential probe). Any
// other error is returned undecorated - an ordinary failure the engine routes via onError or fails.
func classifyTaskError(err error) error {
	switch {
	case errors.StatusCode(err) == http.StatusTooManyRequests:
		return workflow.ErrRateLimited(err, "")
	case errors.StatusCode(err) == http.StatusNotFound && strings.HasPrefix(err.Error(), "ack timeout"):
		return workflow.ErrUnavailable(err, breakerCauseAckTimeout)
	case errors.StatusCode(err) == http.StatusServiceUnavailable:
		return workflow.ErrUnavailable(err, breakerCauseUnavailable)
	case errors.StatusCode(err) == statusOverloaded:
		return workflow.ErrUnavailable(err, breakerCauseOverloaded)
	}
	return errors.Trace(err)
}

// mintActorToken mints a fresh access token from the actor claims carried in the flow's baggage (captured
// from the original caller at Create), so the downstream task runs as that actor. Returns an empty token
// when the flow has no actor claims. The iss/idp swap mirrors the legacy foreman: the minted token's
// issuer is the actor's original identity provider.
func (svc *Service) mintActorToken(ctx context.Context) (string, error) {
	baggage, _ := workflow.BaggageFrom(ctx).(map[string]any)
	if len(baggage) == 0 {
		return "", nil
	}
	actorClaims := make(map[string]any, len(baggage))
	for k, v := range baggage {
		actorClaims[k] = v
	}
	// baggageNotifyHost is foreman bookkeeping (the FlowStopped delivery target), not an actor claim - keep
	// it out of the minted token.
	delete(actorClaims, baggageNotifyHost)
	if len(actorClaims) == 0 {
		return "", nil
	}
	iss, _ := actorClaims["iss"].(string)
	iss = stripProto(iss)
	actorClaims["iss"] = actorClaims["idp"]
	delete(actorClaims, "idp")
	token, err := accesstokenapi.NewClient(svc).ForHost(iss).Mint(ctx, actorClaims)
	if err != nil {
		return "", errors.Trace(err)
	}
	return token, nil
}

// FlowStopped fires the OnFlowStopped outbound event to the notify host carried in the flow's baggage.
// The engine traffics in no delivery address: when the caller set FlowOptions.NotifyOnStop, resolveOptions
// stamped the caller's host into baggage under baggageNotifyHost at Create, and it rides back here on the
// ctx. Absent (caller did not opt in) means nothing to deliver. Implements engine.Host.
func (svc *Service) FlowStopped(ctx context.Context, flowKey string, outcome *workflow.FlowOutcome) {
	baggage, _ := workflow.BaggageFrom(ctx).(map[string]any)
	host, _ := baggage[baggageNotifyHost].(string)
	if host == "" {
		return
	}
	for range foremanapi.NewMulticastTrigger(svc).ForHost(host).OnFlowStopped(ctx, flowKey, outcome) {
	}
}

// SignalPeers multicasts an opaque cross-replica coordination signal to the other foreman replicas via
// the single Signal endpoint. Implements engine.Host.
func (svc *Service) SignalPeers(ctx context.Context, op string, payload []byte) {
	for range foremanapi.NewMulticastClient(svc).Signal(ctx, op, payload) {
	}
}

// stripProto removes the scheme (e.g. "https://") from a URL, returning the bare host/path.
func stripProto(u string) string {
	_, right, cut := strings.Cut(u, "://")
	if !cut {
		return u
	}
	return right
}

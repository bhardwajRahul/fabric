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

// Package svcapi is a Phase 0 pressure-test fixture: a single definition.go that exercises every
// define.* feature kind and every locked edge case (magic HTTP args, the xxxOut task suffix, a
// cross-package inbound event, route shapes, claims, time budgets, observable metrics, callbacks).
// It exists only to prove the define vocabulary compiles and can express the full contract surface.
package svcapi

import (
	"time"

	"github.com/microbus-io/fabric/define"

	"github.com/microbus-io/fabric/cmd/genservice/testdata/pressuretest/srcapi"
)

// Hostname is the default hostname of the microservice.
const Hostname = "svc.pressuretest"

// ---- Functions ----

// DoThing performs a thing and exercises claims and a time budget.
var DoThing = define.Function{
	Host: Hostname, Method: "POST", Route: ":443/do-thing",
	RequiredClaims: `roles.admin || roles.elevated`,
	TimeBudget:     5 * time.Second,
	In:             DoThingIn{}, Out: DoThingOut{},
}

// DoThingIn are the input arguments of DoThing.
type DoThingIn struct {
	Name  string `json:"name,omitzero" jsonschema_description:"Name is the subject of the thing"`
	Count int    `json:"count,omitzero"`
}

// DoThingOut are the output arguments of DoThing.
type DoThingOut struct {
	Result string `json:"result,omitzero"`
}

// CreatePet exercises the httpRequestBody / httpStatusCode magic arguments (REST create).
var CreatePet = define.Function{
	Host: Hostname, Method: "POST", Route: ":443/pets",
	In: CreatePetIn{}, Out: CreatePetOut{},
}

// CreatePetIn are the input arguments of CreatePet.
type CreatePetIn struct {
	HTTPRequestBody *Pet `json:"-"`
}

// CreatePetOut are the output arguments of CreatePet.
type CreatePetOut struct {
	Key            string `json:"key,omitzero"`
	HTTPStatusCode int    `json:"-"`
}

// LoadPet exercises the httpResponseBody / httpStatusCode magic arguments (REST load).
var LoadPet = define.Function{
	Host: Hostname, Method: "GET", Route: ":443/pets/{petID}",
	In: LoadPetIn{}, Out: LoadPetOut{},
}

// LoadPetIn are the input arguments of LoadPet.
type LoadPetIn struct {
	PetID int64 `json:"petID,omitzero"`
}

// LoadPetOut are the output arguments of LoadPet.
type LoadPetOut struct {
	HTTPResponseBody *Pet `json:"-"`
	HTTPStatusCode   int  `json:"-"`
}

// SelfPing is an internal-only endpoint that multicasts to all peers (no queue).
var SelfPing = define.Function{
	Host: Hostname, Method: "POST", Route: ":444/self-ping",
	LoadBalancing: define.None,
	In:            SelfPingIn{}, Out: SelfPingOut{},
}

// SelfPingIn are the input arguments of SelfPing.
type SelfPingIn struct{}

// SelfPingOut are the output arguments of SelfPing.
type SelfPingOut struct{}

// AltHost registers on a non-own host via the //host:port/path form.
var AltHost = define.Function{
	Host: Hostname, Method: "GET", Route: "//alt.pressuretest:0/alt",
	In: AltHostIn{}, Out: AltHostOut{},
}

// AltHostIn are the input arguments of AltHost.
type AltHostIn struct{}

// AltHostOut are the output arguments of AltHost.
type AltHostOut struct{}

// ---- Web ----

// Dashboard serves an HTML dashboard on any method.
var Dashboard = define.Web{
	Host: Hostname, Method: "ANY", Route: "/dashboard",
}

// ---- Tasks ----

// ProcessStep is a workflow task that transforms state.
var ProcessStep = define.Task{
	Host: Hostname, Method: "POST", Route: ":428/process-step",
	In: ProcessStepIn{}, Out: ProcessStepOut{},
}

// ProcessStepIn are the input arguments of ProcessStep.
type ProcessStepIn struct {
	Item Item `json:"item,omitzero"`
}

// ProcessStepOut are the output arguments of ProcessStep.
type ProcessStepOut struct {
	Processed bool `json:"processed,omitzero"`
}

// ReviewStep is a workflow task whose output reads-modifies-writes the shared "count" state field,
// exercising the xxxOut suffix convention (Go field CountOut, JSON tag count).
var ReviewStep = define.Task{
	Host: Hostname, Method: "POST", Route: ":428/review-step",
	TimeBudget: 1 * time.Second,
	In:         ReviewStepIn{}, Out: ReviewStepOut{},
}

// ReviewStepIn are the input arguments of ReviewStep.
type ReviewStepIn struct {
	Count int `json:"count,omitzero"`
}

// ReviewStepOut are the output arguments of ReviewStep.
type ReviewStepOut struct {
	CountOut int `json:"count,omitzero"`
}

// ---- Workflow ----

// MainFlow is the top-level workflow graph.
var MainFlow = define.Workflow{
	Host: Hostname, Method: "GET", Route: ":428/main-flow",
	In: MainFlowIn{}, Out: MainFlowOut{},
}

// MainFlowIn are the input arguments of MainFlow.
type MainFlowIn struct {
	Item Item `json:"item,omitzero"`
}

// MainFlowOut are the output arguments of MainFlow.
type MainFlowOut struct {
	Done bool `json:"done,omitzero"`
}

// ---- Outbound event ----

// OnLocalEvent fires when this microservice completes a local action.
var OnLocalEvent = define.OutboundEvent{
	Host: Hostname, Method: "POST", Route: ":417/on-local-event",
	In: OnLocalEventIn{}, Out: OnLocalEventOut{},
}

// OnLocalEventIn are the input arguments of OnLocalEvent.
type OnLocalEventIn struct {
	Note string `json:"note,omitzero"`
}

// OnLocalEventOut are the output arguments of OnLocalEvent.
type OnLocalEventOut struct {
	Ack bool `json:"ack,omitzero"`
}

// ---- Inbound event ----

// OnSrcEvent handles the upstream srcapi.OnSrcEvent event. The handler method is svc.OnSrcEvent and
// its signature is taken from the referenced source event's In/Out.
var OnSrcEvent = define.InboundEvent{
	Source: srcapi.OnSrcEvent,
}

// ---- Configs ----

// APIKey is a stored credential; never logged.
var APIKey = define.Config{
	Value:  string(""),
	Secret: true,
}

// MaxItems caps the number of items processed per run; changes fire OnChangedMaxItems.
var MaxItems = define.Config{
	Value:      int(0),
	Default:    "100",
	Validation: "int [1,1000]",
	Callback:   true,
}

// RefreshInterval controls how often state is refreshed.
var RefreshInterval = define.Config{
	Value:      time.Duration(0),
	Default:    "1m",
	Validation: "dur (0s,24h]",
}

// RoutingPolicy is a structured (JSON) configuration value, exercising the struct type carrier.
var RoutingPolicy = define.Config{
	Value:      Policy{},
	Default:    `{"mode":"balanced"}`,
	Validation: "json",
}

// ---- Metrics ----

// RequestsTotal counts requests handled, labelled by status.
var RequestsTotal = define.Metric{
	Kind: define.Counter, Value: int(0), Labels: []string{"status"},
	OTelName: "svc_requests_total",
}

// QueueDepth records the current queue depth, observed just-in-time via OnObserveQueueDepth.
var QueueDepth = define.Metric{
	Kind: define.Gauge, Value: int(0),
	OTelName: "svc_queue_depth", Observable: true,
}

// LatencySeconds records request latency in seconds.
var LatencySeconds = define.Metric{
	Kind: define.Histogram, Value: float64(0),
	Buckets:  []float64{0.01, 0.05, 0.1, 0.5, 1, 5},
	OTelName: "svc_latency_seconds",
}

// ---- Ticker ----

// Reconcile runs a periodic reconciliation.
var Reconcile = define.Ticker{
	Interval: 30 * time.Second,
}

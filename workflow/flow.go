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

package workflow

import (
	"encoding/json"
	"maps"
	"math"
	"reflect"
	"time"
)

// Flow is the carrier object passed to tasks. It holds the state and control
// signals for a single step in a workflow execution.
type Flow struct {
	// State
	state   map[string]any
	changes map[string]any

	// Control
	gotoNext         string
	retry            bool
	sleepDuration    time.Duration
	interrupt        bool
	interruptPayload map[string]any

	// Dynamic subgraph
	subgraphWorkflow string
	subgraphInput    map[string]any

	// Backoff retry
	attempt                int
	backoffMaxAttempts     int
	backoffInitialDelay    time.Duration
	backoffDelayMultiplier float64
	backoffMaxDelay        time.Duration
}

// NewFlow creates a new Flow with initialized maps.
func NewFlow() *Flow {
	return &Flow{
		state:   make(map[string]any),
		changes: make(map[string]any),
	}
}

// --- State access ---

// GetString returns a state field as a string.
func (f *Flow) GetString(key string) string {
	var v string
	getFromMap(f.state, key, &v)
	return v
}

// GetStrings returns a state field as a string slice.
func (f *Flow) GetStrings(key string) []string {
	var v []string
	getFromMap(f.state, key, &v)
	return v
}

// GetInt returns a state field as an int.
func (f *Flow) GetInt(key string) int {
	var v int
	getFromMap(f.state, key, &v)
	return v
}

// GetFloat returns a state field as a float64.
func (f *Flow) GetFloat(key string) float64 {
	var v float64
	getFromMap(f.state, key, &v)
	return v
}

// GetBool returns a state field as a bool.
func (f *Flow) GetBool(key string) bool {
	var v bool
	getFromMap(f.state, key, &v)
	return v
}

// GetDuration returns a state field as a time.Duration.
func (f *Flow) GetDuration(key string) time.Duration {
	var v time.Duration
	getFromMap(f.state, key, &v)
	return v
}

// Get unmarshals a state field into the target. Use this for complex types (structs, maps, etc.).
func (f *Flow) Get(key string, target any) error {
	return getFromMap(f.state, key, target)
}

// Has reports whether a state field exists.
func (f *Flow) Has(key string) bool {
	_, ok := f.state[key]
	return ok
}

// ParseState unmarshals state fields into the target struct.
// Fields are matched by their JSON tag names. Fields in state that are not in the struct are ignored.
func (f *Flow) ParseState(target any) error {
	return parseMapInto(f.state, target)
}

// --- State mutation ---

// Set sets a state field and tracks the change. Use this for complex types (structs, maps, etc.).
func (f *Flow) Set(key string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if f.state == nil {
		f.state = make(map[string]any)
	}
	if f.changes == nil {
		f.changes = make(map[string]any)
	}
	raw := json.RawMessage(data)
	f.state[key] = raw
	f.changes[key] = raw
	return nil
}

// SetString sets a state string field and tracks the change.
func (f *Flow) SetString(key string, value string) {
	f.set(key, value)
}

// SetStrings sets a state string slice field and tracks the change.
func (f *Flow) SetStrings(key string, value []string) {
	f.set(key, value)
}

// SetInt sets a state int field and tracks the change.
func (f *Flow) SetInt(key string, value int) {
	f.set(key, value)
}

// SetFloat sets a state float64 field and tracks the change.
func (f *Flow) SetFloat(key string, value float64) {
	f.set(key, value)
}

// SetBool sets a state bool field and tracks the change.
func (f *Flow) SetBool(key string, value bool) {
	f.set(key, value)
}

// SetDuration sets a state time.Duration field and tracks the change.
func (f *Flow) SetDuration(key string, value time.Duration) {
	f.set(key, value)
}

// set is an internal helper that marshals a value and records it in state and changes.
func (f *Flow) set(key string, value any) {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err) // should never happen for primitive types
	}
	if f.state == nil {
		f.state = make(map[string]any)
	}
	if f.changes == nil {
		f.changes = make(map[string]any)
	}
	raw := json.RawMessage(data)
	f.state[key] = raw
	f.changes[key] = raw
}

// Snapshot captures a read-only copy of the flow's current state
// (including any changes applied so far). Pass the returned snapshot to SetChanges
// to record only the fields that differ.
func (f *Flow) Snapshot() map[string]any {
	snap := make(map[string]any, len(f.state))
	maps.Copy(snap, f.state)
	return snap
}

// SetState marshals the source struct fields into state without tracking changes.
// Fields are matched by their JSON tag names.
func (f *Flow) SetState(source any) error {
	if f.state == nil {
		f.state = make(map[string]any)
	}
	return f.applyFields(source, f.state)
}

// SetChanges marshals the source struct back to state, comparing against the provided snapshot.
// Only fields whose JSON value differs from the snapshot are recorded as changes.
// Changed fields are written to both the state and changes maps, so that subsequent reads
// (including transition condition evaluation) see the updated values.
func (f *Flow) SetChanges(source any, snap map[string]any) error {
	if f.changes == nil {
		f.changes = make(map[string]any)
	}
	if f.state == nil {
		f.state = make(map[string]any)
	}
	return f.diffAndApply(source, snap, f.state, f.changes)
}

// --- Control ---

// Goto overrides transition routing. The orchestrator skips condition evaluation
// and follows the specified task instead.
func (f *Flow) Goto(taskName string) {
	f.gotoNext = taskName
}

// Interrupt pauses the flow execution and requests external input.
// The payload is propagated up through the surgraph chain and surfaced
// via State() so the caller can see what data the task needs.
// The task should return normally after calling Interrupt.
func (f *Flow) Interrupt(payload any) {
	f.interrupt = true
	if payload != nil {
		payloadJSON, err := json.Marshal(payload)
		if err == nil {
			var payloadMap map[string]any
			if err = json.Unmarshal(payloadJSON, &payloadMap); err == nil {
				f.interruptPayload = payloadMap
			}
		}
	}
}

// Subgraph signals the orchestrator to create and run a child workflow before
// this step completes. The step is parked until the child finishes - similar to
// how Interrupt pauses until Resume is called. When the child completes, its
// final state is filtered through the child's DeclareOutputs and merged into
// this step's changes using the parent graph's reducers, then the task is
// re-executed. On re-entry the task sees the child's output in its state and
// should return normally without calling Subgraph again.
//
// The child's initial state is built from the parent's full state plus the
// surgraph step's accumulated changes; the explicit input map is then merged on
// top using the child graph's reducers, and the result is filtered through the
// child's DeclareInputs.
func (f *Flow) Subgraph(workflowURL string, input map[string]any) {
	f.subgraphWorkflow = workflowURL
	f.subgraphInput = input
}

/*
Retry requests the orchestrator to retry this task with exponential backoff.
Returns true if a retry will be scheduled (attempts remaining), false if exhausted.
When true, the task should return nil. When false, the task should return its error.
The delay for attempt N is min(initialDelay * multiplier^N, maxDelay).

Example:

	result, err := callExternalAPI(ctx)
	if err != nil {
	    if flow.Retry(5, 1*time.Second, 2.0, 30*time.Second) {
	        return result, nil // retry scheduled, don't report error
	    }
	    return result, err // retries exhausted, report the error
	}
*/
func (f *Flow) Retry(maxAttempts int, initialDelay time.Duration, multiplier float64, maxDelay time.Duration) bool {
	if f.attempt >= maxAttempts {
		f.retry = false
		return false
	}
	f.retry = true
	f.backoffMaxAttempts = maxAttempts
	f.backoffInitialDelay = initialDelay
	f.backoffDelayMultiplier = multiplier
	f.backoffMaxDelay = maxDelay
	return true
}

// RetryNow signals the orchestrator to re-execute this task immediately with no limit.
// Equivalent to Retry(math.MaxInt32, 0, 0, 0).
func (f *Flow) RetryNow() bool {
	return f.Retry(math.MaxInt32, 0, 0, 0)
}

// Sleep tells the orchestrator to wait for the given duration before the next execution.
func (f *Flow) Sleep(duration time.Duration) {
	if duration >= 0 {
		f.sleepDuration = duration
	}
}

// --- Control signal inspection ---

// GotoRequested returns the task URL set by Goto, or empty if not set.
func (f *Flow) GotoRequested() string {
	return f.gotoNext
}

// RetryRequested returns the backoff parameters and true if Retry was called.
// The foreman uses these to compute the sleep delay and check the attempt limit.
func (f *Flow) RetryRequested() (maxAttempts int, initialDelay time.Duration, multiplier float64, maxDelay time.Duration, ok bool) {
	if !f.retry {
		return 0, 0, 0, 0, false
	}
	return f.backoffMaxAttempts, f.backoffInitialDelay, f.backoffDelayMultiplier, f.backoffMaxDelay, true
}

// SleepRequested returns the duration set by Sleep, or zero if not set.
func (f *Flow) SleepRequested() time.Duration {
	return max(f.sleepDuration, 0)
}

// InterruptRequested returns the interrupt payload and true if Interrupt was called.
func (f *Flow) InterruptRequested() (map[string]any, bool) {
	return f.interruptPayload, f.interrupt
}

// SubgraphRequested returns the workflow URL, input state, and true if Subgraph was called.
func (f *Flow) SubgraphRequested() (workflowURL string, input map[string]any, ok bool) {
	if f.subgraphWorkflow == "" {
		return "", nil, false
	}
	return f.subgraphWorkflow, f.subgraphInput, true
}

// --- Internal helpers ---

// applyFields marshals each field of source into the target map.
func (f *Flow) applyFields(source any, target map[string]any) error {
	v := reflect.ValueOf(source)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	t := v.Type()
	for i := range t.NumField() {
		field := t.Field(i)
		tag := jsonTagName(field)
		if tag == "" || tag == "-" {
			continue
		}
		data, err := json.Marshal(v.Field(i).Interface())
		if err != nil {
			return err
		}
		target[tag] = json.RawMessage(data)
	}
	return nil
}

// diffAndApply marshals each field of source, compares against the snapshot,
// and writes changed fields to both state and changes.
func (f *Flow) diffAndApply(source any, snapshot, state, changes map[string]any) error {
	v := reflect.ValueOf(source)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	t := v.Type()
	for i := range t.NumField() {
		field := t.Field(i)
		tag := jsonTagName(field)
		if tag == "" || tag == "-" {
			continue
		}
		data, err := json.Marshal(v.Field(i).Interface())
		if err != nil {
			return err
		}
		// Only record as change if different from snapshot
		if snapshot != nil {
			if prev, ok := snapshot[tag]; ok {
				prevData, _ := marshalAny(prev)
				if string(prevData) == string(data) {
					continue
				}
			}
		}
		raw := json.RawMessage(data)
		state[tag] = raw
		changes[tag] = raw
	}
	return nil
}

// --- JSON serialization ---

// flowJSON is the wire format for Flow.
type flowJSON struct {
	FlowKey                string         `json:"flowKey"`
	WorkflowName           string         `json:"workflowName"`
	TaskName               string         `json:"taskName"`
	StepNum                int            `json:"stepNum"`
	State                  map[string]any `json:"state,omitzero"`
	Changes                map[string]any `json:"changes,omitzero"`
	Goto                   string         `json:"goto,omitzero"`
	Retry                  bool           `json:"retry,omitzero"`
	SleepDuration          time.Duration  `json:"sleepDuration,omitzero"`
	Interrupt              bool           `json:"interrupt,omitzero"`
	InterruptPayload       map[string]any `json:"interruptPayload,omitzero"`
	SubgraphWorkflow       string         `json:"subgraphWorkflow,omitzero"`
	SubgraphInput          map[string]any `json:"subgraphInput,omitzero"`
	Attempt                int            `json:"attempt,omitzero"`
	BackoffMaxAttempts     int            `json:"backoffMaxAttempts,omitzero"`
	BackoffInitialDelay    time.Duration  `json:"backoffInitialDelay,omitzero"`
	BackoffDelayMultiplier float64        `json:"backoffDelayMultiplier,omitzero"`
	BackoffMaxDelay        time.Duration  `json:"backoffMaxDelay,omitzero"`
}

// MarshalJSON serializes the Flow including private fields.
func (f *Flow) MarshalJSON() ([]byte, error) {
	return json.Marshal(flowJSON{
		State:                  f.state,
		Changes:                f.changes,
		Goto:                   f.gotoNext,
		Retry:                  f.retry,
		SleepDuration:          f.sleepDuration,
		Interrupt:              f.interrupt,
		InterruptPayload:       f.interruptPayload,
		SubgraphWorkflow:       f.subgraphWorkflow,
		SubgraphInput:          f.subgraphInput,
		Attempt:                f.attempt,
		BackoffMaxAttempts:     f.backoffMaxAttempts,
		BackoffInitialDelay:    f.backoffInitialDelay,
		BackoffDelayMultiplier: f.backoffDelayMultiplier,
		BackoffMaxDelay:        f.backoffMaxDelay,
	})
}

// UnmarshalJSON deserializes the Flow including private fields.
func (f *Flow) UnmarshalJSON(data []byte) error {
	var wire flowJSON
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	f.state = wire.State
	f.changes = wire.Changes
	f.gotoNext = wire.Goto
	f.retry = wire.Retry
	f.sleepDuration = wire.SleepDuration
	f.interrupt = wire.Interrupt
	f.interruptPayload = wire.InterruptPayload
	f.subgraphWorkflow = wire.SubgraphWorkflow
	f.subgraphInput = wire.SubgraphInput
	f.attempt = wire.Attempt
	f.backoffMaxAttempts = wire.BackoffMaxAttempts
	f.backoffInitialDelay = wire.BackoffInitialDelay
	f.backoffDelayMultiplier = wire.BackoffDelayMultiplier
	f.backoffMaxDelay = wire.BackoffMaxDelay
	return nil
}

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
	"maps"
)

// RawFlow wraps Flow with additional methods used by the foreman orchestrator.
// Task endpoints should use Flow directly; RawFlow is for internal orchestration use only.
type RawFlow struct {
	Flow
}

// NewRawFlow creates a new RawFlow with initialized maps.
func NewRawFlow() *RawFlow {
	return &RawFlow{
		Flow: *NewFlow(),
	}
}

// --- Raw state access (for orchestrator use) ---

// RawState returns a copy of the raw state map.
func (f *RawFlow) RawState() map[string]any {
	result := make(map[string]any, len(f.state))
	maps.Copy(result, f.state)
	return result
}

// RawChanges returns a copy of the raw changes map.
func (f *RawFlow) RawChanges() map[string]any {
	result := make(map[string]any, len(f.changes))
	maps.Copy(result, f.changes)
	return result
}

// SetRawState replaces the entire state with the given raw map, without tracking changes.
func (f *RawFlow) SetRawState(state map[string]any) {
	f.state = make(map[string]any, len(state))
	maps.Copy(f.state, state)
}

// SetRawChanges replaces the entire changes map with the given raw map.
func (f *RawFlow) SetRawChanges(changes map[string]any) {
	f.changes = make(map[string]any, len(changes))
	maps.Copy(f.changes, changes)
}

// ClearChanges resets the changes map. Called by the orchestrator after persisting changes.
func (f *RawFlow) ClearChanges() {
	f.changes = make(map[string]any)
}

// ClearControl resets all control signals. Called by the orchestrator after processing them.
func (f *RawFlow) ClearControl() {
	f.gotoNext = ""
	f.retry = false
	f.sleepDuration = 0
	f.interrupt = false
	f.interruptPayload = nil
	f.attempt = 0
	f.backoffMaxAttempts = 0
	f.backoffInitialDelay = 0
	f.backoffDelayMultiplier = 0
	f.backoffMaxDelay = 0
}

// SetAttempt sets the attempt counter on the flow. Called by the orchestrator before dispatching
// a task so that Retry can check whether attempts are exhausted.
func (f *RawFlow) SetAttempt(attempt int) {
	f.attempt = attempt
}

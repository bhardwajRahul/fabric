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

package llmapi

import (
	"time"

	"github.com/microbus-io/errors"
)

// LastAssistantMessage returns the Content of the last assistant message item in items,
// or "" if there is none.
func LastAssistantMessage(items []Item) string {
	for i := len(items) - 1; i >= 0; i-- {
		if items[i].Message != nil && items[i].Message.Role == "assistant" {
			return items[i].Message.Content
		}
	}
	return ""
}

// PendingToolCalls returns the tool_call items that appear after the last message item, i.e. the calls
// the most recent assistant turn is still waiting on. Reasoning items are ignored.
func PendingToolCalls(items []Item) []ToolCall {
	var calls []ToolCall
	for i := len(items) - 1; i >= 0; i-- {
		if items[i].Message != nil {
			break // reached the assistant turn's text
		}
		if items[i].ToolCall != nil {
			calls = append(calls, *items[i].ToolCall)
		}
	}
	// Collected back-to-front; restore original order.
	for l, r := 0, len(calls)-1; l < r; l, r = l+1, r-1 {
		calls[l], calls[r] = calls[r], calls[l]
	}
	return calls
}

// RetryAfter reports whether err is a rate-limit error that can be retried after a delay, and if so the
// delay the provider asked to wait. It is for a workflow that drives its own retry around Chat: retryable
// true means the same call is worth repeating once wait has elapsed; false means the error is permanent
// and must not be retried. The signal is a retryAfter attribute on the error, not the HTTP status - the
// same 429 can also report a request the provider will never accept, so gating on the status alone would
// retry calls that cannot succeed. Retrying is at the caller's discretion: cap the attempts and set an
// overall give-up duration so a provider that stays throttled is not retried forever. A present but
// unparseable delay reports (0, true) - retryable, with a wait of your choosing. Pair this with the
// messages Chat returns on error to resume the conversation rather than restart it.
func RetryAfter(err error) (wait time.Duration, retryable bool) {
	ra, ok := errors.Convert(err).Properties["retryAfter"].(string)
	if !ok || ra == "" {
		return 0, false
	}
	d, parseErr := time.ParseDuration(ra)
	if parseErr != nil || d < 0 {
		return 0, true
	}
	return d, true
}

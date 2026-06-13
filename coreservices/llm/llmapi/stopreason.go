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

// Normalized stop reasons returned by Turn. Each provider maps its native
// finish_reason / stop_reason taxonomy into this set so callers branch on a
// single vocabulary. Anything a provider returns that does not map cleanly is
// reported as StopReasonUnknown so it surfaces as an error rather than as a
// silent end-of-turn.
const (
	StopReasonEndTurn      = "end_turn"      // Model finished naturally.
	StopReasonToolUse      = "tool_use"      // Model returned tool calls; the caller must execute them and reissue.
	StopReasonMaxTokens    = "max_tokens"    // Response was truncated at the token cap.
	StopReasonStopSequence = "stop_sequence" // Model hit a configured stop string.
	StopReasonRefusal      = "refusal"       // Model declined to respond (content policy).
	StopReasonPauseTurn    = "pause_turn"    // Provider-specific pause (e.g. Anthropic long-running tools).
	StopReasonUnknown      = ""              // Provider did not report a reason, or its value did not map.
)

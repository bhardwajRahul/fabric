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

package weirdapi

import "context"

// PlainAndAck is a convenience helper that wraps Plain. gencreds's
// helper-expansion path resolves a caller's PlainAndAck call to its
// underlying Plain endpoint when emitting downstream rules.
func (_c Client) PlainAndAck(ctx context.Context) (acked bool, err error) {
	result, err := _c.Plain(ctx)
	return result != "", err
}

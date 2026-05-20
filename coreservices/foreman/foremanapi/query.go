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

package foremanapi

import "time"

// Query specifies filtering and pagination options for listing or purging flows.
type Query struct {
	Status       string `json:"status,omitzero"`
	WorkflowName string `json:"workflowName,omitzero"`
	ThreadKey    string `json:"threadKey,omitzero"`
	// TaskName filters to flows whose current step (microbus_flows.step_id) is on the named task.
	// Joins microbus_steps; zero during fan-out (when step_id=0) so fanned-out flows are excluded.
	// Useful for "list flows blocked on tripped-breaker task X" operator queries.
	TaskName string `json:"taskName,omitzero"`
	// TenantID filters to flows belonging to a specific tenant. The tenant_id column is populated
	// from the caller's frame.Tenant() at Create time and inherited by subgraph/fork/continue.
	// Zero disables the filter (tenant 0 is the framework's "no tenant" sentinel and can be matched
	// by leaving the filter off; operators wanting only tenant=0 flows can grep client-side).
	TenantID int `json:"tenantID,omitzero"`
	// OlderThan filters to flows whose updated_at is older than this duration relative to the
	// database's NOW_UTC(). Zero disables the filter.
	OlderThan time.Duration `json:"olderThan,omitzero"`
	// NewerThan filters to flows whose updated_at is within this duration of the database's
	// NOW_UTC(). Zero disables the filter. Composes with OlderThan to express "between X and Y ago."
	NewerThan time.Duration `json:"newerThan,omitzero"`
	// Shard restricts the query to a single 1-based shard. Zero (the default) queries all shards.
	Shard int `json:"shard,omitzero"`
	// Cursor is the opaque pagination cursor returned as NextCursor by the previous List call.
	Cursor string `json:"cursor,omitzero"`
	Limit  int    `json:"limit,omitzero"`
}

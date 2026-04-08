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

import (
	"context"
	"encoding/json"

	"github.com/microbus-io/errors"
)

// RunAndParse creates a flow, starts it, blocks until it stops, then unmarshals the state into result.
func (_c Client) RunAndParse(ctx context.Context, workflowName string, initialState any, result any) (status string, err error) {
	status, state, err := _c.Run(ctx, workflowName, initialState)
	if err != nil {
		return status, errors.Trace(err)
	}
	if result != nil && state != nil {
		data, err := json.Marshal(state)
		if err != nil {
			return status, errors.Trace(err)
		}
		err = json.Unmarshal(data, result)
		if err != nil {
			return status, errors.Trace(err)
		}
	}
	return status, nil
}

// AwaitAndParse blocks until the flow stops, then unmarshals the state into result.
func (_c Client) AwaitAndParse(ctx context.Context, flowID string, result any) (status string, err error) {
	status, state, err := _c.Await(ctx, flowID)
	if err != nil {
		return status, errors.Trace(err)
	}
	if result != nil && state != nil {
		data, err := json.Marshal(state)
		if err != nil {
			return status, errors.Trace(err)
		}
		err = json.Unmarshal(data, result)
		if err != nil {
			return status, errors.Trace(err)
		}
	}
	return status, nil
}

// SnapshotAndParse returns the current status and unmarshals the state into result.
func (_c Client) SnapshotAndParse(ctx context.Context, flowID string, result any) (status string, err error) {
	status, state, err := _c.Snapshot(ctx, flowID)
	if err != nil {
		return status, errors.Trace(err)
	}
	if result != nil && state != nil {
		data, err := json.Marshal(state)
		if err != nil {
			return status, errors.Trace(err)
		}
		err = json.Unmarshal(data, result)
		if err != nil {
			return status, errors.Trace(err)
		}
	}
	return status, nil
}

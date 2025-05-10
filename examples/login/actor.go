/*
Copyright (c) 2023-2025 Microbus LLC and various contributors

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

package login

import (
	"bytes"
	"encoding/json"
	"slices"

	"github.com/microbus-io/fabric/utils"
)

// Actor represents the authenticated user.
// It is parsed from the claims associated with the request.
type Actor struct {
	Subject string   `json:"sub"`
	Roles   []string `json:"roles"`
}

// IsAdmin indicates if the actor can claim the admin role.
func (a Actor) IsAdmin() bool {
	return a.HasRole("a")
}

// SetAdmin claims the admin role for the actor.
func (a *Actor) SetAdmin() {
	a.Roles = append(a.Roles, "a")
}

// IsAdmin indicates if the can claim the manager role.
func (a Actor) IsManager() bool {
	return a.HasRole("m")
}

// SetManager claims the manager role for the actor.
func (a *Actor) SetManager() {
	a.Roles = append(a.Roles, "m")
}

// IsAdmin indicates if the actor can claim the user role.
func (a Actor) IsUser() bool {
	return a.HasRole("u")
}

// SetUser claims the user role for the actor.
func (a *Actor) SetUser() {
	a.Roles = append(a.Roles, "u")
}

// HasRole checks if the actor can claim the indicated role.
func (a Actor) HasRole(role string) bool {
	return slices.Contains(a.Roles, role)
}

// String prints the actor as JSON.
func (a Actor) String() string {
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(a)
	return utils.UnsafeBytesToString(buf.Bytes())
}

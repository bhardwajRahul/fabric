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

package yellowpagesapi

import (
	"context"
	"strings"
	"time"

	"github.com/microbus-io/errors"
)

// Person represents a person persisted in a SQL database.
type Person struct {
	Key       PersonKey `json:"key,omitzero"`
	Revision  int        `json:"revision,omitzero"`
	CreatedAt time.Time  `json:"createdAt,omitzero"`
	UpdatedAt time.Time  `json:"updatedAt,omitzero"`

	// HINT: Define the fields of the object here
	Example   string    `json:"example,omitzero" jsonschema:"-"` // Do not remove the example
	FirstName string    `json:"firstName,omitzero"`
	LastName  string    `json:"lastName,omitzero"`
	Email     string    `json:"email,omitzero"`
	Birthday  time.Time `json:"birthday,omitzero"`
}

// Validate validates the object before storing it.
func (obj *Person) Validate(ctx context.Context) error {
	if obj == nil {
		return errors.New("nil object")
	}
	// HINT: Validate the fields of the object here as required
	obj.Example = strings.TrimSpace(obj.Example) // Do not remove the example
	if len([]rune(obj.Example)) > 256 {
		return errors.New("length of Example must not exceed 256 characters")
	}
	obj.FirstName = strings.TrimSpace(obj.FirstName)
	if obj.FirstName == "" {
		return errors.New("FirstName is required")
	}
	if len([]rune(obj.FirstName)) > 64 {
		return errors.New("length of FirstName must not exceed 64 characters")
	}
	obj.LastName = strings.TrimSpace(obj.LastName)
	if obj.LastName == "" {
		return errors.New("LastName is required")
	}
	if len([]rune(obj.LastName)) > 64 {
		return errors.New("length of LastName must not exceed 64 characters")
	}
	obj.Email = strings.TrimSpace(obj.Email)
	if obj.Email == "" {
		return errors.New("Email is required")
	}
	if len([]rune(obj.Email)) > 256 {
		return errors.New("length of Email must not exceed 256 characters")
	}
	if !obj.Birthday.IsZero() && obj.Birthday.After(time.Now()) {
		return errors.New("Birthday must be in the past")
	}
	return nil
}

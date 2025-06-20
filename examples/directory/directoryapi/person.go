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

package directoryapi

import (
	"strings"
	"time"

	"github.com/microbus-io/fabric/errors"
)

// Person is a personal record that is registered in the directory.
// First and last name and email are required. Birthday is optional.
type Person struct {
	Birthday  time.Time `json:"birthday,omitzero"`
	Email     string    `json:"email,omitzero"`
	FirstName string    `json:"firstName,omitzero"`
	Key       PersonKey `json:"key,omitzero"`
	LastName  string    `json:"lastName,omitzero"`
}

// Validate validates the field of the person.
// First and last name and email are required. Optional birthday must be in the past.
func (person *Person) Validate() error {
	person.FirstName = strings.TrimSpace(person.FirstName)
	person.LastName = strings.TrimSpace(person.LastName)
	person.Email = strings.TrimSpace(person.Email)
	if person.FirstName == "" || person.LastName == "" {
		return errors.New("names cannot be empty")
	}
	if person.Email == "" {
		return errors.New("email cannot be empty")
	}
	if !person.Birthday.IsZero() && person.Birthday.After(time.Now()) {
		return errors.New("birthday must be a past date")
	}
	return nil
}

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

package cfg

import (
	"testing"

	"github.com/microbus-io/testarossa"
)

func TestCfg_Options(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	c, err := NewConfig(
		"int",
		Description("int config"),
		Validation("int [6,7]"),
		DefaultValue("7"),
		Secret(),
	)
	tt.NoError(err)
	tt.Equal(c.Name, "int")
	tt.Equal(c.Description, "int config")
	tt.Equal(c.Validation, "int [6,7]")
	tt.Equal(c.DefaultValue, "7")
	tt.Equal(c.Secret, true)
}

func TestCfg_BadValidation(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	_, err := NewConfig(
		"bad",
		Validation("invalid rule here"),
	)
	tt.Error(err)
}

func TestCfg_DefaultValueValidation(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	// Empty default values are tolerated
	_, err := NewConfig(
		"int",
		Validation("int [6,7]"),
	)
	tt.NoError(err)

	// Order of setting default value and validation shouldn't matter
	_, err = NewConfig(
		"int",
		DefaultValue("8"),
		Validation("int [6,7]"),
	)
	tt.Error(err)

	_, err = NewConfig(
		"int",
		Validation("int [6,7]"),
		DefaultValue("8"),
	)
	tt.Error(err)
}

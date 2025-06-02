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

package spec

import (
	"testing"

	"github.com/microbus-io/testarossa"
	"gopkg.in/yaml.v3"
)

func TestSpec_General(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	var gen General

	err := yaml.Unmarshal([]byte(`
host: super.service
description: foo
`), &gen)
	tt.NoError(err)

	err = yaml.Unmarshal([]byte(`
host: $uper.$ervice
description: foo
`), &gen)
	tt.Contains(err, "invalid host")

	err = yaml.Unmarshal([]byte(`
host:
description: foo
`), &gen)
	tt.Error(err, "invalid host")

	err = yaml.Unmarshal([]byte(`
description: foo
`), &gen)
	tt.Error(err, "invalid host")
}

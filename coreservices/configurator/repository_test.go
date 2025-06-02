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

package configurator

import (
	"testing"

	"github.com/microbus-io/testarossa"
)

func TestRepository_LoadYAML(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	y := `
# Comments should be ok
www.example.com:
  aaa: 111
  multiline: |-
    Line1
    Line2
example.com:
  aaa: xxx
  bbb: 222
  override: 2
com:
  CCC: 333
  override: 1
www.another.com:
  aaa: xxx
empty:
all:
  ddd: 444
  override: 0
`

	var r repository
	err := r.LoadYAML([]byte(y))
	tt.NoError(err)

	cases := map[string]string{
		"aaa":       "111",
		"bbb":       "222",
		"CCC":       "333",
		"ddd":       "444",
		"override":  "2",
		"multiline": "Line1\nLine2",
	}
	for name, expected := range cases {
		value, ok := r.Value("www.example.com", name)
		tt.True(ok)
		tt.Equal(expected, value)
	}

	cases = map[string]string{
		"aaa":      "xxx",
		"bbb":      "222",
		"CCC":      "333",
		"ddd":      "444",
		"override": "2",
	}
	for name, expected := range cases {
		value, ok := r.Value("EXAMPLE.com", name)
		tt.True(ok)
		tt.Equal(expected, value)
	}

	_, ok := r.Value("www.EXAMPLE.com", "foo")
	tt.False(ok)
	_, ok = r.Value("example.com", "multiLINE")
	tt.False(ok)
}

func TestRepository_Equals(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	var r repository
	err := r.LoadYAML([]byte(`
www.example.com:
  aaa: 111
example.com:
  bbb: 222
  bbbb: 2222
com:
  ccc: 333
all:
  ddd: 444
`))
	tt.NoError(err)

	var rr repository
	err = rr.LoadYAML([]byte(`
# Comment
example.com:
  bbbb: 2222
  bbb: 222
com:
  CCC: 333
all:
  ddd: 444
www.example.com:
  aaa: 111
`))
	tt.NoError(err)

	tt.True(r.Equals(&rr))
	tt.True(rr.Equals(&r))

	var rrr repository
	err = rrr.LoadYAML([]byte(`
example.com:
  b: 2
  bbb: 222
com:
  CCC: 333
all:
  ddd: 444
www.example.com:
  aaa: 111
`))
	tt.NoError(err)

	tt.False(r.Equals(&rrr))
	tt.False(rrr.Equals(&r))
}

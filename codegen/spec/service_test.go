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

func TestSpec_ErrorsInFunctions(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	var svc Service
	general := `
general:
  host: ok.host
`

	err := yaml.Unmarshal([]byte(general+`
functions:
  - signature: Func(s []*int)
    path: :BAD/...
`), &svc)
	tt.Contains(err, "invalid port")

	err = yaml.Unmarshal([]byte(general+`
functions:
  - signature: Func(s string)
    queue: skip
`), &svc)
	tt.Contains(err, "invalid queue")

	err = yaml.Unmarshal([]byte(general+`
functions:
  - signature: Func(s string)
    path: //bad.ho$t
`), &svc)
	tt.Contains(err, "invalid hostname")

	err = yaml.Unmarshal([]byte(general+`
functions:
  - signature: Func(s string)
    path: /backtick`+"`"+`
`), &svc)
	tt.Contains(err, "backtick not allowed")

	err = yaml.Unmarshal([]byte(general+`
functions:
  - signature: Func(s string)
    path: /backtick`+"`"+`
`), &svc)
	tt.Contains(err, "backtick not allowed")

	err = yaml.Unmarshal([]byte(general+`
functions:
  - signature: Func(s string)
    actor: roles=~`+"`admin`"+`
`), &svc)
	tt.Contains(err, "backtick not allowed")

	err = yaml.Unmarshal([]byte(general+`
functions:
  - signature: Func(s string)
    actor: x || (y
`), &svc)
	tt.Contains(err, "boolean expression")
}

func TestSpec_ErrorsInPathArguments(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	var svc Service
	general := `
general:
  host: ok.host
`

	err := yaml.Unmarshal([]byte(general+`
functions:
  - signature: Func(s string)
    path: /{ }
`), &svc)
	tt.Contains(err, "must be an identifier")

	err = yaml.Unmarshal([]byte(general+`
functions:
  - signature: Func(s string)
    path: /{p$}
`), &svc)
	tt.Contains(err, "must be an identifier")

	err = yaml.Unmarshal([]byte(general+`
functions:
  - signature: Func(s string)
    path: /{p +}
`), &svc)
	tt.Contains(err, "must be an identifier")

	err = yaml.Unmarshal([]byte(general+`
functions:
  - signature: Func(s string)
    path: /{ +}
`), &svc)
	tt.Contains(err, "must be an identifier")

	err = yaml.Unmarshal([]byte(general+`
functions:
  - signature: Func(s string)
    path: /{$+}
`), &svc)
	tt.Contains(err, "must be an identifier")

	err = yaml.Unmarshal([]byte(general+`
functions:
  - signature: Func(s string)
    path: /{+}/hello
`), &svc)
	tt.Contains(err, "must end path")
}

func TestSpec_ErrorsInEvents(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	var svc Service
	general := `
general:
  host: ok.host
`

	err := yaml.Unmarshal([]byte(general+`
events:
  - signature: Func(s []*int)
`), &svc)
	tt.Contains(err, "must start with 'On'")

	err = yaml.Unmarshal([]byte(general+`
events:
  - signature: OnFunc(s []*int)
    path: :BAD/...
`), &svc)
	tt.Contains(err, "invalid port")

	err = yaml.Unmarshal([]byte(general+`
events:
  - signature: OnFunc(s []*int)
    path: :0/...
`), &svc)
	tt.Contains(err, "invalid port")
}

func TestSpec_ErrorsInSinks(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	var svc Service
	general := `
general:
  host: ok.host
`

	err := yaml.Unmarshal([]byte(general+`
sinks:
  - signature: Func(s []*int)
    source: from/somewhere/else
`), &svc)
	tt.Contains(err, "must start with 'On'")

	err = yaml.Unmarshal([]byte(general+`
sinks:
  - signature: OnFunc(s []*int)
`), &svc)
	tt.Contains(err, "invalid source")

	err = yaml.Unmarshal([]byte(general+`
sinks:
  - signature: OnFunc(s []*int)
    source: https://www.example.com
`), &svc)
	tt.Contains(err, "invalid source")

	err = yaml.Unmarshal([]byte(general+`
sinks:
  - signature: OnFunc(s []*int)
    source: from/somewhere/else
    forHost: invalid.ho$t
`), &svc)
	tt.Contains(err, "invalid hostname")
}

func TestSpec_ErrorsInConfigs(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	var svc Service
	general := `
general:
  host: ok.host
`

	err := yaml.Unmarshal([]byte(general+`
configs:
  - signature: func() (b bool)
`), &svc)
	tt.Contains(err, "start with uppercase")

	err = yaml.Unmarshal([]byte(general+`
configs:
  - signature: Func()
`), &svc)
	tt.Contains(err, "single return value")

	err = yaml.Unmarshal([]byte(general+`
configs:
  - signature: Func() (x int, y int)
`), &svc)
	tt.Contains(err, "single return value")

	err = yaml.Unmarshal([]byte(general+`
configs:
  - signature: Func(x int) (b bool)
`), &svc)
	tt.Contains(err, "arguments not allowed")

	err = yaml.Unmarshal([]byte(general+`
configs:
  - signature: Func() (b byte)
`), &svc)
	tt.Contains(err, "invalid return type")

	err = yaml.Unmarshal([]byte(general+`
configs:
  - signature: Func() (b string)
    validation: xyz
`), &svc)
	tt.Contains(err, "invalid validation rule")

	err = yaml.Unmarshal([]byte(general+`
configs:
  - signature: Func() (b string)
    validation: str ^[a-z]+$
    default: 123
`), &svc)
	tt.Contains(err, "doesn't validate against rule")
}

func TestSpec_ErrorsInTickers(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	var svc Service
	general := `
general:
  host: ok.host
`

	err := yaml.Unmarshal([]byte(general+`
tickers:
  - signature: func()
`), &svc)
	tt.Contains(err, "start with uppercase")

	err = yaml.Unmarshal([]byte(general+`
tickers:
  - signature: Func(x int)
`), &svc)
	tt.Contains(err, "arguments or return values not allowed")

	err = yaml.Unmarshal([]byte(general+`
tickers:
  - signature: Func() (x int)
`), &svc)
	tt.Contains(err, "arguments or return values not allowed")

	err = yaml.Unmarshal([]byte(general+`
tickers:
  - signature: Func()
`), &svc)
	tt.Contains(err, "non-positive interval")

	err = yaml.Unmarshal([]byte(general+`
tickers:
  - signature: Func()
    interval: "-2m"
`), &svc)
	tt.Contains(err, "non-positive interval")
}

func TestSpec_ErrorsInWebs(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	var svc Service
	general := `
general:
  host: ok.host
`

	err := yaml.Unmarshal([]byte(general+`
webs:
  - signature: func()
`), &svc)
	tt.Contains(err, "start with uppercase")

	err = yaml.Unmarshal([]byte(general+`
webs:
  - signature: Func(x int)
`), &svc)
	tt.Contains(err, "arguments or return values not allowed")

	err = yaml.Unmarshal([]byte(general+`
webs:
  - signature: Func() (x int)
`), &svc)
	tt.Contains(err, "arguments or return values not allowed")

	err = yaml.Unmarshal([]byte(general+`
webs:
  - signature: Func()
    path: :BAD/...
`), &svc)
	tt.Contains(err, "invalid port")

	err = yaml.Unmarshal([]byte(general+`
webs:
  - signature: Func()
    queue: skip
`), &svc)
	tt.Contains(err, "invalid queue")
}

func TestSpec_ErrorsInService(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	var svc Service
	general := `
general:
  host: ok.host
`

	err := yaml.Unmarshal([]byte(general+`
functions:
  - signature: Func(x int) (y int)
webs:
  - signature: Func()
`), &svc)
	tt.Contains(err, "duplicate")

	err = yaml.Unmarshal([]byte(general+`
functions:
  - signature: Func(x int) (y int)
webs:
  - signature: FUNC()
`), &svc)
	tt.Contains(err, "duplicate")

	err = yaml.Unmarshal([]byte(general+`
functions:
  - signature: Func(x int) (y int)
configs:
  - signature: FUNC() (x int)
`), &svc)
	tt.Contains(err, "duplicate")
}

func TestSpec_HandlerInAndOut(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	code := `signature: Func(i integer, b boolean, s string, f  float64)  (m map[string]string, a []int)`
	var h Handler
	err := yaml.Unmarshal([]byte(code), &h)
	tt.NoError(err)
	tt.Equal(h.In("name type"), "i int, b bool, s string, f float64")
	tt.Equal(h.Out("name type"), "m map[string]string, a []int")
}

func TestSpec_QualifyTypes(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	code := `
general:
  host: example.com
functions:
  - signature: Func(d Defined) (i Imported)
`
	var svc Service
	err := yaml.Unmarshal([]byte(code), &svc)
	tt.NoError(err)
	svc.PackagePath = "from/my"

	tt.Equal("Defined", svc.Functions[0].Signature.InputArgs[0].Type)
	tt.Equal("Imported", svc.Functions[0].Signature.OutputArgs[0].Type)

	svc.FullyQualifyTypes()

	tt.Equal("myapi.Defined", svc.Functions[0].Signature.InputArgs[0].Type)
	tt.Equal("myapi.Imported", svc.Functions[0].Signature.OutputArgs[0].Type)

	svc.ShorthandTypes()

	tt.Equal("Defined", svc.Functions[0].Signature.InputArgs[0].Type)
	tt.Equal("Imported", svc.Functions[0].Signature.OutputArgs[0].Type)
}

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

func TestSpec_Signature(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	var sig Signature

	err := yaml.Unmarshal([]byte("Hello(x int, y string) (ok bool)"), &sig)
	tt.NoError(err)
	tt.Equal("Hello", sig.Name)
	tt.Len(sig.InputArgs, 2)
	tt.Equal("x", sig.InputArgs[0].Name)
	tt.Equal("int", sig.InputArgs[0].Type)
	tt.Equal("y", sig.InputArgs[1].Name)
	tt.Equal("string", sig.InputArgs[1].Type)
	tt.Len(sig.OutputArgs, 1)
	tt.Equal("ok", sig.OutputArgs[0].Name)
	tt.Equal("bool", sig.OutputArgs[0].Type)

	err = yaml.Unmarshal([]byte("Hello(x int)"), &sig)
	tt.NoError(err)
	tt.Equal("Hello", sig.Name)
	tt.Len(sig.InputArgs, 1)
	tt.Equal("x", sig.InputArgs[0].Name)
	tt.Equal("int", sig.InputArgs[0].Type)
	tt.Len(sig.OutputArgs, 0)

	err = yaml.Unmarshal([]byte("Hello() (e string, ok bool)"), &sig)
	tt.NoError(err)
	tt.Equal("Hello", sig.Name)
	tt.Len(sig.InputArgs, 0)
	tt.Len(sig.OutputArgs, 2)
	tt.Equal("e", sig.OutputArgs[0].Name)
	tt.Equal("string", sig.OutputArgs[0].Type)
	tt.Equal("ok", sig.OutputArgs[1].Name)
	tt.Equal("bool", sig.OutputArgs[1].Type)

	err = yaml.Unmarshal([]byte("Hello()"), &sig)
	tt.NoError(err)
	tt.Equal("Hello", sig.Name)
	tt.Len(sig.InputArgs, 0)
	tt.Len(sig.OutputArgs, 0)

	err = yaml.Unmarshal([]byte("Hello"), &sig)
	tt.NoError(err)
	tt.Equal("Hello", sig.Name)
	tt.Len(sig.InputArgs, 0)
	tt.Len(sig.OutputArgs, 0)

	err = yaml.Unmarshal([]byte("MockMe"), &sig)
	tt.Error(err)

	err = yaml.Unmarshal([]byte("TestMe"), &sig)
	tt.Error(err)
}

func TestSpec_HTTPArguments(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	var sig Signature

	err := yaml.Unmarshal([]byte("Hello(x int, y string) (ok bool)"), &sig)
	tt.NoError(err)

	err = yaml.Unmarshal([]byte("Hello(x int, httpResponseBody string) (ok bool)"), &sig)
	tt.Error(err, "httpResponseBody can't be an input argument")
	err = yaml.Unmarshal([]byte("Hello(x int, httpStatusCode string) (ok bool)"), &sig)
	tt.Error(err, "httpStatusCode can't be an input argument")
	err = yaml.Unmarshal([]byte("Hello(x int, y string) (httpRequestBody bool)"), &sig)
	tt.Error(err, "httpRequestBody can't be an output argument")

	err = yaml.Unmarshal([]byte("Hello(x int, y string) (httpResponseBody bool)"), &sig)
	tt.NoError(err)
	err = yaml.Unmarshal([]byte("Hello(x int, y string) (httpResponseBody bool, httpStatusCode int)"), &sig)
	tt.NoError(err)

	err = yaml.Unmarshal([]byte("Hello(x int, y string) (httpResponseBody bool, httpStatusCode bool)"), &sig)
	tt.Error(err, "httpStatusCode must be of type int")
	err = yaml.Unmarshal([]byte("Hello(x int, y string) (httpResponseBody bool, z int, httpStatusCode int)"), &sig)
	tt.Error(err, "Output argument not allowed alongside httpResponseBody")
}

func TestSpec_TypedHandlers(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	var sig Signature

	err := yaml.Unmarshal([]byte("OnFunc(ctx context.Context) (result int)"), &sig)
	tt.Error(err, "Context type not allowed")
	err = yaml.Unmarshal([]byte("OnFunc(x int) (result int, err error)"), &sig)
	tt.Error(err, "Error type not allowed")
	err = yaml.Unmarshal([]byte("onFunc(x int) (y int)"), &sig)
	tt.Error(err, "Endpoint name must start with uppercase")
	err = yaml.Unmarshal([]byte("OnFunc(X int) (y int)"), &sig)
	tt.Error(err, "Arg name must start with lowercase")
	err = yaml.Unmarshal([]byte("OnFunc(x int) (Y int)"), &sig)
	tt.Error(err, "Arg name must start with lowercase")
	err = yaml.Unmarshal([]byte("OnFunc(x os.File) (y int)"), &sig)
	tt.Error(err, "Dot notation not allowed")
	err = yaml.Unmarshal([]byte("OnFunc(x Time) (x Duration)"), &sig)
	tt.Error(err, "Duplicate arg name")
	err = yaml.Unmarshal([]byte("OnFunc(b boolean, x uint64, x int) (y int)"), &sig)
	tt.Error(err, "Duplicate arg name")
	err = yaml.Unmarshal([]byte("OnFunc(x map[string]string) (y int, b bool, y int)"), &sig)
	tt.Error(err, "Duplicate arg name")
	err = yaml.Unmarshal([]byte("OnFunc(m map[int]int)"), &sig)
	tt.Error(err, "Map keys must ne strings")
	err = yaml.Unmarshal([]byte("OnFunc(m mutex)"), &sig)
	tt.Error(err, "Primitive type")
	err = yaml.Unmarshal([]byte("OnFunc(m int"), &sig)
	tt.Error(err, "Missing closing parenthesis")
	err = yaml.Unmarshal([]byte("OnFunc(m int) (x int"), &sig)
	tt.Error(err, "Missing closing parenthesis")
	err = yaml.Unmarshal([]byte("OnFunc(mint) (x int)"), &sig)
	tt.Error(err, "Missing argument type")
	err = yaml.Unmarshal([]byte("OnFunc(m int) (xint)"), &sig)
	tt.Error(err, "Missing argument type")
}

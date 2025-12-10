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
	assert := testarossa.For(t)

	var sig Signature

	err := yaml.Unmarshal([]byte("Hello(x int, y string) (ok bool)"), &sig)
	assert.NoError(err)
	assert.Equal("Hello", sig.Name)
	assert.Len(sig.InputArgs, 2)
	assert.Equal("x", sig.InputArgs[0].Name)
	assert.Equal("int", sig.InputArgs[0].Type)
	assert.Equal("y", sig.InputArgs[1].Name)
	assert.Equal("string", sig.InputArgs[1].Type)
	assert.Len(sig.OutputArgs, 1)
	assert.Equal("ok", sig.OutputArgs[0].Name)
	assert.Equal("bool", sig.OutputArgs[0].Type)

	err = yaml.Unmarshal([]byte("Hello(x int)"), &sig)
	assert.NoError(err)
	assert.Equal("Hello", sig.Name)
	assert.Len(sig.InputArgs, 1)
	assert.Equal("x", sig.InputArgs[0].Name)
	assert.Equal("int", sig.InputArgs[0].Type)
	assert.Len(sig.OutputArgs, 0)

	err = yaml.Unmarshal([]byte("Hello() (e string, ok bool)"), &sig)
	assert.NoError(err)
	assert.Equal("Hello", sig.Name)
	assert.Len(sig.InputArgs, 0)
	assert.Len(sig.OutputArgs, 2)
	assert.Equal("e", sig.OutputArgs[0].Name)
	assert.Equal("string", sig.OutputArgs[0].Type)
	assert.Equal("ok", sig.OutputArgs[1].Name)
	assert.Equal("bool", sig.OutputArgs[1].Type)

	err = yaml.Unmarshal([]byte("Hello()"), &sig)
	assert.NoError(err)
	assert.Equal("Hello", sig.Name)
	assert.Len(sig.InputArgs, 0)
	assert.Len(sig.OutputArgs, 0)

	err = yaml.Unmarshal([]byte("Hello"), &sig)
	assert.NoError(err)
	assert.Equal("Hello", sig.Name)
	assert.Len(sig.InputArgs, 0)
	assert.Len(sig.OutputArgs, 0)

	err = yaml.Unmarshal([]byte("MockMe"), &sig)
	assert.Error(err)

	err = yaml.Unmarshal([]byte("TestMe"), &sig)
	assert.Error(err)
}

func TestSpec_HTTPArguments(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	var sig Signature

	err := yaml.Unmarshal([]byte("Hello(x int, y string) (ok bool)"), &sig)
	assert.NoError(err)

	err = yaml.Unmarshal([]byte("Hello(x int, httpResponseBody string) (ok bool)"), &sig)
	assert.Contains(err, "can only be an output argument")
	err = yaml.Unmarshal([]byte("Hello(x int, httpStatusCode string) (ok bool)"), &sig)
	assert.Contains(err, "can only be an output argument")
	err = yaml.Unmarshal([]byte("Hello(x int, y string) (httpRequestBody bool)"), &sig)
	assert.Contains(err, "can only be an input argument")

	err = yaml.Unmarshal([]byte("Hello(x int, y string) (httpResponseBody bool)"), &sig)
	assert.NoError(err)
	err = yaml.Unmarshal([]byte("Hello(x int, y string) (httpResponseBody bool, httpStatusCode int)"), &sig)
	assert.NoError(err)

	err = yaml.Unmarshal([]byte("Hello(x int, y string) (httpResponseBody bool, httpStatusCode bool)"), &sig)
	assert.Contains(err, "must be of type int")
	err = yaml.Unmarshal([]byte("Hello(x int, y string) (httpResponseBody bool, z int, httpStatusCode int)"), &sig)
	assert.Contains(err, "cannot return other arguments")
}

func TestSpec_TypedHandlers(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	var sig Signature

	err := yaml.Unmarshal([]byte("OnFunc(ctx context.Context) (result int)"), &sig)
	assert.Contains(err, "context type not allowed")
	err = yaml.Unmarshal([]byte("OnFunc(x int) (result int, err error)"), &sig)
	assert.Contains(err, "error type not allowed")
	err = yaml.Unmarshal([]byte("onFunc(x int) (y int)"), &sig)
	assert.Contains(err, "must start with uppercase")
	err = yaml.Unmarshal([]byte("OnFunc(X int) (y int)"), &sig)
	assert.Contains(err, "must start with lowercase")
	err = yaml.Unmarshal([]byte("OnFunc(x int) (Y int)"), &sig)
	assert.Contains(err, "must start with lowercase")
	err = yaml.Unmarshal([]byte("OnFunc(x os.File) (y int)"), &sig)
	assert.Contains(err, "dot notation not allowed")
	err = yaml.Unmarshal([]byte("OnFunc(x Time) (x Duration)"), &sig)
	assert.Contains(err, "duplicate arg name")
	err = yaml.Unmarshal([]byte("OnFunc(b boolean, x uint64, x int) (y int)"), &sig)
	assert.Contains(err, "duplicate arg name")
	err = yaml.Unmarshal([]byte("OnFunc(x map[string]string) (y int, b bool, y int)"), &sig)
	assert.Contains(err, "duplicate arg name")
	err = yaml.Unmarshal([]byte("OnFunc(m map[int]int)"), &sig)
	assert.Contains(err, "map keys must be strings")
	err = yaml.Unmarshal([]byte("OnFunc(m mutex)"), &sig)
	assert.Contains(err, "primitive type")
	err = yaml.Unmarshal([]byte("OnFunc(m int"), &sig)
	assert.Contains(err, "missing closing parenthesis")
	err = yaml.Unmarshal([]byte("OnFunc(m int) (x int"), &sig)
	assert.Contains(err, "missing closing parenthesis")
	err = yaml.Unmarshal([]byte("OnFunc(mint) (x int)"), &sig)
	assert.Contains(err, "invalid argument")
	err = yaml.Unmarshal([]byte("OnFunc(m int) (xint)"), &sig)
	assert.Contains(err, "invalid argument")
}

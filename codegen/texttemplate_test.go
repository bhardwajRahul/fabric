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

package main

import (
	"os"
	"testing"

	"github.com/microbus-io/fabric/codegen/spec"
	"github.com/microbus-io/fabric/rand"
	"github.com/microbus-io/testarossa"
)

func TestCodegen_CapitalizeIdentifier(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	testCases := map[string]string{
		"fooBar":     "FooBar",
		"fooBAR":     "FooBAR",
		"urlEncoder": "URLEncoder",
		"URLEncoder": "URLEncoder",
		"a":          "A",
		"A":          "A",
		"":           "",
		"id":         "ID",
		"xId":        "XId",
	}
	for id, expected := range testCases {
		tt.Equal(expected, capitalizeIdentifier(id))
	}
}

func TestCodegen_TextTemplate(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	_, err := LoadTemplate("service/doesn't.exist")
	tt.Error(err)

	tmpl, err := LoadTemplate("service/service.txt")
	tt.NoError(err)

	var x struct{}
	_, err = tmpl.Execute(&x)
	tt.Error(err)

	specs := &spec.Service{
		PackagePath: "testing/text/template",
		General: spec.General{
			Host:        "example.com",
			Description: "Example",
		},
	}
	rendered, err := tmpl.Execute(specs)
	n := len(rendered)
	tt.NoError(err)
	tt.Contains(string(rendered), specs.PackagePathSuffix())
	tt.Contains(string(rendered), specs.General.Host)

	fileName := "testing-" + rand.AlphaNum32(12)
	defer os.Remove(fileName)

	err = tmpl.AppendTo(fileName, specs)
	tt.NoError(err)
	onDisk, err := os.ReadFile(fileName)
	tt.NoError(err)
	tt.Equal(rendered, onDisk)

	err = tmpl.AppendTo(fileName, specs)
	tt.NoError(err)
	onDisk, err = os.ReadFile(fileName)
	tt.NoError(err)
	tt.Len(onDisk, n*2)

	err = tmpl.Overwrite(fileName, specs)
	tt.NoError(err)
	onDisk, err = os.ReadFile(fileName)
	tt.NoError(err)
	tt.Equal(rendered, onDisk)
}

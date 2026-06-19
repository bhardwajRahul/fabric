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

// Package bundlemain is a fixture main.go for gencreds's bundle parser tests.
// It composes the kitchen+weird testdata fixtures into an application via
// app.Add, exercising the bare and Init-wrapped argument shapes.
package main

import (
	"github.com/microbus-io/fabric/application"
	"github.com/microbus-io/fabric/cmd/gencreds/testdata/kitchen"
	"github.com/microbus-io/fabric/cmd/gencreds/testdata/weird"
)

func main() {
	app := application.New()
	app.Add(
		kitchen.NewService(),
		weird.NewService(),
	)
	_ = app
}

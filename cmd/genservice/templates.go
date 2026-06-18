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

package main

import (
	"embed"
	"strconv"
	"text/template"
)

// templatesFS holds the client.go text/templates as Go source. The static pieces (e.g. client.txt,
// the marshal* helpers) carry no actions; the per-feature pieces (function.txt, task.txt, ...) and
// the root (root.txt) use {{.Field}} placeholders, {{if}} on the feature flags, and {{range}} over
// the feature views.
//
//go:embed templates/*.txt
var templatesFS embed.FS

// clientTemplate is the parsed template set; each file is referenced by its base name (e.g.
// {{template "function.txt" .}}). root.txt is the entry point.
var clientTemplate = template.Must(template.New("genservice").Funcs(template.FuncMap{
	"quote": strconv.Quote,
	"snake": snakeCase,
}).ParseFS(templatesFS, "templates/*.txt"))

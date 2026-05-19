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

// genopenapispecs is a filter: it reads an OpenAPI document (JSON or YAML) on
// stdin and writes the normalized specs JSON on stdout. The specs file is the
// intermediate representation; this tool owns parsing the OpenAPI document
// deterministically, while the feature-scaffolding skills own the shape of the
// generated code. It captures only what those skills need - functions, webs,
// type definitions, the remote API base URL, and the authentication scheme.
// The microservice's hostname is decided when it is scaffolded, not here.
//
// The tool never accesses the network. Fetching a document from a URL is the
// caller's responsibility (the importer skill has the agent do it); the tool
// only transforms what arrives on stdin.
//
// Usage:
//
//	genopenapispecs [-base-url URL] < openapi.json > openapispecs.json
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
)

func main() {
	baseURL := flag.String("base-url", "", "remote API base URL (default from the document's servers)")
	flag.Parse()

	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, "genopenapispecs:", err)
		os.Exit(1)
	}

	err = run(raw, *baseURL, os.Stdout, os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "genopenapispecs:", err)
		os.Exit(1)
	}
}

// run parses the OpenAPI document in raw, applies the base-URL override, writes
// the resulting specs JSON to stdout, and warns on stderr when the resolved
// remote API base URL is not absolute.
func run(raw []byte, baseURL string, stdout, stderr io.Writer) error {
	doc, err := parseDocument(raw)
	if err != nil {
		return err
	}

	specs, err := buildSpecs(doc, baseURL)
	if err != nil {
		return err
	}

	if !isAbsoluteURL(specs.Remote.BaseURL) {
		fmt.Fprintf(stderr, "genopenapispecs: warning: remote API base URL %q is not absolute; "+
			"pass -base-url to set the real API origin\n", specs.Remote.BaseURL)
	}

	encoded, err := specs.encode()
	if err != nil {
		return err
	}
	_, err = stdout.Write(encoded)
	return err
}

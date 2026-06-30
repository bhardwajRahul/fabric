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

// geninit scaffolds the boilerplate of a new Microbus project: the main package,
// config and env files, the agent and editor files, and a freshly generated
// Ed25519 token-signing key. It is idempotent - existing files are left intact -
// so it is safe to re-run on an established project. The init-project skill invokes it.
package main

import (
	"embed"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed templates/*.txt
var templatesFS embed.FS

func main() {
	dir := flag.String("dir", ".", "project root directory to scaffold")
	flag.Parse()

	root, err := filepath.Abs(*dir)
	if err != nil {
		fail(err)
	}
	g := &generator{root: root}
	err = g.run()
	if err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "geninit: %v\n", err)
	os.Exit(1)
}

// template returns the named embedded template as a string.
func template(name string) string {
	b, err := templatesFS.ReadFile("templates/" + name)
	if err != nil {
		panic(err)
	}
	return string(b)
}

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

// Command genservice generates a microservice's boilerplate from its api package definition.go.
// This first increment generates *api/client.go; intermediate.go, mock.go, and manifest.yaml follow.
package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: genservice <api-package-dir> [<api-package-dir>...]")
		os.Exit(2)
	}
	for _, dir := range os.Args[1:] {
		svc, err := parseService(dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "genservice: %s: %v\n", dir, err)
			os.Exit(1)
		}
		path := filepath.Join(dir, "client.go")
		src, err := emitClient(svc, existingHeader(path))
		if err != nil {
			fmt.Fprintf(os.Stderr, "genservice: %s: %v\n", dir, err)
			os.Exit(1)
		}
		err = os.WriteFile(path, src, 0o644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "genservice: %s: %v\n", dir, err)
			os.Exit(1)
		}
		fmt.Printf("genservice: wrote %s\n", path)
	}
}

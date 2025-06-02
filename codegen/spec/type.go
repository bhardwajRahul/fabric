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

import "strings"

// Type is a complex type used in a function.
type Type struct {
	Name        string
	Exists      bool
	PackagePath string
}

// PackagePathSuffix returns only the last portion of the full package path.
func (t *Type) PackagePathSuffix() string {
	p := strings.LastIndex(t.PackagePath, "/")
	if p < 0 {
		return t.PackagePath
	}
	return t.PackagePath[p+1:]
}

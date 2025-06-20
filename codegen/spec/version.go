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
	"path/filepath"
)

// Version keeps the versioning information of the code.
type Version struct {
	PackagePath string
	Version     int    `json:"ver"`
	SHA256      string `json:"sha256"`
	Timestamp   string `json:"ts"`
}

// PackagePathSuffix returns only the last portion of the full package path.
func (v *Version) PackagePathSuffix() string {
	return filepath.Base(v.PackagePath)
}

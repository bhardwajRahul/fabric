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

package utils

import (
	"github.com/microbus-io/codegen/versionhash"
)

// SourceCodeSHA256 generates a SHA256 of the source code files in the indicated directory and its sub-directories.
// The directory is interpreted relative to the current working directory.
func SourceCodeSHA256(directory string) (string, error) {
	return versionhash.SHA256(directory)
}

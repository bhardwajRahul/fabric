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

package browserapi

import (
	"github.com/microbus-io/fabric/define"
)

// Hostname is the default hostname of the microservice.
const Hostname = "browser.example"

// Name is the decorative PascalCase name of the microservice.
const Name = "Browser"

// Version is the major version of the microservice's public API.
const Version = 135

// Description is the human-readable summary of the microservice, surfaced in OpenAPI and discovery.
const Description = `The browser microservice implements a simple web browser that utilizes the egress proxy.`

// Browse shows a simple address bar and the source code of a URL.
var Browse = define.Web{
	Host: Hostname, Method: "ANY", Route: "/browse",
}

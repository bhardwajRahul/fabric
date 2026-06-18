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

package metricsapi

import (
	"github.com/microbus-io/fabric/define"
)

// Hostname is the default hostname of the microservice.
const Hostname = "metrics.core"

// Name is the decorative PascalCase name of the microservice.
const Name = "Metrics"

// Version is the major version of the microservice's public API.
const Version = 215

// Description is the human-readable summary of the microservice, surfaced in OpenAPI and discovery.
const Description = `The Metrics service is a core microservice that aggregates metrics from other microservices and makes them available for collection.`

// SecretKey must be provided with the request to collect the metrics.
// This key is required except in local development and tests.
var SecretKey = define.Config{
	Value:  string(""),
	Secret: true,
}

// Collect returns the latest aggregated metrics.
var Collect = define.Web{
	Host: Hostname, Method: "GET", Route: "/collect",
}

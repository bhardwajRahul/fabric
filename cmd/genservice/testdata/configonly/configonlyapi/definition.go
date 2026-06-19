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

// Package configonlyapi is a fixture for an endpoint-less microservice: it declares only configs and a
// metric, no functions/webs/tasks/workflows/events. It pins the generated intermediate.go and
// mock_test.go for a microservice with no Subscribe surface, so the http/httpx/sub imports (intermediate)
// and the context import (mock_test) are emitted from the feature mix rather than leaking in via the api
// client.go. The duration-valued metric is the sole reason time is imported, pinning that a metric value
// type's package is resolved into the recorder's imports.
package configonlyapi

import (
	"time"

	"github.com/microbus-io/fabric/define"
)

// Hostname is the default hostname of the microservice.
const Hostname = "configonly.example"

// Name is the decorative PascalCase name of the microservice.
const Name = "ConfigOnly"

// Version is a generation counter bumped on each regeneration, not a semantic version.
const Version = 1

// Description is the human-readable summary of the microservice, surfaced in OpenAPI and discovery.
const Description = `The configonly microservice is a fixture exercising an endpoint-less (config-only) service.`

// Threshold is a plain integer config.
var Threshold = define.Config{
	Value:      int(0),
	Default:    "10",
	Validation: "int [0,100]",
}

// DenyList is a multi-line config default; changes fire OnChangedDenyList.
var DenyList = define.Config{
	Value: string(""),
	Default: `/admin
/.git`,
	Callback: true,
}

// ReconcileDuration records how long a reconciliation took, as a duration. Its time.Duration value type
// is the only thing in this fixture that requires the time import in the generated recorder.
var ReconcileDuration = define.Metric{
	Kind: define.Histogram, Value: time.Duration(0),
	Buckets:  []float64{0.1, 0.5, 1, 5},
	OTelName: "configonly_reconcile_duration",
}

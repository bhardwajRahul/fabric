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

package messagingapi

import (
	"github.com/microbus-io/fabric/define"
)

// Hostname is the default hostname of the microservice.
const Hostname = "messaging.example"

// Name is the decorative PascalCase name of the microservice.
const Name = "Messaging"

// Version is the major version of the microservice's public API.
const Version = 230

// Description is the human-readable summary of the microservice, surfaced in OpenAPI and discovery.
const Description = `The Messaging microservice demonstrates service-to-service communication patterns.`

// Home demonstrates making requests using multicast and unicast request/response patterns.
var Home = define.Web{
	Host: Hostname, Method: "GET", Route: "/home",
}

// NoQueue demonstrates how the NoQueue subscription option is used to create
// a multicast request/response communication pattern.
// All instances of this microservice will respond to each request.
var NoQueue = define.Web{
	Host: Hostname, Method: "GET", Route: "/no-queue",
	LoadBalancing: define.None,
}

// DefaultQueue demonstrates how the DefaultQueue subscription option is used to create
// a unicast request/response communication pattern.
// Only one of the instances of this microservice will respond to each request.
var DefaultQueue = define.Web{
	Host: Hostname, Method: "GET", Route: "/default-queue",
}

// CacheLoad looks up an element in the distributed cache of the microservice.
var CacheLoad = define.Web{
	Host: Hostname, Method: "GET", Route: "/cache-load",
}

// CacheStore stores an element in the distributed cache of the microservice.
var CacheStore = define.Web{
	Host: Hostname, Method: "GET", Route: "/cache-store",
}

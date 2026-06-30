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

package loginapi

import (
	"github.com/microbus-io/fabric/define"
)

// HINT: This file is the single source of truth for the microservice's API. After editing it, run
// cmd/genservice on the microservice's directory (the parent of this api package) to regenerate client.go,
// intermediate.go, mock.go, mock_test.go, and manifest.yaml. Do not hand-edit those generated files.

// Hostname is the default hostname of the microservice.
const Hostname = "login.example"

// Name is the decorative PascalCase name of the microservice.
const Name = "Login"

// Version is a generation counter bumped on each regeneration, not a semantic version.
const Version = 93

// Description is the human-readable summary of the microservice, surfaced in OpenAPI and discovery.
const Description = `The Login microservice demonstrates usage of authentication and authorization.`

// Login renders a simple login screen that authenticates a user.
// Known users are hardcoded as "admin", "manager" and "user".
// The password is "password".
var Login = define.Web{ // MARKER: Login
	Host: Hostname, Method: "ANY", Route: "/login",
}

// Logout renders a page that logs out the user.
var Logout = define.Web{ // MARKER: Logout
	Host: Hostname, Method: "ANY", Route: "/logout",
}

// Welcome renders a page that is shown to the user after a successful login.
// Rendering is adjusted based on the user's roles.
var Welcome = define.Web{ // MARKER: Welcome
	Host: Hostname, Method: "ANY", Route: "/welcome",
	RequiredClaims: "roles.a || roles.m || roles.u",
}

// AdminOnly is only accessible by admins.
var AdminOnly = define.Web{ // MARKER: AdminOnly
	Host: Hostname, Method: "GET", Route: "/admin-only",
	RequiredClaims: "roles.a",
}

// ManagerOnly is only accessible by managers.
var ManagerOnly = define.Web{ // MARKER: ManagerOnly
	Host: Hostname, Method: "GET", Route: "/manager-only",
	RequiredClaims: "roles.m",
}

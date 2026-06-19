package myserviceapi

import "github.com/microbus-io/fabric/define"

var _ = define.None

// HINT: This file is the single source of truth for the microservice's API. After editing it, run
// cmd/genservice on the microservice's directory (the parent of this api package) to regenerate client.go,
// intermediate.go, mock.go, mock_test.go, and manifest.yaml. Do not hand-edit those generated files.

// Hostname is the default hostname of the microservice.
const Hostname = "myservice.myproject.mycompany"

// Name is the decorative PascalCase name of the microservice.
const Name = "MyService"

// Version is the major version of the microservice's public API.
const Version = 1

// Description is the human-readable summary of the microservice, surfaced in OpenAPI and discovery.
const Description = `MyService does X.`

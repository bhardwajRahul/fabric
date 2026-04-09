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

package sub

import (
	"regexp"

	"github.com/microbus-io/boolexp"
	"github.com/microbus-io/errors"
)

// Option is used to construct a subscription in Connector.Subscribe
type Option func(sub *Subscription) error

// Queue names the queue of the subscription.
// Requests will be load-balanced among all consumers with the same queue name
func Queue(queue string) Option {
	return func(sub *Subscription) error {
		match, err := regexp.MatchString(`^[a-zA-Z0-9\.]+$`, queue)
		if err != nil {
			return errors.Trace(err)
		}
		if !match {
			return errors.New("invalid queue name '%s'", queue)
		}
		sub.Queue = queue
		return nil
	}
}

// NoQueue sets no queue for this subscription.
// Requests will be not be load-balanced, all instances of this microservice will receive the request
func NoQueue() Option {
	return func(sub *Subscription) error {
		sub.Queue = ""
		return nil
	}
}

// DefaultQueue names the queue of this subscription to the hostname of the service.
// Requests will be load-balanced among all instances of this microservice.
func DefaultQueue() Option {
	return func(sub *Subscription) error {
		sub.Queue = sub.Host
		return nil
	}
}

// RequiredClaims requires that the properties of the actor associated with the request satisfy the boolean expression.
// For example: iss=='my_issuer' && (roles.admin || roles.manager) && region=="US".
// The =~ and !~ operators evaluate the left operand against a regexp.
// String constants, including regexp patterns, must be quoted using single quotes, double quotes or backticks.
// A request that doesn't satisfy the constraint is denied with a 403 forbidden error.
func RequiredClaims(boolExp string) Option {
	return func(sub *Subscription) error {
		_, err := boolexp.Eval(boolExp, nil)
		if err != nil {
			return errors.Trace(err)
		}
		sub.RequiredClaims = boolExp
		return nil
	}
}

// Description sets a human-readable description for the subscription.
// Used by the built-in OpenAPI handler.
func Description(text string) Option {
	return func(sub *Subscription) error {
		sub.Description = text
		return nil
	}
}

// Method overrides the default "ANY" method for a Listen subscription.
// The subscription can be set to a single standard HTTP method such as "GET", "POST", etc. or to "ANY" in order to accept any method.
func Method(method string) Option {
	return func(sub *Subscription) error {
		sub.Method = method
		return nil
	}
}

/*
Route overrides the default ":443/my-subscription" route for a Listen subscription.
If the route does not include a hostname, it is resolved relative to the microservice's default hostname.
If a port is not specified, 443 is used by default. Port 0 is used to designate any port.
Path arguments are designated by curly braces.

Examples of valid routes:

	(empty)
	/
	/path
	:1080
	:1080/
	:1080/path
	:0/any/port
	/path/with/slash
	path/with/no/slash
	/section/{section}/page/{page...}
	https://www.example.com/path
	https://www.example.com:1080/path
	//www.example.com:1080/path
*/
func Route(route string) Option {
	return func(sub *Subscription) error {
		sub.specPath = route
		return nil
	}
}

// At sets both the method and the route of a subscription in a single option.
// It is shorthand for [Method] followed by [Route].
func At(method string, route string) Option {
	return func(sub *Subscription) error {
		sub.Method = method
		sub.specPath = route
		return nil
	}
}

// Function declares the subscription as a typed function (RPC) endpoint.
// The inputs and outputs values are zero-value struct instances used for OpenAPI schema reflection.
func Function(inputs any, outputs any) Option {
	return func(sub *Subscription) error {
		if sub.Type != "" {
			return errors.New("type already set to '%s'", sub.Type)
		}
		sub.Type = TypeFunction
		sub.Inputs = inputs
		sub.Outputs = outputs
		return nil
	}
}

// Web declares the subscription as a raw HTTP web handler endpoint.
func Web() Option {
	return func(sub *Subscription) error {
		if sub.Type != "" {
			return errors.New("type already set to '%s'", sub.Type)
		}
		sub.Type = TypeWeb
		return nil
	}
}

// InboundEvent declares the subscription as an inbound event sink.
// The inputs and outputs values are zero-value struct instances used for OpenAPI schema reflection.
func InboundEvent(inputs any, outputs any) Option {
	return func(sub *Subscription) error {
		if sub.Type != "" {
			return errors.New("type already set to '%s'", sub.Type)
		}
		sub.Type = TypeInboundEvent
		sub.Inputs = inputs
		sub.Outputs = outputs
		return nil
	}
}

// Task declares the subscription as a workflow task endpoint.
// The inputs and outputs values are zero-value struct instances used for OpenAPI schema reflection.
func Task(inputs any, outputs any) Option {
	return func(sub *Subscription) error {
		if sub.Type != "" {
			return errors.New("type already set to '%s'", sub.Type)
		}
		sub.Type = TypeTask
		sub.Inputs = inputs
		sub.Outputs = outputs
		return nil
	}
}

// Workflow declares the subscription as a workflow graph endpoint.
// The inputs and outputs values are zero-value struct instances used for OpenAPI schema reflection.
func Workflow(inputs any, outputs any) Option {
	return func(sub *Subscription) error {
		if sub.Type != "" {
			return errors.New("type already set to '%s'", sub.Type)
		}
		sub.Type = TypeWorkflow
		sub.Inputs = inputs
		sub.Outputs = outputs
		return nil
	}
}

// Infra marks the subscription as framework infrastructure that brackets the user lifecycle:
// it is activated before [OnStartup] runs and deactivated after [OnShutdown] returns.
// Non-infra subscriptions are activated after OnStartup returns and deactivated before
// OnShutdown runs, so user code gets a "clean" environment that doesn't receive requests
// outside the OnStartup/OnShutdown window. Reserved for framework-internal subscriptions whose
// handlers must be reachable from inside OnStartup and OnShutdown - e.g. the distributed cache.
func Infra() Option {
	return func(sub *Subscription) error {
		sub.Infra = true
		return nil
	}
}

// Ultra clears the [Infra] flag, restoring the default activation/deactivation behavior
// where the subscription brackets the user lifecycle from the outside (activated after
// OnStartup, deactivated before OnShutdown). It is the symmetric counterpart of [Infra]
// and is provided for API completeness; in practice subscriptions default to non-infra.
func Ultra() Option {
	return func(sub *Subscription) error {
		sub.Infra = false
		return nil
	}
}

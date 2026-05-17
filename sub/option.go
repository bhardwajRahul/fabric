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
	"net/http"
	"regexp"
	"strings"
	"time"

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

// TimeBudget declares the maximum duration the endpoint's handler may run.
// The inbound request's context deadline is shortened to the smaller of the
// caller-provided budget and this declared budget. A zero or negative duration
// declares no budget. The deadline binds only this handler's own context; it
// does not change what an upstream caller waits.
func TimeBudget(d time.Duration) Option {
	return func(sub *Subscription) error {
		if d < 0 {
			d = 0
		}
		sub.TimeBudget = d
		return nil
	}
}

// Method overrides the default "ANY" method for a Listen subscription.
// Accepts one of the recognized HTTP methods (GET, HEAD, POST, PUT, DELETE, CONNECT, OPTIONS,
// TRACE, PATCH) or "ANY" to match any method. Matching is case-insensitive; the value is
// normalized to uppercase. Returns an error at option-application time if the method is
// not recognized.
func Method(method string) Option {
	return func(sub *Subscription) error {
		upper := strings.ToUpper(method)
		if !knownMethods[upper] {
			return errors.New("unknown HTTP method '%s'", method, http.StatusMethodNotAllowed)
		}
		sub.Method = upper
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
// It is shorthand for [Method] followed by [Route]. Same method validation rules apply.
func At(method string, route string) Option {
	return func(sub *Subscription) error {
		upper := strings.ToUpper(method)
		if !knownMethods[upper] {
			return errors.New("unknown HTTP method '%s'", method, http.StatusMethodNotAllowed)
		}
		sub.Method = upper
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

// Manual opts the subscription out of the connector's automatic activate/deactivate passes.
// User code drives its lifecycle via [connector.Connector.ActivateSubscription] and
// [connector.Connector.DeactivateSubscription].
func Manual() Option {
	return func(sub *Subscription) error {
		sub.Manual = true
		return nil
	}
}

// Automatic clears the [Manual] flag, restoring the default automatic activate/deactivate behavior.
func Automatic() Option {
	return func(sub *Subscription) error {
		sub.Manual = false
		return nil
	}
}

// Tag attaches one or more free-form labels to the subscription. Tags are surfaced through
// [connector.Connector.Subscriptions] and let user code group related subscriptions for
// activation, deactivation, or other bulk operations. Tag names are not parsed by the
// framework; convention is short, lowercase, hyphen-or-dot-free identifiers (e.g. "python",
// "billing", "experimental"). Multiple calls accumulate; duplicates are preserved as written.
func Tag(tags ...string) Option {
	return func(sub *Subscription) error {
		for _, t := range tags {
			if t == "" {
				return errors.New("empty tag")
			}
		}
		sub.Tags = append(sub.Tags, tags...)
		return nil
	}
}

// Untag removes one or more tags previously attached with [Tag]. Tags not currently set are
// silently ignored. Provided for symmetry with [Tag] in programmatically composed option lists.
func Untag(tags ...string) Option {
	return func(sub *Subscription) error {
		if len(sub.Tags) == 0 || len(tags) == 0 {
			return nil
		}
		remove := make(map[string]bool, len(tags))
		for _, t := range tags {
			remove[t] = true
		}
		kept := sub.Tags[:0]
		for _, t := range sub.Tags {
			if !remove[t] {
				kept = append(kept, t)
			}
		}
		sub.Tags = kept
		return nil
	}
}

// NoTrace suppresses OpenTelemetry span creation for this subscription.
// Requests handled by this subscription will not appear in distributed traces.
// Useful for high-frequency or internal subscriptions that would otherwise add noise to traces.
func NoTrace() Option {
	return func(sub *Subscription) error {
		sub.NoTrace = true
		return nil
	}
}

// Trace clears the [NoTrace] flag, restoring the default behavior where each handled request creates an OpenTelemetry server span.
func Trace() Option {
	return func(sub *Subscription) error {
		sub.NoTrace = false
		return nil
	}
}

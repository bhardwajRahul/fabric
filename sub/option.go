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

// Pervasive is synonymous with NoQueue.
// Requests will be not be load-balanced, all instances of this microservice will receive the request
func Pervasive() Option {
	return NoQueue()
}

// DefaultQueue names the queue of this subscription to the hostname of the service.
// Requests will be load-balanced among all instances of this microservice.
func DefaultQueue() Option {
	return func(sub *Subscription) error {
		sub.Queue = sub.Host
		return nil
	}
}

// LoadBalanced is synonymous with DefaultQueue.
// Requests will be load-balanced among all instances of this microservice
func LoadBalanced() Option {
	return DefaultQueue()
}

// Actor requires that the properties of the actor associated with the request satisfy the boolean expression.
// For example: iss=='my_issuer' && (roles=~'admin' || roles=~'manager') && region=="US".
// The =~ and !~ operators evaluate the left operand against a regexp.
// String constants, including regexp patterns, must be quoted using single quotes, double quotes or backticks.
// A request that doesn't satisfy the constraint is denied with a 403 forbidden error.
func Actor(boolExp string) Option {
	return func(sub *Subscription) error {
		_, err := boolexp.Eval(boolExp, nil)
		if err != nil {
			return errors.Trace(err)
		}
		sub.Actor = boolExp
		return nil
	}
}

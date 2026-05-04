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
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/fabric/transport"
	"github.com/microbus-io/fabric/utils"
)

// knownMethods is the set of HTTP method tokens accepted on subscriptions: the standard
// methods per RFC 9110 §9 plus the framework-specific "ANY" wildcard meaning "match any method".
// Lookup expects the caller to have already uppercased the input.
var knownMethods = map[string]bool{
	http.MethodGet:     true,
	http.MethodHead:    true,
	http.MethodPost:    true,
	http.MethodPut:     true,
	http.MethodDelete:  true,
	http.MethodConnect: true,
	http.MethodOptions: true,
	http.MethodTrace:   true,
	http.MethodPatch:   true,
	"ANY":              true,
}


// HTTPHandler extends the standard http.Handler to also return an error.
type HTTPHandler func(w http.ResponseWriter, r *http.Request) (err error)

// Type values identify the kind of a Microbus endpoint behind a subscription.
const (
	TypeFunction     = "function"
	TypeWeb          = "web"
	TypeInboundEvent = "inboundevent"
	TypeTask         = "task"
	TypeWorkflow     = "workflow"
)

// Subscription handles incoming requests.
// Although technically public, it is used internally and should not be constructed by microservices directly.
type Subscription struct {
	Name           string
	Description    string
	Host           string
	Port           string
	Method         string
	Route          string
	Queue          string
	Handler        any
	Subs           []*transport.Subscription
	specPath       string
	RequiredClaims string
	Type           string
	Inputs         any
	Outputs        any
	Infra          bool
	NoTrace        bool
}

/*
NewSubscription creates a new subscription registered by name. The name must be a legal Go-style
upper-case identifier (e.g. "MyEndpoint2"). Exactly one feature option ([Function], [Web],
[InboundEvent], [Task], [Graph]) must be supplied. Defaults filled in after options are applied:

  - Method defaults to "ANY".
  - Route defaults to ":<port>/<kebab-name>" with the port determined by the feature type
    (443 for function/web, 417 for inbound events, 428 for tasks/graphs).
  - Queue defaults to defaultHost.
*/
func NewSubscription(name string, defaultHost string, handler HTTPHandler, options ...Option) (*Subscription, error) {
	if !utils.IsUpperCaseIdentifier(name) {
		return nil, errors.New("invalid subscription name '%s', must be an upper-case identifier", name)
	}
	if handler == nil {
		return nil, errors.New("nil handler for subscription '%s'", name)
	}
	s := &Subscription{
		Name:    name,
		Handler: handler,
		Host:    defaultHost,
		Queue:   defaultHost,
	}
	if err := s.Apply(options...); err != nil {
		return nil, errors.Trace(err)
	}
	if s.Type == "" {
		return nil, errors.New("subscription '%s' missing a type option", name)
	}
	if s.Method == "" {
		s.Method = "ANY"
	} else {
		s.Method = strings.ToUpper(s.Method)
		if !knownMethods[s.Method] {
			return nil, errors.New("unknown HTTP method '%s'", s.Method, http.StatusMethodNotAllowed)
		}
	}
	if s.specPath == "" {
		s.specPath = ":" + defaultPortForType(s.Type) + "/" + utils.ToKebabCase(name)
	}
	joined := httpx.JoinHostAndPath(defaultHost, s.specPath)
	u, err := httpx.ParseURL(joined)
	if err != nil {
		return nil, errors.Trace(err)
	}
	if _, err := strconv.Atoi(u.Port()); err != nil {
		return nil, errors.Trace(err)
	}
	if err := validatePathArgs(u.Path); err != nil {
		return nil, errors.Trace(err)
	}
	s.Host = u.Hostname()
	s.Port = u.Port()
	s.Route = u.Path
	return s, nil
}

func defaultPortForType(t string) string {
	switch t {
	case TypeInboundEvent:
		return "417"
	case TypeTask, TypeWorkflow:
		return "428"
	default:
		return "443"
	}
}

func validatePathArgs(path string) error {
	parts := strings.Split(path, "/")
	for i := range parts {
		open := strings.Index(parts[i], "{")
		if open > 0 {
			return errors.New("path argument '%s' must span entire section", parts[i])
		}
		close := strings.LastIndex(parts[i], "}")
		if open == -1 && close == -1 {
			continue
		}
		if close <= open || open == -1 {
			return errors.New("malformed path argument '%s'", parts[i])
		}
		if close < len(parts[i])-1 {
			return errors.New("path argument '%s' must span entire section", parts[i])
		}
		name := parts[i]
		name = strings.TrimPrefix(name, "{")
		name = strings.TrimSuffix(name, "}")
		if strings.HasSuffix(name, "...") && i != len(parts)-1 {
			return errors.New("greedy path argument '%s' must end path", parts[i])
		}
		name = strings.TrimSuffix(name, "...")
		if name != "" && !utils.IsLowerCaseIdentifier(name) {
			return errors.New("name of path argument '%s' must be an identifier", parts[i])
		}
	}
	return nil
}

// Apply the provided options to the subscription.
func (sub *Subscription) Apply(options ...Option) error {
	for _, opt := range options {
		err := opt(sub)
		if err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

// Canonical returns the fully-qualified canonical host:port/path of the subscription, not including the scheme or the method.
func (sub *Subscription) Canonical() string {
	return fmt.Sprintf("%s:%s%s", sub.Host, sub.Port, sub.Route)
}

// RefreshHostname refreshes the subscription for a different hostname.
func (sub *Subscription) RefreshHostname(defaultHost string) error {
	joined := httpx.JoinHostAndPath(defaultHost, sub.specPath)
	u, err := httpx.ParseURL(joined)
	if err != nil {
		return errors.Trace(err)
	}
	sub.Host = u.Hostname()
	return nil
}

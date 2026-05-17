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
	"time"

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
	Path           string
	Queue          string
	Handler        any
	Subs           []*transport.Subscription
	specPath       string
	RequiredClaims string
	TimeBudget     time.Duration
	Type           string
	Inputs         any
	Outputs        any
	Manual         bool
	NoTrace        bool
	Tags           []string
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
	canonicalHost, err := validateRouteHostname(u.Hostname())
	if err != nil {
		return nil, errors.Trace(err)
	}
	if _, err := strconv.Atoi(u.Port()); err != nil {
		return nil, errors.Trace(err)
	}
	if err := validatePathArgs(u.Path); err != nil {
		return nil, errors.Trace(err)
	}
	s.Host = canonicalHost
	s.Port = u.Port()
	s.Path = u.Path
	return s, nil
}

// validateRouteHostname normalizes a subscription's route hostname to canonical
// lowercase form and validates it. Returns the canonical hostname for the caller
// to store, or an error if the hostname doesn't conform to Microbus rules.
//
// Routes are accepted in mixed case (e.g. "//UPPERCASE.xml/path") and lowercased
// here so they match request URLs after equivalent normalization. Underscores and
// other URL special characters are translated to the placeholder 'x' before the
// strict identity check, so a route on hostname "my_.xml" passes through to a
// base hostname of "myx.xml" and validates. The bare broadcast hostname "all" is
// accepted as a special case (the framework registers control endpoints on
// //all<route>).
//
// The 'x' translation is a hack contained to this package - it lets us reuse the
// single strict validator in httpx for both identity and route hostnames without
// exposing a generic "translate URL specials" helper to the rest of the codebase.
func validateRouteHostname(host string) (string, error) {
	canonical := strings.ToLower(host)
	if canonical == "all" {
		return canonical, nil
	}
	if err := httpx.ValidateHostname(translateHostnameSpecials(canonical)); err != nil {
		return "", err
	}
	return canonical, nil
}

// translateHostnameSpecials replaces any character outside [a-zA-Z0-9.-] with the
// placeholder 'x'. Uppercase letters are preserved so they fail the strict identity
// check downstream.
func translateHostnameSpecials(host string) string {
	var b strings.Builder
	b.Grow(len(host))
	for _, r := range host {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '.', r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('x')
		}
	}
	return b.String()
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
	return fmt.Sprintf("%s:%s%s", sub.Host, sub.Port, sub.Path)
}

// RefreshHostname refreshes the subscription for a different hostname.
func (sub *Subscription) RefreshHostname(defaultHost string) error {
	defaultHost = strings.ToLower(defaultHost)
	joined := httpx.JoinHostAndPath(defaultHost, sub.specPath)
	u, err := httpx.ParseURL(joined)
	if err != nil {
		return errors.Trace(err)
	}
	canonicalHost, err := validateRouteHostname(u.Hostname())
	if err != nil {
		return errors.Trace(err)
	}
	sub.Host = canonicalHost
	return nil
}

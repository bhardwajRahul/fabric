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

package main

// Rule construction for NATS PUB/SUB ACL subjects. Takes an aclInput
// populated by the source-driven scan (scan.go) and emits the rule set
// that gets signed into per-service user JWTs.

import (
	"fmt"
	"sort"
	"strings"

	"github.com/microbus-io/fabric/cmd/schema"
	"github.com/microbus-io/fabric/httpx"
)

// aclRule is one ACL line. Verb is "PUB" or "SUB"; Subject is the NATS
// subject pattern with {{plane}} as the only substitution token.
type aclRule struct {
	Verb    string
	Subject string
}

// aclInput is the data the rule builder needs, populated by the
// source-driven AST scan (scan.go).
type aclInput struct {
	Self           string            // service hostname
	Downstream     []aclDownstream   // outbound dependencies
	OutboundEvents []aclEventDef     // events this service triggers
	InboundEvents  []aclInboundEvent // events this service hooks
	OwnRoutes      []aclOwnRoute     // webs + functions + tasks + workflows
}

// aclDownstream is one downstream service with the per-call endpoints
// that this service invokes on it.
type aclDownstream struct {
	Hostname  string
	Endpoints []aclEndpoint
}

// aclEndpoint is one method call into a downstream. HostnameOverride
// represents a `.ForHost(...)` at the call site (literal hostname, or "*"
// for ForHost(varExpr) / dynamic dispatch).
type aclEndpoint struct {
	Route            string
	Method           string
	HostnameOverride string
}

// aclEventDef is the route/method of an outbound event this service
// triggers (self-PUB).
type aclEventDef struct {
	Route  string
	Method string
}

// aclInboundEvent is an event subscription. Hostname is the source
// service's hostname (where the event is published from).
type aclInboundEvent struct {
	Hostname string
	Route    string
	Method   string
}

// aclOwnRoute is one of the service's own subscribed endpoints. Used
// for the alt-host SUB pass and the trust-root self-rule check.
type aclOwnRoute struct {
	Route  string
	Method string
}

// buildACLRules constructs the full rule set: standard per-service rules
// followed by per-route rules from downstream[], outboundEvents[],
// inboundEvents[], and an alt-host SUB pass for own routes registered on
// a different hostname.
func buildACLRules(in aclInput) (rules []aclRule, warnings []string, err error) {
	if in.Self == "" {
		return nil, nil, fmt.Errorf("input has no Self")
	}
	selfFlat := schema.FlattenHostname(in.Self)

	// Standard per-service rules (framework-implicit; not derived from
	// per-call data).
	rules = append(rules,
		aclRule{"PUB", "{{plane}}.safe.888." + selfFlat + ".>"},
		aclRule{"PUB", "{{plane}}.reply._." + selfFlat + ".>"},
		aclRule{"SUB", "{{plane}}.safe.*.*." + selfFlat + ".>"},
		aclRule{"SUB", "{{plane}}.reply._.*." + selfFlat + ".*"},
		aclRule{"SUB", "{{plane}}.safe.888.*.all.>"},
	)
	if hasOwnTrustRoot(in) {
		rules = append(rules, aclRule{"SUB", "{{plane}}.danger.666.*." + selfFlat + ".>"})
	}

	// Per-route rules from downstream endpoints. Sorted by hostname for
	// deterministic output.
	dsList := append([]aclDownstream(nil), in.Downstream...)
	sort.Slice(dsList, func(i, j int) bool { return dsList[i].Hostname < dsList[j].Hostname })
	for _, ds := range dsList {
		for _, ep := range ds.Endpoints {
			subj, w := buildDownstreamSubject(in.Self, ds, ep)
			if w != "" {
				warnings = append(warnings, w)
				continue
			}
			rules = append(rules, aclRule{"PUB", subj})
		}
	}

	// outboundEvents - per-route self-PUB.
	for i, ev := range in.OutboundEvents {
		subj, w := buildOutboundEventSubject(in.Self, ev)
		if w != "" {
			warnings = append(warnings, fmt.Sprintf("outboundEvent %d: %s", i, w))
			continue
		}
		rules = append(rules, aclRule{"PUB", subj})
	}

	// inboundEvents - per-route SUB on the source's hostname.
	for i, ev := range in.InboundEvents {
		subj, w := buildInboundEventSubject(ev)
		if w != "" {
			warnings = append(warnings, fmt.Sprintf("inboundEvent %d: %s", i, w))
			continue
		}
		rules = append(rules, aclRule{"SUB", subj})
	}

	// Alt-host SUBs for own routes that register on a different hostname
	// (e.g. `//openapi.json:0`, `//root`). The connector subscribes the
	// service on the resolved alt-host; without these rules the broker
	// rejects the subscription.
	seenAlt := map[string]bool{}
	for _, r := range in.OwnRoutes {
		dest, port, encoded, ok := resolveACLRoute(in.Self, r.Route)
		if !ok || dest == selfFlat {
			continue
		}
		subj := formatSubSubject(dest, port, r.Method, encoded)
		if seenAlt[subj] {
			continue
		}
		seenAlt[subj] = true
		rules = append(rules, aclRule{"SUB", subj})
	}

	return rules, warnings, nil
}

// buildDownstreamSubject constructs the PUB subject for one downstream
// endpoint. Resolution goes through httpx.JoinHostAndPath + httpx.ParseURL,
// matching connector subscription semantics.
func buildDownstreamSubject(self string, ds aclDownstream, ep aclEndpoint) (subject, warning string) {
	effectiveHost := ds.Hostname
	if ep.HostnameOverride != "" {
		effectiveHost = ep.HostnameOverride
	}
	dest, port, path, ok := resolveACLRoute(effectiveHost, ep.Route)
	if !ok {
		return "", fmt.Sprintf("downstream %s route %q failed to resolve", ds.Hostname, ep.Route)
	}
	return formatPubSubject(self, dest, port, ep.Method, path), ""
}

// buildOutboundEventSubject constructs the self-PUB subject for an
// outbound event.
func buildOutboundEventSubject(self string, ev aclEventDef) (subject, warning string) {
	dest, port, path, ok := resolveACLRoute(self, ev.Route)
	if !ok {
		return "", fmt.Sprintf("route %q failed to resolve", ev.Route)
	}
	return formatPubSubject(self, dest, port, ev.Method, path), ""
}

// buildInboundEventSubject constructs the SUB subject for an inbound
// event. Source slot is `*` (subscriptions wildcard the source).
func buildInboundEventSubject(ev aclInboundEvent) (subject, warning string) {
	if ev.Hostname == "" {
		return "", "missing hostname"
	}
	dest, port, path, ok := resolveACLRoute(ev.Hostname, ev.Route)
	if !ok {
		return "", fmt.Sprintf("route %q failed to resolve", ev.Route)
	}
	return formatSubSubject(dest, port, ev.Method, path), ""
}

// resolveACLRoute runs (effectiveHost, route) through the connector's URL
// parsing, returning the flattened dest, port, and the already-encoded path
// segment. ok=false means the route is malformed.
//
// Wildcard inputs are handled before parsing: effectiveHost "*" → dest "*";
// route "*" → port "*" and path ">". These are emitted by the AST scan for
// dynamic dispatch and need to flow through to the ACL as wildcards.
func resolveACLRoute(effectiveHost, route string) (dest, port, encodedPath string, ok bool) {
	if effectiveHost == "" {
		return "", "", "", false
	}
	if route == "*" {
		dest = "*"
		if effectiveHost != "*" {
			dest = schema.FlattenHostname(effectiveHost)
		}
		return dest, "*", ">", true
	}
	if effectiveHost == "*" {
		_, port, encodedPath, ok = resolveACLRoute("placeholder.host", route)
		if !ok {
			return "", "", "", false
		}
		return "*", port, encodedPath, true
	}
	joined := httpx.JoinHostAndPath(effectiveHost, route)
	u, err := httpx.ParseURL(joined)
	if err != nil {
		return "", "", "", false
	}
	host := u.Hostname()
	if host == "" {
		return "", "", "", false
	}
	// Don't apply strict identity validation here. Route hostnames (`//my$.xml/...`)
	// legitimately contain URL specials that fail the identity check; they're already
	// validated by sub.NewSubscription at registration time and are simply encoded here.
	port = u.Port()
	if port == "" {
		port = "443"
	}
	return schema.FlattenHostname(host), port, encodePath(u.Path), true
}

// formatPubSubject renders a PUB rule's subject pattern from resolved parts.
//
//	{plane}.{trust}.{port}.{src_flat}.{dest_flat}.*.{method}.{path}
//
// id_or_locality is `*` for outbound PUB - publishers don't pin
// instance-id or locality on permissions.
func formatPubSubject(self, destFlat, port, method, encodedPath string) string {
	return "{{plane}}." + trustOf(port) + "." + portOrStar(port) + "." +
		schema.FlattenHostname(self) + "." + destFlat + ".*." +
		methodOrStar(method) + "." + encodedPath
}

// formatSubSubject renders a SUB rule's subject pattern. Source slot is
// always `*`.
func formatSubSubject(destFlat, port, method, encodedPath string) string {
	return "{{plane}}." + trustOf(port) + "." + portOrStar(port) + ".*." +
		destFlat + ".*." + methodOrStar(method) + "." + encodedPath
}

// trustOf maps a port to its trust segment. :666 is the trust-root
// boundary; every other port is on the safe trust.
func trustOf(port string) string {
	if port == "666" {
		return "danger"
	}
	return "safe"
}

// portOrStar returns "*" for the wildcard port (manifest "*"), otherwise
// the port literal. Port `0` is also wildcarded (matches connector
// semantics for "any port" routes registered via `:0`).
func portOrStar(port string) string {
	if port == "*" || port == "0" {
		return "*"
	}
	return port
}

// methodOrStar mirrors the connector's method-encoding: `ANY` and explicit
// `*` become `*`; otherwise the method is preserved uppercase.
func methodOrStar(method string) string {
	if method == "" || method == "ANY" || method == "*" {
		return "*"
	}
	return strings.ToUpper(method)
}

// hasOwnTrustRoot reports whether the input has any :666 own-route. Used
// to gate the `SUB <plane>.danger.666.*.<self>.>` standard rule.
func hasOwnTrustRoot(in aclInput) bool {
	for _, r := range in.OwnRoutes {
		if _, port, _, ok := resolveACLRoute(in.Self, r.Route); ok && port == "666" {
			return true
		}
	}
	return false
}

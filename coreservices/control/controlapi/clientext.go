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

/*
Package controlapi implements the public API of the control.core microservice,
including clients and data structures.

This microservice is created for the sake of generating the client API for the :888 control subscriptions.
The microservice itself does nothing and should not be included in applications.
*/
package controlapi

import (
	"context"
	"fmt"
	"iter"

	"github.com/microbus-io/fabric/frame"
)

// ServiceInfo is a descriptor of the microservice that answers the ping.
type ServiceInfo struct {
	Hostname string
	Version  int
	ID       string
}

// PingServices performs a ping and returns service info for microservices on the network.
// Results are deduped on a per-service basis.
func (_c MulticastClient) PingServices(ctx context.Context) iter.Seq[*ServiceInfo] {
	return func(yield func(*ServiceInfo) bool) {
		seen := map[string]bool{}
		for pingRes := range _c.Ping(ctx) {
			_r := (*multicastResponse)(pingRes)
			if _r.err != nil {
				continue
			}
			f := frame.Of(_r.HTTPResponse)
			info := &ServiceInfo{
				Hostname: f.FromHost(),
			}
			if seen[info.Hostname] {
				continue
			}
			seen[info.Hostname] = true
			if !yield(info) {
				return
			}
		}
	}
}

// PingVersions performs a ping and returns service info for microservice versions on the network.
// Results are deduped on a per-version basis.
func (_c MulticastClient) PingVersions(ctx context.Context) iter.Seq[*ServiceInfo] {
	return func(yield func(*ServiceInfo) bool) {
		seen := map[string]bool{}
		for pingRes := range _c.Ping(ctx) {
			_r := (*multicastResponse)(pingRes)
			if _r.err != nil {
				continue
			}
			f := frame.Of(_r.HTTPResponse)
			info := &ServiceInfo{
				Hostname: f.FromHost(),
				Version:  f.FromVersion(),
			}
			key := fmt.Sprintf("%s:%d", info.Hostname, info.Version)
			if seen[key] {
				continue
			}
			seen[key] = true
			if !yield(info) {
				return
			}
		}
	}
}

// PingInstances performs a ping and returns service info for all instances on the network.
func (_c MulticastClient) PingInstances(ctx context.Context) iter.Seq[*ServiceInfo] {
	return func(yield func(*ServiceInfo) bool) {
		for pingRes := range _c.Ping(ctx) {
			_r := (*multicastResponse)(pingRes)
			if _r.err != nil {
				continue
			}
			f := frame.Of(_r.HTTPResponse)
			info := &ServiceInfo{
				Hostname: f.FromHost(),
				Version:  f.FromVersion(),
				ID:       f.FromID(),
			}
			if !yield(info) {
				return
			}
		}
	}
}

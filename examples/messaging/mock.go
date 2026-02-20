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

package messaging

import (
	"context"
	"net/http"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/connector"

	"github.com/microbus-io/fabric/examples/messaging/messagingapi"
)

var (
	_ http.Request
	_ errors.TracedError
	_ messagingapi.Client
)

// Mock is a mockable version of the microservice, allowing functions, event sinks and web handlers to be mocked.
type Mock struct {
	*Intermediate
	mockHome         func(w http.ResponseWriter, r *http.Request) (err error) // MARKER: Home
	mockNoQueue      func(w http.ResponseWriter, r *http.Request) (err error) // MARKER: NoQueue
	mockDefaultQueue func(w http.ResponseWriter, r *http.Request) (err error) // MARKER: DefaultQueue
	mockCacheLoad    func(w http.ResponseWriter, r *http.Request) (err error) // MARKER: CacheLoad
	mockCacheStore   func(w http.ResponseWriter, r *http.Request) (err error) // MARKER: CacheStore
}

// NewMock creates a new mockable version of the microservice.
func NewMock() *Mock {
	svc := &Mock{}
	svc.Intermediate = NewIntermediate(svc)
	svc.SetVersion(7357) // Stands for TEST
	return svc
}

// OnStartup is called when the microservice is started up.
func (svc *Mock) OnStartup(ctx context.Context) (err error) {
	if svc.Deployment() != connector.LOCAL && svc.Deployment() != connector.TESTING {
		return errors.New("mocking disallowed in %s deployment", svc.Deployment())
	}
	return nil
}

// OnShutdown is called when the microservice is shut down.
func (svc *Mock) OnShutdown(ctx context.Context) (err error) {
	return nil
}

// MockHome sets up a mock handler for Home.
func (svc *Mock) MockHome(handler func(w http.ResponseWriter, r *http.Request) (err error)) *Mock { // MARKER: Home
	svc.mockHome = handler
	return svc
}

// Home executes the mock handler.
func (svc *Mock) Home(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Home
	if svc.mockHome == nil {
		return errors.New("mock not implemented", http.StatusNotImplemented)
	}
	err = svc.mockHome(w, r)
	return errors.Trace(err)
}

// MockNoQueue sets up a mock handler for NoQueue.
func (svc *Mock) MockNoQueue(handler func(w http.ResponseWriter, r *http.Request) (err error)) *Mock { // MARKER: NoQueue
	svc.mockNoQueue = handler
	return svc
}

// NoQueue executes the mock handler.
func (svc *Mock) NoQueue(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: NoQueue
	if svc.mockNoQueue == nil {
		return errors.New("mock not implemented", http.StatusNotImplemented)
	}
	err = svc.mockNoQueue(w, r)
	return errors.Trace(err)
}

// MockDefaultQueue sets up a mock handler for DefaultQueue.
func (svc *Mock) MockDefaultQueue(handler func(w http.ResponseWriter, r *http.Request) (err error)) *Mock { // MARKER: DefaultQueue
	svc.mockDefaultQueue = handler
	return svc
}

// DefaultQueue executes the mock handler.
func (svc *Mock) DefaultQueue(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: DefaultQueue
	if svc.mockDefaultQueue == nil {
		return errors.New("mock not implemented", http.StatusNotImplemented)
	}
	err = svc.mockDefaultQueue(w, r)
	return errors.Trace(err)
}

// MockCacheLoad sets up a mock handler for CacheLoad.
func (svc *Mock) MockCacheLoad(handler func(w http.ResponseWriter, r *http.Request) (err error)) *Mock { // MARKER: CacheLoad
	svc.mockCacheLoad = handler
	return svc
}

// CacheLoad executes the mock handler.
func (svc *Mock) CacheLoad(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: CacheLoad
	if svc.mockCacheLoad == nil {
		return errors.New("mock not implemented", http.StatusNotImplemented)
	}
	err = svc.mockCacheLoad(w, r)
	return errors.Trace(err)
}

// MockCacheStore sets up a mock handler for CacheStore.
func (svc *Mock) MockCacheStore(handler func(w http.ResponseWriter, r *http.Request) (err error)) *Mock { // MARKER: CacheStore
	svc.mockCacheStore = handler
	return svc
}

// CacheStore executes the mock handler.
func (svc *Mock) CacheStore(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: CacheStore
	if svc.mockCacheStore == nil {
		return errors.New("mock not implemented", http.StatusNotImplemented)
	}
	err = svc.mockCacheStore(w, r)
	return errors.Trace(err)
}

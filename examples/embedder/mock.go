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

package embedder

import (
	"context"
	"net/http"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/connector"
)

// Mock is a mockable version of the microservice, allowing functions, event sinks and web handlers to be mocked.
type Mock struct {
	*Intermediate
	mockEmbed      func(ctx context.Context, text string) (vector []float64, err error)     // MARKER: Embed
	mockSimilarity func(ctx context.Context, a string, b string) (score float64, err error) // MARKER: Similarity
	mockDemo       func(w http.ResponseWriter, r *http.Request) (err error)                 // MARKER: Demo
	mockDemoInit   func(w http.ResponseWriter, r *http.Request) (err error)                 // MARKER: DemoInit
	mockDemoStatus func(w http.ResponseWriter, r *http.Request) (err error)                 // MARKER: DemoStatus
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

// MockEmbed sets up a mock handler for Embed.
func (svc *Mock) MockEmbed(handler func(ctx context.Context, text string) (vector []float64, err error)) *Mock { // MARKER: Embed
	svc.mockEmbed = handler
	return svc
}

// Embed executes the mock handler.
func (svc *Mock) Embed(ctx context.Context, text string) (vector []float64, err error) { // MARKER: Embed
	if svc.mockEmbed != nil {
		vector, err = svc.mockEmbed(ctx, text)
	}
	return vector, errors.Trace(err)
}

// MockSimilarity sets up a mock handler for Similarity.
func (svc *Mock) MockSimilarity(handler func(ctx context.Context, a string, b string) (score float64, err error)) *Mock { // MARKER: Similarity
	svc.mockSimilarity = handler
	return svc
}

// Similarity executes the mock handler.
func (svc *Mock) Similarity(ctx context.Context, a string, b string) (score float64, err error) { // MARKER: Similarity
	if svc.mockSimilarity != nil {
		score, err = svc.mockSimilarity(ctx, a, b)
	}
	return score, errors.Trace(err)
}

// MockDemo sets up a mock handler for Demo.
func (svc *Mock) MockDemo(handler func(w http.ResponseWriter, r *http.Request) (err error)) *Mock { // MARKER: Demo
	svc.mockDemo = handler
	return svc
}

// Demo executes the mock handler.
func (svc *Mock) Demo(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Demo
	if svc.mockDemo != nil {
		err = svc.mockDemo(w, r)
	}
	return errors.Trace(err)
}

// MockDemoInit sets up a mock handler for DemoInit.
func (svc *Mock) MockDemoInit(handler func(w http.ResponseWriter, r *http.Request) (err error)) *Mock { // MARKER: DemoInit
	svc.mockDemoInit = handler
	return svc
}

// DemoInit executes the mock handler.
func (svc *Mock) DemoInit(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: DemoInit
	if svc.mockDemoInit != nil {
		err = svc.mockDemoInit(w, r)
	}
	return errors.Trace(err)
}

// MockDemoStatus sets up a mock handler for DemoStatus.
func (svc *Mock) MockDemoStatus(handler func(w http.ResponseWriter, r *http.Request) (err error)) *Mock { // MARKER: DemoStatus
	svc.mockDemoStatus = handler
	return svc
}

// DemoStatus executes the mock handler.
func (svc *Mock) DemoStatus(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: DemoStatus
	if svc.mockDemoStatus != nil {
		err = svc.mockDemoStatus(w, r)
	}
	return errors.Trace(err)
}

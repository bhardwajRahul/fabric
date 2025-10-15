/*
Copyright (c) 2023-2025 Microbus LLC and various contributors

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

package helloworld

import (
	"io"
	"net/http"
	"testing"

	"github.com/microbus-io/fabric/application"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/testarossa"

	"github.com/microbus-io/fabric/examples/helloworld/helloworldapi"
)

func TestHelloworld_HelloWorld(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	tt := testarossa.For(t)
	_ = ctx
	_ = tt

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("helloworld.helloworld.tester")
	client := helloworldapi.NewClient(tester)
	_ = client

	// Run the testing app
	app := application.NewTesting()
	app.Add(
		// Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	/*
		Recommended pattern:

		res, err := client.HelloWorld(ctx, "")
		if tt.NoError(err) && tt.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if tt.NoError(err) {
				// For HTML responses
				tt.HTMLMatch(body, `DIV.class > DIV#id`, "")
				// For text responses
				tt.Contains(body, "")
			}
		}
	*/

	res, err := client.HelloWorld(ctx, "")
	if tt.NoError(err) && tt.Expect(res.StatusCode, http.StatusOK) {
		body, err := io.ReadAll(res.Body)
		if tt.NoError(err) {
			tt.Contains(body, "Hello, World!")
		}
	}
}

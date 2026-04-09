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

package openapiportal

import (
	"context"
	"net/http"
	"testing"

	"github.com/golang-jwt/jwt/v5"

	"github.com/microbus-io/fabric/application"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/fabric/pub"
	"github.com/microbus-io/testarossa"

	"github.com/microbus-io/fabric/coreservices/openapiportal/openapiportalapi"
)

var (
	_ context.Context
	_ *testing.T
	_ jwt.MapClaims
	_ application.Application
	_ connector.Connector
	_ frame.Frame
	_ pub.Option
	_ testarossa.Asserter
	_ openapiportalapi.Client
)

func TestOpenapiportal_Mock(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	mock := NewMock()
	mock.SetDeployment(connector.TESTING)

	t.Run("on_startup", func(t *testing.T) {
		assert := testarossa.For(t)
		err := mock.OnStartup(ctx)
		assert.NoError(err)

		mock.SetDeployment(connector.PROD)
		err = mock.OnStartup(ctx)
		assert.Error(err)
		mock.SetDeployment(connector.TESTING)
	})

	t.Run("on_shutdown", func(t *testing.T) {
		assert := testarossa.For(t)
		err := mock.OnShutdown(ctx)
		assert.NoError(err)
	})

	t.Run("document", func(t *testing.T) { // MARKER: Document
		assert := testarossa.For(t)

		w := httpx.NewResponseRecorder()
		r := httpx.MustNewRequest("GET", "/", nil)

		err := mock.Document(w, r)
		assert.Error(err) // Not mocked yet
		mock.MockDocument(func(w http.ResponseWriter, r *http.Request) (err error) {
			w.WriteHeader(http.StatusOK)
			return nil
		})
		err = mock.Document(w, r)
		assert.NoError(err)
	})

	t.Run("explorer", func(t *testing.T) { // MARKER: Explorer
		assert := testarossa.For(t)

		w := httpx.NewResponseRecorder()
		r := httpx.MustNewRequest("GET", "/", nil)

		err := mock.Explorer(w, r)
		assert.Error(err) // Not mocked yet
		mock.MockExplorer(func(w http.ResponseWriter, r *http.Request) (err error) {
			w.WriteHeader(http.StatusOK)
			return nil
		})
		err = mock.Explorer(w, r)
		assert.NoError(err)
	})
}

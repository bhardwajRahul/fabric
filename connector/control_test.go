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

package connector

import (
	"testing"

	"github.com/microbus-io/fabric/pub"
	"github.com/microbus-io/testarossa"
)

func TestConnector_Ping(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	ctx := t.Context()

	// Create the microservice
	con := New("ping.connector")

	// Startup the microservice
	err := con.Startup(ctx)
	assert.NoError(err)
	defer con.Shutdown(ctx)

	// Send messages
	for r := range con.Publish(ctx, pub.GET("https://ping.connector:888/ping")) {
		_, err := r.Get()
		assert.NoError(err)
	}
	for r := range con.Publish(ctx, pub.GET("https://"+con.id+".ping.connector:888/ping")) {
		_, err := r.Get()
		assert.NoError(err)
	}
	for r := range con.Publish(ctx, pub.GET("https://all:888/ping")) {
		_, err := r.Get()
		assert.NoError(err)
	}
	for r := range con.Publish(ctx, pub.GET("https://"+con.id+".all:888/ping")) {
		_, err := r.Get()
		assert.NoError(err)
	}
}

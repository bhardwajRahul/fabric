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
	"context"
	"html"
	"io"
	"net/http"
	"testing"

	"github.com/microbus-io/fabric/pub"
	"github.com/microbus-io/testarossa"
)

func TestConnector_ReadResFile(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	// Create the microservices
	con := New("read.res.file.connector")
	con.SetResFSDir("testdata")

	tt.Equal("<html>{{ . }}</html>\n", string(con.MustReadResFile("res.txt")))
	tt.Equal("<html>{{ . }}</html>\n", con.MustReadResTextFile("res.txt"))

	tt.Nil(con.MustReadResFile("nothing.txt"))
	tt.Equal("", con.MustReadResTextFile("nothing.txt"))

	v, err := con.ExecuteResTemplate("res.txt", "<body></body>")
	tt.NoError(err)
	tt.Equal("<html><body></body></html>\n", v)

	v, err = con.ExecuteResTemplate("res.html", "<body></body>")
	tt.NoError(err)
	tt.Equal("<html>"+html.EscapeString("<body></body>")+"</html>\n", v)
}

func TestConnector_LoadResString(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	ctx := context.Background()

	// Create the microservices
	alpha := New("alpha.load.res.string.connector")

	beta := New("beta.load.res.string.connector")
	beta.Subscribe("GET", "localized", func(w http.ResponseWriter, r *http.Request) error {
		s, _ := beta.LoadResString(r.Context(), "hello")
		w.Write([]byte(s))
		return nil
	})
	beta.SetResFSDir("testdata")

	// Startup the microservices
	err := alpha.Startup()
	tt.NoError(err)
	defer alpha.Shutdown()
	err = beta.Startup()
	tt.NoError(err)
	defer beta.Shutdown()

	// Send message and validate the correct language
	testCases := []string{
		"", "Hello",
		"en", "Hello",
		"en-CA", "Hello",
		"en-AU", "G'day",
		"fr;q=0.9", "Hello",
		"it", "Ciao",
		"en;q=0.8, it", "Ciao",
		"en;q=0.8, en-AU;q=0.85", "G'day",
	}
	for i := 0; i < len(testCases); i += 2 {
		response, err := alpha.Request(ctx, pub.GET("https://beta.load.res.string.connector/localized"), pub.Header("Accept-Language", testCases[i]))
		if tt.NoError(err) {
			body, err := io.ReadAll(response.Body)
			if tt.NoError(err) {
				tt.Equal(testCases[i+1], string(body))
			}
		}
	}
}

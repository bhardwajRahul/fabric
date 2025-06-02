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

package middleware

import (
	"net/http"
	"testing"

	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/testarossa"
)

func TestChain_CRUD(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	noop := func(next connector.HTTPHandler) connector.HTTPHandler {
		return func(w http.ResponseWriter, r *http.Request) error {
			return nil
		}
	}

	chain := &Chain{}
	tt.Equal("", chain.String())

	chain.Append("10", noop)
	chain.Append("20", noop)
	tt.Equal("10 -> 20", chain.String())
	tt.False(chain.Exists("5"))
	tt.True(chain.Exists("10"))
	tt.False(chain.Exists("15"))
	tt.True(chain.Exists("20"))

	chain.InsertBefore("10", "5", noop)
	chain.InsertAfter("10", "15", noop)
	tt.Equal("5 -> 10 -> 15 -> 20", chain.String())
	tt.True(chain.Exists("5"))
	tt.True(chain.Exists("10"))
	tt.True(chain.Exists("15"))
	tt.True(chain.Exists("20"))

	chain.Replace("10", noop)
	tt.Equal("5 -> 10 -> 15 -> 20", chain.String())

	chain.Delete("10")
	chain.Delete("20")
	tt.Equal("5 -> 15", chain.String())

	chain.Prepend("0", noop)
	tt.Equal("0 -> 5 -> 15", chain.String())

	chain.Clear()
	tt.Equal("", chain.String())

	chain.Replace("10", noop)
	chain.InsertBefore("10", "5", noop)
	chain.InsertAfter("10", "15", noop)
	chain.Delete("20")
	tt.Equal("", chain.String())
	tt.False(chain.Exists("5"))
	tt.False(chain.Exists("10"))
	tt.False(chain.Exists("15"))
	tt.False(chain.Exists("20"))

	chain.Prepend("ALPHA", noop)
	tt.Equal("ALPHA", chain.String())
	tt.True(chain.Exists("ALPHA"))
	tt.True(chain.Exists("alpha"))
}

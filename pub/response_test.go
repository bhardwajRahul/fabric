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

package pub

import (
	"errors"
	"net/http"
	"testing"

	"github.com/microbus-io/testarossa"
)

func TestPub_Response(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	myErr := errors.New("my error")
	r := NewErrorResponse(myErr)
	res, err := r.Get()
	assert.Nil(res)
	assert.Equal(&myErr, &err)

	var myRes http.Response
	r = NewHTTPResponse(&myRes)
	res, err = r.Get()
	assert.NoError(err)
	assert.Equal(&myRes, res)
}

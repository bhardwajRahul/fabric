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
	"net/http"
	"sync"
	"testing"

	"github.com/microbus-io/testarossa"
)

func TestPub_ResponseQueueSingleConsumer(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	q := NewResponseQueue(0)

	// Push 5 responses then close
	go func() {
		for i := range 5 {
			q.Push(NewHTTPResponse(&http.Response{StatusCode: 200 + i}))
		}
		q.Close()
	}()

	// Single consumer collects all responses
	var codes []int
	for r := range q.Q() {
		res, err := r.Get()
		assert.NoError(err)
		codes = append(codes, res.StatusCode)
	}
	assert.Equal([]int{200, 201, 202, 203, 204}, codes)
}

func TestPub_ResponseQueueTwoConsumers(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	q := NewResponseQueue(0)

	// Push 100 responses then close
	go func() {
		for i := range 100 {
			q.Push(NewHTTPResponse(&http.Response{StatusCode: i}))
		}
		q.Close()
	}()

	// Two consumers fan out the responses between them
	var mu sync.Mutex
	var codes1, codes2 []int
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for r := range q.Q() {
			res, _ := r.Get()
			mu.Lock()
			codes1 = append(codes1, res.StatusCode)
			mu.Unlock()
		}
	}()
	go func() {
		defer wg.Done()
		for r := range q.Q() {
			res, _ := r.Get()
			mu.Lock()
			codes2 = append(codes2, res.StatusCode)
			mu.Unlock()
		}
	}()
	wg.Wait()

	// Every response should be consumed exactly once across both consumers
	all := append(codes1, codes2...)
	assert.Equal(100, len(all))

	seen := map[int]bool{}
	for _, code := range all {
		assert.False(seen[code]) // No duplicates
		seen[code] = true
	}
	for i := range 100 {
		assert.True(seen[i]) // None missing
	}
}

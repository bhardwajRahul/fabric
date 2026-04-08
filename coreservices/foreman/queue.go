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

package foreman

import "sync"

// job holds a step ID and its shard index for the worker queue.
type job struct {
	stepID int
	shard  int
}

// jobNode is a linked list node holding a job.
type jobNode struct {
	job  job
	next *jobNode
}

// jobQueue is a FIFO linked list queue with condition variable signaling.
type jobQueue struct {
	mu     sync.Mutex
	cond   *sync.Cond
	head   *jobNode
	tail   *jobNode
	closed bool
}

// init initializes the condition variable. Must be called before use.
func (q *jobQueue) init() {
	q.cond = sync.NewCond(&q.mu)
	q.head = nil
	q.tail = nil
	q.closed = false
}

// push appends a job to the tail of the queue and signals one waiting worker.
func (q *jobQueue) push(j job) {
	n := &jobNode{job: j}
	q.mu.Lock()
	if q.tail != nil {
		q.tail.next = n
	} else {
		q.head = n
	}
	q.tail = n
	q.mu.Unlock()
	q.cond.Signal()
}

// pop removes and returns the job at the head of the queue.
// It blocks until a job is available or the queue is closed.
// Returns an empty job and false if the queue is closed and empty.
func (q *jobQueue) pop() (j job, ok bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for q.head == nil && !q.closed {
		q.cond.Wait()
	}
	if q.head == nil {
		return job{}, false
	}
	n := q.head
	q.head = n.next
	if q.head == nil {
		q.tail = nil
	}
	return n.job, true
}

// len returns the number of items currently in the queue.
func (q *jobQueue) len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	n := 0
	for node := q.head; node != nil; node = node.next {
		n++
	}
	return n
}

// close signals all waiting workers to wake up and exit.
func (q *jobQueue) close() {
	if q.cond == nil {
		return
	}
	q.mu.Lock()
	q.closed = true
	q.mu.Unlock()
	q.cond.Broadcast()
}

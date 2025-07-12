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

package transport

import (
	"fmt"
	"io"
	"strings"
	"sync"
)

// trie is a prefix tree that keeps track of all subscriptions, supporting wildcards in the path.
// The trie data structure is safe for concurrent use.
type trie struct {
	children map[string]*trie
	parent   *trie
	queues   map[string]*ringList
	segment  string
	mux      sync.Mutex // Only used by root
}

// IsEmpty indicates if the trie is empty.
func (pt *trie) IsEmpty() bool {
	pt.mux.Lock()
	empty := len(pt.children) == 0 && len(pt.queues) == 0
	pt.mux.Unlock()
	return empty
}

// Sub sets a handler to process messages delivered to the subject.
// If a queue is specified, only one of the subscribers is passed any given message.
// If no queue is specified, all subscribers are passed all messages.
// The returned unsub function removes interest in the subject.
func (pt *trie) Sub(subject string, queue string, handler MsgHandler) (unsub func()) {
	pt.mux.Lock()
	j := pt
	for seg := range strings.SplitSeq(subject, ".") {
		if seg == "" {
			continue
		}
		next := j.children[seg]
		if next == nil {
			if j.children == nil {
				j.children = map[string]*trie{}
			}
			next = &trie{
				segment: seg,
				parent:  j,
			}
			j.children[seg] = next
		}
		j = next
	}
	if j.queues == nil {
		j.queues = map[string]*ringList{
			queue: {},
		}
	} else if j.queues[queue] == nil {
		j.queues[queue] = &ringList{}
	}
	gem := j.queues[queue].Insert(handler)
	pt.mux.Unlock()
	return func() {
		pt.unsub(j, queue, gem)
	}
}

// nodeUnsub removes interest in the subscription and trims empty nodes going up towards the root of the trie.
func (pt *trie) unsub(node *trie, queue string, gem *stone) {
	var j *trie
	pt.mux.Lock()
	ring := node.queues[queue]
	if ring.Remove(gem) && ring.IsEmpty() {
		delete(node.queues, queue)
		j = node
	}
	for j != nil {
		empty := len(j.children) == 0 && len(j.queues) == 0
		parent := j.parent
		if empty && parent != nil {
			delete(parent.children, j.segment)
		}
		if !empty {
			break
		}
		j = parent
	}
	pt.mux.Unlock()
}

var queuePool = sync.Pool{
	New: func() any {
		b := make([]*trie, 0, 32)
		return &b
	},
}

// appendNodeHandlers selects handlers from each of the queues associated with the trie node.
// One handler is selected from each named queue.
// All handlers are selected from the unnamed queue.
// This method must be called under a lock.
func (pt *trie) appendNodeHandlers(appendTo []MsgHandler) []MsgHandler {
	for q, namedRing := range pt.queues {
		if q == "" {
			continue
		}
		head := namedRing.Rotate()
		if head == nil {
			continue // Ring is empty
		}
		appendTo = append(appendTo, head.Handler)
	}
	unnamedRing, ok := pt.queues[""]
	if ok {
		head := unnamedRing.Rotate()
		if head != nil {
			appendTo = append(appendTo, head.Handler)
			for {
				h := unnamedRing.Rotate()
				if h == head {
					break
				}
				appendTo = append(appendTo, h.Handler)
			}
		}
	}
	return appendTo
}

// Handlers returns the message handlers matching the indicated subject, considering wildcards.
func (pt *trie) Handlers(subject string) (handlers []MsgHandler) {
	ptrQueue := queuePool.Get().(*[]*trie)
	defer queuePool.Put(ptrQueue)
	queue := *ptrQueue

	var suffix []*trie
	queue = append(queue, pt)
	pt.mux.Lock()
	for seg := range strings.SplitSeq(subject, ".") {
		if len(queue) == 0 {
			break
		}
		if seg == "" {
			continue
		}
		n := len(queue)
		for range n {
			// Pop from the queue
			j := queue[0]
			queue = queue[1:]
			if len(queue) == 0 {
				queue = *ptrQueue
			}

			segChild := j.children[seg]
			starChild := j.children["*"]
			suffixChild := j.children[">"]
			if segChild != nil {
				queue = append(queue, segChild)
			}
			if starChild != nil {
				queue = append(queue, starChild)
			}
			if suffixChild != nil {
				suffix = append(suffix, suffixChild)
			}
		}
	}
	for _, j := range queue {
		handlers = j.appendNodeHandlers(handlers)
	}
	for _, j := range suffix {
		handlers = j.appendNodeHandlers(handlers)
	}
	pt.mux.Unlock()
	return handlers
}

// String prints the trie to a string.
func (pt *trie) String() string {
	var sb strings.Builder
	pt.mux.Lock()
	pt.nodePrint(&sb, 0)
	pt.mux.Unlock()
	return sb.String()
}

// nodePrint prints the data of the node to a writer.
// This method must be called under a lock.
func (pt *trie) nodePrint(w io.Writer, indent int) {
	seg := pt.segment
	if seg == "" {
		seg = "~"
	}
	fmt.Fprintf(w, "%s%s", strings.Repeat(" ", indent), seg)
	if len(pt.queues) > 0 {
		fmt.Fprint(w, " {")
	}
	for q, ring := range pt.queues {
		idx := 0
		if ring != nil {
			idx = ring.index
		}
		fmt.Fprintf(w, "%d:%s", idx, q)
	}
	if len(pt.queues) > 0 {
		fmt.Fprint(w, "}")
	}
	fmt.Fprint(w, "\n")
	for _, next := range pt.children {
		next.nodePrint(w, indent+2)
	}
}

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

package transport

import (
	"github.com/microbus-io/errors"
	"github.com/nats-io/nats.go"
)

// Subscription is an expression of interest in a subject.
type Subscription struct {
	conn              *Conn
	next              *Subscription
	prev              *Subscription
	shortCircuitUnsub func()
	natsSub           *nats.Subscription
	done              bool
}

// Unsubscribe removes interest in the subject of the subscription.
func (s *Subscription) Unsubscribe() (err error) {
	err = s.conn.unsubscribe(s)
	return errors.Trace(err)
}

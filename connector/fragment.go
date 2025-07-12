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
	"net/http"
	"time"

	"github.com/microbus-io/fabric/errors"
	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/fabric/httpx"
)

const fragTimeoutMultiplier = 8

// defragRequest assembles all fragments of an incoming HTTP request and returns the integrated HTTP request.
// If not all fragments are available yet, it returns nil
func (c *Connector) defragRequest(r *http.Request) (integrated *http.Request, err error) {
	_, fragmentMax := frame.Of(r).Fragment()
	if fragmentMax <= 1 {
		return r, nil
	}
	fromID := frame.Of(r).FromID()
	msgID := frame.Of(r).MessageID()
	fragKey := fromID + "|" + msgID

	defragger, loaded := c.requestDefrags.LoadOrStoreFunc(fragKey, func() *httpx.DefragRequest {
		return httpx.NewDefragRequest()
	})
	if !loaded {
		// Timeout if fragments stop arriving
		go func() {
			for {
				time.Sleep(c.networkHop)
				if _, ok := c.requestDefrags.Load(fragKey); !ok {
					break
				}
				if defragger.LastActivity() > fragTimeoutMultiplier*c.networkHop {
					c.requestDefrags.Store(fragKey, nil) // Nil indicates a timeouts
					break
				}
			}
		}()
	}
	if defragger == nil {
		// Most likely caused after a timeout, but can also happen if initial chunk has wrong index
		return nil, errors.New("defrag timeout", http.StatusRequestTimeout)
	}
	final, err := defragger.Add(r)
	if err != nil {
		return nil, errors.Trace(err)
	}
	if !final {
		// Not all fragments arrived yet
		return nil, nil
	}
	c.requestDefrags.Delete(fragKey)
	integrated, err = defragger.Integrated()
	return integrated, errors.Trace(err)
}

// defragResponse assembles all fragments of an incoming HTTP response and returns the integrated HTTP request.
// If not all fragments are available yet, it returns nil
func (c *Connector) defragResponse(r *http.Response) (integrated *http.Response, err error) {
	_, fragmentMax := frame.Of(r).Fragment()
	if fragmentMax <= 1 {
		return r, nil
	}
	fromID := frame.Of(r).FromID()
	msgID := frame.Of(r).MessageID()
	fragKey := fromID + "|" + msgID

	defragger, loaded := c.responseDefrags.LoadOrStoreFunc(fragKey, func() *httpx.DefragResponse {
		return httpx.NewDefragResponse()
	})
	if !loaded {
		// Timeout if fragments stop arriving
		go func() {
			for {
				time.Sleep(c.networkHop)
				if _, ok := c.responseDefrags.Load(fragKey); !ok {
					break
				}
				if defragger.LastActivity() > fragTimeoutMultiplier*c.networkHop {
					c.responseDefrags.Store(fragKey, nil) // Nil indicates a timeouts
					break
				}
			}
		}()
	}
	if defragger == nil {
		// Most likely caused after a timeout, but can also happen if initial chunk has wrong index
		return nil, errors.New("defrag timeout", http.StatusRequestTimeout)
	}
	final, err := defragger.Add(r)
	if err != nil {
		return nil, errors.Trace(err)
	}
	if !final {
		// Not all fragments arrived yet
		return nil, nil
	}
	c.responseDefrags.Delete(fragKey)
	integrated, err = defragger.Integrated()
	return integrated, errors.Trace(err)
}

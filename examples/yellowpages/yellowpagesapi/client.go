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

package yellowpagesapi

import (
	"context"
	"encoding/json"
	"iter"
	"net/http"
	"reflect"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/fabric/pub"
	"github.com/microbus-io/fabric/service"
	"github.com/microbus-io/fabric/sub"
)

var (
	_ context.Context
	_ json.Encoder
	_ *http.Request
	_ *errors.TracedError
	_ *httpx.BodyReader
	_ = marshalRequest
	_ = marshalPublish
	_ = marshalFunction
)

// Hostname is the default hostname of the microservice.
const Hostname = "yellowpages.example"

// Def defines an endpoint of the microservice.
type Def struct {
	Method string
	Route  string
}

// URL is the full URL to the endpoint.
func (d *Def) URL() string {
	return httpx.JoinHostAndPath(Hostname, d.Route)
}

// Endpoint routes.
const (
	// HINT: Insert endpoint route definitions here
	RouteOfWebUI = `/web-ui` // MARKER: WebUI
)

// Endpoint URLs.
var (
	// HINT: Insert endpoint URL definitions here
	URLOfWebUI = httpx.JoinHostAndPath(Hostname, RouteOfWebUI) // MARKER: WebUI
)

var (
	// HINT: Insert endpoint definitions here
	Create     = Def{Method: "ANY", Route: "/create"}           // MARKER: Create
	Store      = Def{Method: "ANY", Route: "/store"}            // MARKER: Store
	MustStore  = Def{Method: "ANY", Route: "/must-store"}       // MARKER: MustStore
	Revise     = Def{Method: "ANY", Route: "/revise"}           // MARKER: Revise
	MustRevise = Def{Method: "ANY", Route: "/must-revise"}      // MARKER: MustRevise
	Delete     = Def{Method: "ANY", Route: "/delete"}           // MARKER: Delete
	MustDelete = Def{Method: "ANY", Route: "/must-delete"}      // MARKER: MustDelete
	List       = Def{Method: "ANY", Route: "/list"}             // MARKER: List
	Lookup     = Def{Method: "ANY", Route: "/lookup"}           // MARKER: Lookup
	MustLookup = Def{Method: "ANY", Route: "/must-lookup"}      // MARKER: MustLookup
	Load       = Def{Method: "ANY", Route: "/load"}             // MARKER: Load
	MustLoad   = Def{Method: "ANY", Route: "/must-load"}        // MARKER: MustLoad
	BulkLoad   = Def{Method: "ANY", Route: "/bulk-load"}        // MARKER: BulkLoad
	BulkDelete = Def{Method: "ANY", Route: "/bulk-delete"}      // MARKER: BulkDelete
	BulkCreate = Def{Method: "ANY", Route: "/bulk-create"}      // MARKER: BulkCreate
	BulkStore  = Def{Method: "ANY", Route: "/bulk-store"}       // MARKER: BulkStore
	BulkRevise = Def{Method: "ANY", Route: "/bulk-revise"}      // MARKER: BulkRevise
	Purge      = Def{Method: "ANY", Route: "/purge"}            // MARKER: Purge
	Count      = Def{Method: "ANY", Route: "/count"}            // MARKER: Count
	CreateREST = Def{Method: "POST", Route: "/persons"}         // MARKER: CreateREST
	StoreREST  = Def{Method: "PUT", Route: "/persons/{key}"}    // MARKER: StoreREST
	DeleteREST = Def{Method: "DELETE", Route: "/persons/{key}"} // MARKER: DeleteREST
	LoadREST   = Def{Method: "GET", Route: "/persons/{key}"}    // MARKER: LoadREST
	ListREST   = Def{Method: "GET", Route: "/persons"}          // MARKER: ListREST
)

// multicastResponse packs the response of a functional multicast.
type multicastResponse struct {
	data         any
	HTTPResponse *http.Response
	err          error
}

// Client is a lightweight proxy for making unicast calls to the microservice.
type Client struct {
	svc  service.Publisher
	host string
	opts []pub.Option
}

// NewClient creates a new unicast client proxy to the microservice.
func NewClient(caller service.Publisher) Client {
	return Client{svc: caller, host: Hostname}
}

// ForHost returns a copy of the client with a different hostname to be applied to requests.
func (_c Client) ForHost(host string) Client {
	return Client{
		svc:  _c.svc,
		host: host,
		opts: _c.opts,
	}
}

// WithOptions returns a copy of the client with options to be applied to requests.
func (_c Client) WithOptions(opts ...pub.Option) Client {
	return Client{svc: _c.svc, host: _c.host, opts: append(_c.opts, opts...)}
}

// MulticastClient is a lightweight proxy for making multicast calls to the microservice.
type MulticastClient struct {
	svc  service.Publisher
	host string
	opts []pub.Option
}

// NewMulticastClient creates a new multicast client proxy to the microservice.
func NewMulticastClient(caller service.Publisher) MulticastClient {
	return MulticastClient{svc: caller, host: Hostname}
}

// ForHost returns a copy of the client with a different hostname to be applied to requests.
func (_c MulticastClient) ForHost(host string) MulticastClient {
	return MulticastClient{svc: _c.svc, host: host, opts: _c.opts}
}

// WithOptions returns a copy of the client with options to be applied to requests.
func (_c MulticastClient) WithOptions(opts ...pub.Option) MulticastClient {
	return MulticastClient{svc: _c.svc, host: _c.host, opts: append(_c.opts, opts...)}
}

// MulticastTrigger is a lightweight proxy for triggering the events of the microservice.
type MulticastTrigger struct {
	svc  service.Publisher
	host string
	opts []pub.Option
}

// NewMulticastTrigger creates a new multicast trigger of events of the microservice.
func NewMulticastTrigger(caller service.Publisher) MulticastTrigger {
	return MulticastTrigger{svc: caller, host: Hostname}
}

// ForHost returns a copy of the trigger with a different hostname to be applied to requests.
func (_c MulticastTrigger) ForHost(host string) MulticastTrigger {
	return MulticastTrigger{svc: _c.svc, host: host, opts: _c.opts}
}

// WithOptions returns a copy of the trigger with options to be applied to requests.
func (_c MulticastTrigger) WithOptions(opts ...pub.Option) MulticastTrigger {
	return MulticastTrigger{svc: _c.svc, host: _c.host, opts: append(_c.opts, opts...)}
}

// Hook assists in the subscription to the events of the microservice.
type Hook struct {
	svc  service.Subscriber
	host string
	opts []sub.Option
}

// NewHook creates a new hook to the events of the microservice.
func NewHook(listener service.Subscriber) Hook {
	return Hook{svc: listener, host: Hostname}
}

// ForHost returns a copy of the hook with a different hostname to be applied to the subscription.
func (c Hook) ForHost(host string) Hook {
	return Hook{svc: c.svc, host: host, opts: c.opts}
}

// WithOptions returns a copy of the hook with options to be applied to subscriptions.
func (c Hook) WithOptions(opts ...sub.Option) Hook {
	return Hook{svc: c.svc, host: c.host, opts: append(c.opts, opts...)}
}

// marshalRequest supports functional endpoints.
func marshalRequest(ctx context.Context, svc service.Publisher, opts []pub.Option, host string, method string, route string, in any, out any) (err error) {
	if method == "ANY" {
		method = "POST"
	}
	u := httpx.JoinHostAndPath(host, route)
	query, body, err := httpx.WriteInputPayload(method, in)
	if err != nil {
		return err // No trace
	}
	httpRes, err := svc.Request(
		ctx,
		pub.Method(method),
		pub.URL(u),
		pub.Query(query),
		pub.Body(body),
		pub.Options(opts...),
	)
	if err != nil {
		return err // No trace
	}
	err = httpx.ReadOutputPayload(httpRes, out)
	return errors.Trace(err)
}

// marshalPublish supports multicast functional endpoints.
func marshalPublish(ctx context.Context, svc service.Publisher, opts []pub.Option, host string, method string, route string, in any, out any) iter.Seq[*multicastResponse] {
	if method == "ANY" {
		method = "POST"
	}
	u := httpx.JoinHostAndPath(host, route)
	query, body, err := httpx.WriteInputPayload(method, in)
	if err != nil {
		return func(yield func(*multicastResponse) bool) {
			yield(&multicastResponse{err: err})
		}
	}
	_queue := svc.Publish(
		ctx,
		pub.Method(method),
		pub.URL(u),
		pub.Query(query),
		pub.Body(body),
		pub.Options(opts...),
	)
	return func(yield func(*multicastResponse) bool) {
		for qi := range _queue {
			httpResp, err := qi.Get()
			if err == nil {
				reflect.ValueOf(out).Elem().SetZero()
				err = httpx.ReadOutputPayload(httpResp, out)
			}
			if err != nil {
				if !yield(&multicastResponse{err: err, HTTPResponse: httpResp}) {
					return
				}
			} else {
				if !yield(&multicastResponse{data: out, HTTPResponse: httpResp}) {
					return
				}
			}
		}
	}
}

// --- Payload Structs ---

// CreateIn are the input arguments of Create.
type CreateIn struct { // MARKER: Create
	Obj *Person `json:"obj,omitzero"`
}

// CreateOut are the output arguments of Create.
type CreateOut struct { // MARKER: Create
	ObjKey PersonKey `json:"objKey,omitzero"`
}

// CreateResponse packs the response of Create.
type CreateResponse multicastResponse // MARKER: Create

// Get unpacks the return arguments of Create.
func (_res *CreateResponse) Get() (objKey PersonKey, err error) { // MARKER: Create
	_d := _res.data.(*CreateOut)
	return _d.ObjKey, _res.err
}

// StoreIn are the input arguments of Store.
type StoreIn struct { // MARKER: Store
	Obj *Person `json:"obj,omitzero"`
}

// StoreOut are the output arguments of Store.
type StoreOut struct { // MARKER: Store
	Stored bool `json:"stored,omitzero"`
}

// StoreResponse packs the response of Store.
type StoreResponse multicastResponse // MARKER: Store

// Get unpacks the return arguments of Store.
func (_res *StoreResponse) Get() (stored bool, err error) { // MARKER: Store
	_d := _res.data.(*StoreOut)
	return _d.Stored, _res.err
}

// MustStoreIn are the input arguments of MustStore.
type MustStoreIn struct { // MARKER: MustStore
	Obj *Person `json:"obj,omitzero"`
}

// MustStoreOut are the output arguments of MustStore.
type MustStoreOut struct { // MARKER: MustStore
}

// MustStoreResponse packs the response of MustStore.
type MustStoreResponse multicastResponse // MARKER: MustStore

// Get unpacks the return arguments of MustStore.
func (_res *MustStoreResponse) Get() (err error) { // MARKER: MustStore
	return _res.err
}

// ReviseIn are the input arguments of Revise.
type ReviseIn struct { // MARKER: Revise
	Obj *Person `json:"obj,omitzero"`
}

// ReviseOut are the output arguments of Revise.
type ReviseOut struct { // MARKER: Revise
	Revised bool `json:"revised,omitzero"`
}

// ReviseResponse packs the response of Revise.
type ReviseResponse multicastResponse // MARKER: Revise

// Get unpacks the return arguments of Revise.
func (_res *ReviseResponse) Get() (revised bool, err error) { // MARKER: Revise
	_d := _res.data.(*ReviseOut)
	return _d.Revised, _res.err
}

// MustReviseIn are the input arguments of MustRevise.
type MustReviseIn struct { // MARKER: MustRevise
	Obj *Person `json:"obj,omitzero"`
}

// MustReviseOut are the output arguments of MustRevise.
type MustReviseOut struct { // MARKER: MustRevise
}

// MustReviseResponse packs the response of MustRevise.
type MustReviseResponse multicastResponse // MARKER: MustRevise

// Get unpacks the return arguments of MustRevise.
func (_res *MustReviseResponse) Get() (err error) { // MARKER: MustRevise
	return _res.err
}

// DeleteIn are the input arguments of Delete.
type DeleteIn struct { // MARKER: Delete
	ObjKey PersonKey `json:"objKey,omitzero"`
}

// DeleteOut are the output arguments of Delete.
type DeleteOut struct { // MARKER: Delete
	Deleted bool `json:"deleted,omitzero"`
}

// DeleteResponse packs the response of Delete.
type DeleteResponse multicastResponse // MARKER: Delete

// Get unpacks the return arguments of Delete.
func (_res *DeleteResponse) Get() (deleted bool, err error) { // MARKER: Delete
	_d := _res.data.(*DeleteOut)
	return _d.Deleted, _res.err
}

// MustDeleteIn are the input arguments of MustDelete.
type MustDeleteIn struct { // MARKER: MustDelete
	ObjKey PersonKey `json:"objKey,omitzero"`
}

// MustDeleteOut are the output arguments of MustDelete.
type MustDeleteOut struct { // MARKER: MustDelete
}

// MustDeleteResponse packs the response of MustDelete.
type MustDeleteResponse multicastResponse // MARKER: MustDelete

// Get unpacks the return arguments of MustDelete.
func (_res *MustDeleteResponse) Get() (err error) { // MARKER: MustDelete
	return _res.err
}

// ListIn are the input arguments of List.
type ListIn struct { // MARKER: List
	Query Query `json:"query,omitzero"`
}

// ListOut are the output arguments of List.
type ListOut struct { // MARKER: List
	Objs       []*Person `json:"objs,omitzero"`
	TotalCount int       `json:"totalCount,omitzero"`
}

// ListResponse packs the response of List.
type ListResponse multicastResponse // MARKER: List

// Get unpacks the return arguments of List.
func (_res *ListResponse) Get() (objs []*Person, totalCount int, err error) { // MARKER: List
	_d := _res.data.(*ListOut)
	return _d.Objs, _d.TotalCount, _res.err
}

// LookupIn are the input arguments of Lookup.
type LookupIn struct { // MARKER: Lookup
	Query Query `json:"query,omitzero"`
}

// LookupOut are the output arguments of Lookup.
type LookupOut struct { // MARKER: Lookup
	Obj   *Person `json:"obj,omitzero"`
	Found bool    `json:"found,omitzero"`
}

// LookupResponse packs the response of Lookup.
type LookupResponse multicastResponse // MARKER: Lookup

// Get unpacks the return arguments of Lookup.
func (_res *LookupResponse) Get() (obj *Person, found bool, err error) { // MARKER: Lookup
	_d := _res.data.(*LookupOut)
	return _d.Obj, _d.Found, _res.err
}

// MustLookupIn are the input arguments of MustLookup.
type MustLookupIn struct { // MARKER: MustLookup
	Query Query `json:"query,omitzero"`
}

// MustLookupOut are the output arguments of MustLookup.
type MustLookupOut struct { // MARKER: MustLookup
	Obj *Person `json:"obj,omitzero"`
}

// MustLookupResponse packs the response of MustLookup.
type MustLookupResponse multicastResponse // MARKER: MustLookup

// Get unpacks the return arguments of MustLookup.
func (_res *MustLookupResponse) Get() (obj *Person, err error) { // MARKER: MustLookup
	_d := _res.data.(*MustLookupOut)
	return _d.Obj, _res.err
}

// LoadIn are the input arguments of Load.
type LoadIn struct { // MARKER: Load
	ObjKey PersonKey `json:"objKey,omitzero"`
}

// LoadOut are the output arguments of Load.
type LoadOut struct { // MARKER: Load
	Obj   *Person `json:"obj,omitzero"`
	Found bool    `json:"found,omitzero"`
}

// LoadResponse packs the response of Load.
type LoadResponse multicastResponse // MARKER: Load

// Get unpacks the return arguments of Load.
func (_res *LoadResponse) Get() (obj *Person, found bool, err error) { // MARKER: Load
	_d := _res.data.(*LoadOut)
	return _d.Obj, _d.Found, _res.err
}

// MustLoadIn are the input arguments of MustLoad.
type MustLoadIn struct { // MARKER: MustLoad
	ObjKey PersonKey `json:"objKey,omitzero"`
}

// MustLoadOut are the output arguments of MustLoad.
type MustLoadOut struct { // MARKER: MustLoad
	Obj *Person `json:"obj,omitzero"`
}

// MustLoadResponse packs the response of MustLoad.
type MustLoadResponse multicastResponse // MARKER: MustLoad

// Get unpacks the return arguments of MustLoad.
func (_res *MustLoadResponse) Get() (obj *Person, err error) { // MARKER: MustLoad
	_d := _res.data.(*MustLoadOut)
	return _d.Obj, _res.err
}

// BulkLoadIn are the input arguments of BulkLoad.
type BulkLoadIn struct { // MARKER: BulkLoad
	ObjKeys []PersonKey `json:"objKeys,omitzero"`
}

// BulkLoadOut are the output arguments of BulkLoad.
type BulkLoadOut struct { // MARKER: BulkLoad
	Objs []*Person `json:"objs,omitzero"`
}

// BulkLoadResponse packs the response of BulkLoad.
type BulkLoadResponse multicastResponse // MARKER: BulkLoad

// Get unpacks the return arguments of BulkLoad.
func (_res *BulkLoadResponse) Get() (objs []*Person, err error) { // MARKER: BulkLoad
	_d := _res.data.(*BulkLoadOut)
	return _d.Objs, _res.err
}

// BulkDeleteIn are the input arguments of BulkDelete.
type BulkDeleteIn struct { // MARKER: BulkDelete
	ObjKeys []PersonKey `json:"objKeys,omitzero"`
}

// BulkDeleteOut are the output arguments of BulkDelete.
type BulkDeleteOut struct { // MARKER: BulkDelete
	DeletedKeys []PersonKey `json:"deletedKeys,omitzero"`
}

// BulkDeleteResponse packs the response of BulkDelete.
type BulkDeleteResponse multicastResponse // MARKER: BulkDelete

// Get unpacks the return arguments of BulkDelete.
func (_res *BulkDeleteResponse) Get() (deletedKeys []PersonKey, err error) { // MARKER: BulkDelete
	_d := _res.data.(*BulkDeleteOut)
	return _d.DeletedKeys, _res.err
}

// BulkCreateIn are the input arguments of BulkCreate.
type BulkCreateIn struct { // MARKER: BulkCreate
	Objs []*Person `json:"objs,omitzero"`
}

// BulkCreateOut are the output arguments of BulkCreate.
type BulkCreateOut struct { // MARKER: BulkCreate
	ObjKeys []PersonKey `json:"objKeys,omitzero"`
}

// BulkCreateResponse packs the response of BulkCreate.
type BulkCreateResponse multicastResponse // MARKER: BulkCreate

// Get unpacks the return arguments of BulkCreate.
func (_res *BulkCreateResponse) Get() (objKeys []PersonKey, err error) { // MARKER: BulkCreate
	_d := _res.data.(*BulkCreateOut)
	return _d.ObjKeys, _res.err
}

// BulkStoreIn are the input arguments of BulkStore.
type BulkStoreIn struct { // MARKER: BulkStore
	Objs []*Person `json:"objs,omitzero"`
}

// BulkStoreOut are the output arguments of BulkStore.
type BulkStoreOut struct { // MARKER: BulkStore
	StoredKeys []PersonKey `json:"storedKeys,omitzero"`
}

// BulkStoreResponse packs the response of BulkStore.
type BulkStoreResponse multicastResponse // MARKER: BulkStore

// Get unpacks the return arguments of BulkStore.
func (_res *BulkStoreResponse) Get() (storedKeys []PersonKey, err error) { // MARKER: BulkStore
	_d := _res.data.(*BulkStoreOut)
	return _d.StoredKeys, _res.err
}

// BulkReviseIn are the input arguments of BulkRevise.
type BulkReviseIn struct { // MARKER: BulkRevise
	Objs []*Person `json:"objs,omitzero"`
}

// BulkReviseOut are the output arguments of BulkRevise.
type BulkReviseOut struct { // MARKER: BulkRevise
	RevisedKeys []PersonKey `json:"revisedKeys,omitzero"`
}

// BulkReviseResponse packs the response of BulkRevise.
type BulkReviseResponse multicastResponse // MARKER: BulkRevise

// Get unpacks the return arguments of BulkRevise.
func (_res *BulkReviseResponse) Get() (revisedKeys []PersonKey, err error) { // MARKER: BulkRevise
	_d := _res.data.(*BulkReviseOut)
	return _d.RevisedKeys, _res.err
}

// PurgeIn are the input arguments of Purge.
type PurgeIn struct { // MARKER: Purge
	Query Query `json:"query,omitzero"`
}

// PurgeOut are the output arguments of Purge.
type PurgeOut struct { // MARKER: Purge
	DeletedKeys []PersonKey `json:"deletedKeys,omitzero"`
}

// PurgeResponse packs the response of Purge.
type PurgeResponse multicastResponse // MARKER: Purge

// Get unpacks the return arguments of Purge.
func (_res *PurgeResponse) Get() (deletedKeys []PersonKey, err error) { // MARKER: Purge
	_d := _res.data.(*PurgeOut)
	return _d.DeletedKeys, _res.err
}

// CountIn are the input arguments of Count.
type CountIn struct { // MARKER: Count
	Query Query `json:"query,omitzero"`
}

// CountOut are the output arguments of Count.
type CountOut struct { // MARKER: Count
	Count int `json:"count,omitzero"`
}

// CountResponse packs the response of Count.
type CountResponse multicastResponse // MARKER: Count

// Get unpacks the return arguments of Count.
func (_res *CountResponse) Get() (count int, err error) { // MARKER: Count
	_d := _res.data.(*CountOut)
	return _d.Count, _res.err
}

// CreateRESTIn are the input arguments of CreateREST.
type CreateRESTIn struct { // MARKER: CreateREST
	HTTPRequestBody *Person `json:"-"`
}

// CreateRESTOut are the output arguments of CreateREST.
type CreateRESTOut struct { // MARKER: CreateREST
	ObjKey         PersonKey `json:"objKey,omitzero"`
	HTTPStatusCode int       `json:"-"`
}

// CreateRESTResponse packs the response of CreateREST.
type CreateRESTResponse multicastResponse // MARKER: CreateREST

// Get unpacks the return arguments of CreateREST.
func (_res *CreateRESTResponse) Get() (objKey PersonKey, httpStatusCode int, err error) { // MARKER: CreateREST
	_d := _res.data.(*CreateRESTOut)
	return _d.ObjKey, _d.HTTPStatusCode, _res.err
}

// StoreRESTIn are the input arguments of StoreREST.
type StoreRESTIn struct { // MARKER: StoreREST
	Key             PersonKey `json:"key,omitzero"`
	HTTPRequestBody *Person   `json:"-"`
}

// StoreRESTOut are the output arguments of StoreREST.
type StoreRESTOut struct { // MARKER: StoreREST
	HTTPStatusCode int `json:"-"`
}

// StoreRESTResponse packs the response of StoreREST.
type StoreRESTResponse multicastResponse // MARKER: StoreREST

// Get unpacks the return arguments of StoreREST.
func (_res *StoreRESTResponse) Get() (httpStatusCode int, err error) { // MARKER: StoreREST
	_d := _res.data.(*StoreRESTOut)
	return _d.HTTPStatusCode, _res.err
}

// DeleteRESTIn are the input arguments of DeleteREST.
type DeleteRESTIn struct { // MARKER: DeleteREST
	Key PersonKey `json:"key,omitzero"`
}

// DeleteRESTOut are the output arguments of DeleteREST.
type DeleteRESTOut struct { // MARKER: DeleteREST
	HTTPStatusCode int `json:"-"`
}

// DeleteRESTResponse packs the response of DeleteREST.
type DeleteRESTResponse multicastResponse // MARKER: DeleteREST

// Get unpacks the return arguments of DeleteREST.
func (_res *DeleteRESTResponse) Get() (httpStatusCode int, err error) { // MARKER: DeleteREST
	_d := _res.data.(*DeleteRESTOut)
	return _d.HTTPStatusCode, _res.err
}

// LoadRESTIn are the input arguments of LoadREST.
type LoadRESTIn struct { // MARKER: LoadREST
	Key PersonKey `json:"key,omitzero"`
}

// LoadRESTOut are the output arguments of LoadREST.
type LoadRESTOut struct { // MARKER: LoadREST
	HTTPResponseBody *Person `json:"-"`
	HTTPStatusCode   int     `json:"-"`
}

// LoadRESTResponse packs the response of LoadREST.
type LoadRESTResponse multicastResponse // MARKER: LoadREST

// Get unpacks the return arguments of LoadREST.
func (_res *LoadRESTResponse) Get() (httpResponseBody *Person, httpStatusCode int, err error) { // MARKER: LoadREST
	_d := _res.data.(*LoadRESTOut)
	return _d.HTTPResponseBody, _d.HTTPStatusCode, _res.err
}

// ListRESTIn are the input arguments of ListREST.
type ListRESTIn struct { // MARKER: ListREST
	Q Query `json:"q,omitzero"`
}

// ListRESTOut are the output arguments of ListREST.
type ListRESTOut struct { // MARKER: ListREST
	HTTPResponseBody []*Person `json:"-"`
	HTTPStatusCode   int       `json:"-"`
}

// ListRESTResponse packs the response of ListREST.
type ListRESTResponse multicastResponse // MARKER: ListREST

// Get unpacks the return arguments of ListREST.
func (_res *ListRESTResponse) Get() (httpResponseBody []*Person, httpStatusCode int, err error) { // MARKER: ListREST
	_d := _res.data.(*ListRESTOut)
	return _d.HTTPResponseBody, _d.HTTPStatusCode, _res.err
}

// --- Multicast Client Methods ---

/*
Create creates a new person, returning its key.
*/
func (_c MulticastClient) Create(ctx context.Context, obj *Person) iter.Seq[*CreateResponse] { // MARKER: Create
	_in := CreateIn{Obj: obj}
	_out := CreateOut{}
	_queue := marshalPublish(ctx, _c.svc, _c.opts, _c.host, Create.Method, Create.Route, &_in, &_out)
	return func(yield func(*CreateResponse) bool) {
		for _r := range _queue {
			_clone := _out
			_r.data = &_clone
			if !yield((*CreateResponse)(_r)) {
				return
			}
		}
	}
}

/*
Store updates the person.
*/
func (_c MulticastClient) Store(ctx context.Context, obj *Person) iter.Seq[*StoreResponse] { // MARKER: Store
	_in := StoreIn{Obj: obj}
	_out := StoreOut{}
	_queue := marshalPublish(ctx, _c.svc, _c.opts, _c.host, Store.Method, Store.Route, &_in, &_out)
	return func(yield func(*StoreResponse) bool) {
		for _r := range _queue {
			_clone := _out
			_r.data = &_clone
			if !yield((*StoreResponse)(_r)) {
				return
			}
		}
	}
}

/*
MustStore updates the person, erroring if not found.
*/
func (_c MulticastClient) MustStore(ctx context.Context, obj *Person) iter.Seq[*MustStoreResponse] { // MARKER: MustStore
	_in := MustStoreIn{Obj: obj}
	_out := MustStoreOut{}
	_queue := marshalPublish(ctx, _c.svc, _c.opts, _c.host, MustStore.Method, MustStore.Route, &_in, &_out)
	return func(yield func(*MustStoreResponse) bool) {
		for _r := range _queue {
			_clone := _out
			_r.data = &_clone
			if !yield((*MustStoreResponse)(_r)) {
				return
			}
		}
	}
}

/*
Revise updates the person only if the revision matches.
*/
func (_c MulticastClient) Revise(ctx context.Context, obj *Person) iter.Seq[*ReviseResponse] { // MARKER: Revise
	_in := ReviseIn{Obj: obj}
	_out := ReviseOut{}
	_queue := marshalPublish(ctx, _c.svc, _c.opts, _c.host, Revise.Method, Revise.Route, &_in, &_out)
	return func(yield func(*ReviseResponse) bool) {
		for _r := range _queue {
			_clone := _out
			_r.data = &_clone
			if !yield((*ReviseResponse)(_r)) {
				return
			}
		}
	}
}

/*
MustRevise updates the person only if the revision matches, erroring on conflict.
*/
func (_c MulticastClient) MustRevise(ctx context.Context, obj *Person) iter.Seq[*MustReviseResponse] { // MARKER: MustRevise
	_in := MustReviseIn{Obj: obj}
	_out := MustReviseOut{}
	_queue := marshalPublish(ctx, _c.svc, _c.opts, _c.host, MustRevise.Method, MustRevise.Route, &_in, &_out)
	return func(yield func(*MustReviseResponse) bool) {
		for _r := range _queue {
			_clone := _out
			_r.data = &_clone
			if !yield((*MustReviseResponse)(_r)) {
				return
			}
		}
	}
}

/*
Delete deletes the person.
*/
func (_c MulticastClient) Delete(ctx context.Context, objKey PersonKey) iter.Seq[*DeleteResponse] { // MARKER: Delete
	_in := DeleteIn{ObjKey: objKey}
	_out := DeleteOut{}
	_queue := marshalPublish(ctx, _c.svc, _c.opts, _c.host, Delete.Method, Delete.Route, &_in, &_out)
	return func(yield func(*DeleteResponse) bool) {
		for _r := range _queue {
			_clone := _out
			_r.data = &_clone
			if !yield((*DeleteResponse)(_r)) {
				return
			}
		}
	}
}

/*
MustDelete deletes the person, erroring if not found.
*/
func (_c MulticastClient) MustDelete(ctx context.Context, objKey PersonKey) iter.Seq[*MustDeleteResponse] { // MARKER: MustDelete
	_in := MustDeleteIn{ObjKey: objKey}
	_out := MustDeleteOut{}
	_queue := marshalPublish(ctx, _c.svc, _c.opts, _c.host, MustDelete.Method, MustDelete.Route, &_in, &_out)
	return func(yield func(*MustDeleteResponse) bool) {
		for _r := range _queue {
			_clone := _out
			_r.data = &_clone
			if !yield((*MustDeleteResponse)(_r)) {
				return
			}
		}
	}
}

/*
List returns the persons matching the query, and the total count of matches regardless of the limit.
*/
func (_c MulticastClient) List(ctx context.Context, query Query) iter.Seq[*ListResponse] { // MARKER: List
	_in := ListIn{Query: query}
	_out := ListOut{}
	_queue := marshalPublish(ctx, _c.svc, _c.opts, _c.host, List.Method, List.Route, &_in, &_out)
	return func(yield func(*ListResponse) bool) {
		for _r := range _queue {
			_clone := _out
			_r.data = &_clone
			if !yield((*ListResponse)(_r)) {
				return
			}
		}
	}
}

/*
Lookup returns the single person matching the query.
*/
func (_c MulticastClient) Lookup(ctx context.Context, query Query) iter.Seq[*LookupResponse] { // MARKER: Lookup
	_in := LookupIn{Query: query}
	_out := LookupOut{}
	_queue := marshalPublish(ctx, _c.svc, _c.opts, _c.host, Lookup.Method, Lookup.Route, &_in, &_out)
	return func(yield func(*LookupResponse) bool) {
		for _r := range _queue {
			_clone := _out
			_r.data = &_clone
			if !yield((*LookupResponse)(_r)) {
				return
			}
		}
	}
}

/*
MustLookup returns the single person matching the query. It errors unless exactly one person matches the query.
*/
func (_c MulticastClient) MustLookup(ctx context.Context, query Query) iter.Seq[*MustLookupResponse] { // MARKER: MustLookup
	_in := MustLookupIn{Query: query}
	_out := MustLookupOut{}
	_queue := marshalPublish(ctx, _c.svc, _c.opts, _c.host, MustLookup.Method, MustLookup.Route, &_in, &_out)
	return func(yield func(*MustLookupResponse) bool) {
		for _r := range _queue {
			_clone := _out
			_r.data = &_clone
			if !yield((*MustLookupResponse)(_r)) {
				return
			}
		}
	}
}

/*
Load returns the person associated with the key.
*/
func (_c MulticastClient) Load(ctx context.Context, objKey PersonKey) iter.Seq[*LoadResponse] { // MARKER: Load
	_in := LoadIn{ObjKey: objKey}
	_out := LoadOut{}
	_queue := marshalPublish(ctx, _c.svc, _c.opts, _c.host, Load.Method, Load.Route, &_in, &_out)
	return func(yield func(*LoadResponse) bool) {
		for _r := range _queue {
			_clone := _out
			_r.data = &_clone
			if !yield((*LoadResponse)(_r)) {
				return
			}
		}
	}
}

/*
MustLoad returns the person associated with the key, erroring if not found.
*/
func (_c MulticastClient) MustLoad(ctx context.Context, objKey PersonKey) iter.Seq[*MustLoadResponse] { // MARKER: MustLoad
	_in := MustLoadIn{ObjKey: objKey}
	_out := MustLoadOut{}
	_queue := marshalPublish(ctx, _c.svc, _c.opts, _c.host, MustLoad.Method, MustLoad.Route, &_in, &_out)
	return func(yield func(*MustLoadResponse) bool) {
		for _r := range _queue {
			_clone := _out
			_r.data = &_clone
			if !yield((*MustLoadResponse)(_r)) {
				return
			}
		}
	}
}

/*
BulkLoad returns the persons matching the keys.
*/
func (_c MulticastClient) BulkLoad(ctx context.Context, objKeys []PersonKey) iter.Seq[*BulkLoadResponse] { // MARKER: BulkLoad
	_in := BulkLoadIn{ObjKeys: objKeys}
	_out := BulkLoadOut{}
	_queue := marshalPublish(ctx, _c.svc, _c.opts, _c.host, BulkLoad.Method, BulkLoad.Route, &_in, &_out)
	return func(yield func(*BulkLoadResponse) bool) {
		for _r := range _queue {
			_clone := _out
			_r.data = &_clone
			if !yield((*BulkLoadResponse)(_r)) {
				return
			}
		}
	}
}

/*
BulkDelete deletes the persons matching the keys, returning the keys of the deleted persons.
*/
func (_c MulticastClient) BulkDelete(ctx context.Context, objKeys []PersonKey) iter.Seq[*BulkDeleteResponse] { // MARKER: BulkDelete
	_in := BulkDeleteIn{ObjKeys: objKeys}
	_out := BulkDeleteOut{}
	_queue := marshalPublish(ctx, _c.svc, _c.opts, _c.host, BulkDelete.Method, BulkDelete.Route, &_in, &_out)
	return func(yield func(*BulkDeleteResponse) bool) {
		for _r := range _queue {
			_clone := _out
			_r.data = &_clone
			if !yield((*BulkDeleteResponse)(_r)) {
				return
			}
		}
	}
}

/*
BulkCreate creates multiple persons, returning their keys.
*/
func (_c MulticastClient) BulkCreate(ctx context.Context, objs []*Person) iter.Seq[*BulkCreateResponse] { // MARKER: BulkCreate
	_in := BulkCreateIn{Objs: objs}
	_out := BulkCreateOut{}
	_queue := marshalPublish(ctx, _c.svc, _c.opts, _c.host, BulkCreate.Method, BulkCreate.Route, &_in, &_out)
	return func(yield func(*BulkCreateResponse) bool) {
		for _r := range _queue {
			_clone := _out
			_r.data = &_clone
			if !yield((*BulkCreateResponse)(_r)) {
				return
			}
		}
	}
}

/*
BulkStore updates multiple persons, returning the keys of the stored persons.
*/
func (_c MulticastClient) BulkStore(ctx context.Context, objs []*Person) iter.Seq[*BulkStoreResponse] { // MARKER: BulkStore
	_in := BulkStoreIn{Objs: objs}
	_out := BulkStoreOut{}
	_queue := marshalPublish(ctx, _c.svc, _c.opts, _c.host, BulkStore.Method, BulkStore.Route, &_in, &_out)
	return func(yield func(*BulkStoreResponse) bool) {
		for _r := range _queue {
			_clone := _out
			_r.data = &_clone
			if !yield((*BulkStoreResponse)(_r)) {
				return
			}
		}
	}
}

/*
BulkRevise updates multiple persons only if the revisions match, returning the keys of the revised persons.
*/
func (_c MulticastClient) BulkRevise(ctx context.Context, objs []*Person) iter.Seq[*BulkReviseResponse] { // MARKER: BulkRevise
	_in := BulkReviseIn{Objs: objs}
	_out := BulkReviseOut{}
	_queue := marshalPublish(ctx, _c.svc, _c.opts, _c.host, BulkRevise.Method, BulkRevise.Route, &_in, &_out)
	return func(yield func(*BulkReviseResponse) bool) {
		for _r := range _queue {
			_clone := _out
			_r.data = &_clone
			if !yield((*BulkReviseResponse)(_r)) {
				return
			}
		}
	}
}

/*
Purge deletes all persons matching the query, returning the keys of the deleted persons.
*/
func (_c MulticastClient) Purge(ctx context.Context, query Query) iter.Seq[*PurgeResponse] { // MARKER: Purge
	_in := PurgeIn{Query: query}
	_out := PurgeOut{}
	_queue := marshalPublish(ctx, _c.svc, _c.opts, _c.host, Purge.Method, Purge.Route, &_in, &_out)
	return func(yield func(*PurgeResponse) bool) {
		for _r := range _queue {
			_clone := _out
			_r.data = &_clone
			if !yield((*PurgeResponse)(_r)) {
				return
			}
		}
	}
}

/*
Count returns the number of persons matching the query.
*/
func (_c MulticastClient) Count(ctx context.Context, query Query) iter.Seq[*CountResponse] { // MARKER: Count
	_in := CountIn{Query: query}
	_out := CountOut{}
	_queue := marshalPublish(ctx, _c.svc, _c.opts, _c.host, Count.Method, Count.Route, &_in, &_out)
	return func(yield func(*CountResponse) bool) {
		for _r := range _queue {
			_clone := _out
			_r.data = &_clone
			if !yield((*CountResponse)(_r)) {
				return
			}
		}
	}
}

/*
CreateREST creates a new person via REST, returning its key.
*/
func (_c MulticastClient) CreateREST(ctx context.Context, httpRequestBody *Person) iter.Seq[*CreateRESTResponse] { // MARKER: CreateREST
	_in := CreateRESTIn{HTTPRequestBody: httpRequestBody}
	_out := CreateRESTOut{}
	_queue := marshalPublish(ctx, _c.svc, _c.opts, _c.host, CreateREST.Method, CreateREST.Route, &_in, &_out)
	return func(yield func(*CreateRESTResponse) bool) {
		for _r := range _queue {
			_clone := _out
			_r.data = &_clone
			if !yield((*CreateRESTResponse)(_r)) {
				return
			}
		}
	}
}

/*
StoreREST updates an existing person via REST.
*/
func (_c MulticastClient) StoreREST(ctx context.Context, key PersonKey, httpRequestBody *Person) iter.Seq[*StoreRESTResponse] { // MARKER: StoreREST
	_in := StoreRESTIn{Key: key, HTTPRequestBody: httpRequestBody}
	_out := StoreRESTOut{}
	_queue := marshalPublish(ctx, _c.svc, _c.opts, _c.host, StoreREST.Method, StoreREST.Route, &_in, &_out)
	return func(yield func(*StoreRESTResponse) bool) {
		for _r := range _queue {
			_clone := _out
			_r.data = &_clone
			if !yield((*StoreRESTResponse)(_r)) {
				return
			}
		}
	}
}

/*
DeleteREST deletes an existing person via REST.
*/
func (_c MulticastClient) DeleteREST(ctx context.Context, key PersonKey) iter.Seq[*DeleteRESTResponse] { // MARKER: DeleteREST
	_in := DeleteRESTIn{Key: key}
	_out := DeleteRESTOut{}
	_queue := marshalPublish(ctx, _c.svc, _c.opts, _c.host, DeleteREST.Method, DeleteREST.Route, &_in, &_out)
	return func(yield func(*DeleteRESTResponse) bool) {
		for _r := range _queue {
			_clone := _out
			_r.data = &_clone
			if !yield((*DeleteRESTResponse)(_r)) {
				return
			}
		}
	}
}

/*
LoadREST loads a person by key via REST.
*/
func (_c MulticastClient) LoadREST(ctx context.Context, key PersonKey) iter.Seq[*LoadRESTResponse] { // MARKER: LoadREST
	_in := LoadRESTIn{Key: key}
	_out := LoadRESTOut{}
	_queue := marshalPublish(ctx, _c.svc, _c.opts, _c.host, LoadREST.Method, LoadREST.Route, &_in, &_out)
	return func(yield func(*LoadRESTResponse) bool) {
		for _r := range _queue {
			_clone := _out
			_r.data = &_clone
			if !yield((*LoadRESTResponse)(_r)) {
				return
			}
		}
	}
}

/*
ListREST lists persons matching the query via REST.
*/
func (_c MulticastClient) ListREST(ctx context.Context, q Query) iter.Seq[*ListRESTResponse] { // MARKER: ListREST
	_in := ListRESTIn{Q: q}
	_out := ListRESTOut{}
	_queue := marshalPublish(ctx, _c.svc, _c.opts, _c.host, ListREST.Method, ListREST.Route, &_in, &_out)
	return func(yield func(*ListRESTResponse) bool) {
		for _r := range _queue {
			_clone := _out
			_r.data = &_clone
			if !yield((*ListRESTResponse)(_r)) {
				return
			}
		}
	}
}

/*
WebUI provides a form for making web requests to the CRUD endpoints.

If a URL is provided, it is resolved relative to the URL of the endpoint.
If the body is of type io.Reader, []byte or string, it is serialized in binary form.
If it is of type url.Values, it is serialized as form data. All other types are serialized as JSON.
*/
func (_c MulticastClient) WebUI(ctx context.Context, method string, relativeURL string, body any) iter.Seq[*pub.Response] { // MARKER: WebUI
	if method == "" {
		method = "POST"
	}
	return _c.svc.Publish(
		ctx,
		pub.Method(method),
		pub.URL(httpx.JoinHostAndPath(_c.host, RouteOfWebUI)),
		pub.RelativeURL(relativeURL),
		pub.Body(body),
		pub.Options(_c.opts...),
	)
}

// --- Unicast Client Methods ---

/*
Create creates a new person, returning its key.
*/
func (_c Client) Create(ctx context.Context, obj *Person) (objKey PersonKey, err error) { // MARKER: Create
	_in := CreateIn{Obj: obj}
	_out := CreateOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, Create.Method, Create.Route, &_in, &_out)
	return _out.ObjKey, err // No trace
}

/*
Store updates the person.
*/
func (_c Client) Store(ctx context.Context, obj *Person) (stored bool, err error) { // MARKER: Store
	_in := StoreIn{Obj: obj}
	_out := StoreOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, Store.Method, Store.Route, &_in, &_out)
	return _out.Stored, err // No trace
}

/*
MustStore updates the person, erroring if not found.
*/
func (_c Client) MustStore(ctx context.Context, obj *Person) (err error) { // MARKER: MustStore
	_in := MustStoreIn{Obj: obj}
	_out := MustStoreOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, MustStore.Method, MustStore.Route, &_in, &_out)
	return err // No trace
}

/*
Revise updates the person only if the revision matches.
*/
func (_c Client) Revise(ctx context.Context, obj *Person) (revised bool, err error) { // MARKER: Revise
	_in := ReviseIn{Obj: obj}
	_out := ReviseOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, Revise.Method, Revise.Route, &_in, &_out)
	return _out.Revised, err // No trace
}

/*
MustRevise updates the person only if the revision matches, erroring on conflict.
*/
func (_c Client) MustRevise(ctx context.Context, obj *Person) (err error) { // MARKER: MustRevise
	_in := MustReviseIn{Obj: obj}
	_out := MustReviseOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, MustRevise.Method, MustRevise.Route, &_in, &_out)
	return err // No trace
}

/*
Delete deletes the person.
*/
func (_c Client) Delete(ctx context.Context, objKey PersonKey) (deleted bool, err error) { // MARKER: Delete
	_in := DeleteIn{ObjKey: objKey}
	_out := DeleteOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, Delete.Method, Delete.Route, &_in, &_out)
	return _out.Deleted, err // No trace
}

/*
MustDelete deletes the person, erroring if not found.
*/
func (_c Client) MustDelete(ctx context.Context, objKey PersonKey) (err error) { // MARKER: MustDelete
	_in := MustDeleteIn{ObjKey: objKey}
	_out := MustDeleteOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, MustDelete.Method, MustDelete.Route, &_in, &_out)
	return err // No trace
}

/*
List returns the persons matching the query, and the total count of matches regardless of the limit.
*/
func (_c Client) List(ctx context.Context, query Query) (objs []*Person, totalCount int, err error) { // MARKER: List
	_in := ListIn{Query: query}
	_out := ListOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, List.Method, List.Route, &_in, &_out)
	return _out.Objs, _out.TotalCount, err // No trace
}

/*
Lookup returns the single person matching the query.
*/
func (_c Client) Lookup(ctx context.Context, query Query) (obj *Person, found bool, err error) { // MARKER: Lookup
	_in := LookupIn{Query: query}
	_out := LookupOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, Lookup.Method, Lookup.Route, &_in, &_out)
	return _out.Obj, _out.Found, err // No trace
}

/*
MustLookup returns the single person matching the query. It errors unless exactly one person matches the query.
*/
func (_c Client) MustLookup(ctx context.Context, query Query) (obj *Person, err error) { // MARKER: MustLookup
	_in := MustLookupIn{Query: query}
	_out := MustLookupOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, MustLookup.Method, MustLookup.Route, &_in, &_out)
	return _out.Obj, err // No trace
}

/*
Load returns the person associated with the key.
*/
func (_c Client) Load(ctx context.Context, objKey PersonKey) (obj *Person, found bool, err error) { // MARKER: Load
	_in := LoadIn{ObjKey: objKey}
	_out := LoadOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, Load.Method, Load.Route, &_in, &_out)
	return _out.Obj, _out.Found, err // No trace
}

/*
MustLoad returns the person associated with the key, erroring if not found.
*/
func (_c Client) MustLoad(ctx context.Context, objKey PersonKey) (obj *Person, err error) { // MARKER: MustLoad
	_in := MustLoadIn{ObjKey: objKey}
	_out := MustLoadOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, MustLoad.Method, MustLoad.Route, &_in, &_out)
	return _out.Obj, err // No trace
}

/*
BulkLoad returns the persons matching the keys.
*/
func (_c Client) BulkLoad(ctx context.Context, objKeys []PersonKey) (objs []*Person, err error) { // MARKER: BulkLoad
	_in := BulkLoadIn{ObjKeys: objKeys}
	_out := BulkLoadOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, BulkLoad.Method, BulkLoad.Route, &_in, &_out)
	return _out.Objs, err // No trace
}

/*
BulkDelete deletes the persons matching the keys, returning the keys of the deleted persons.
*/
func (_c Client) BulkDelete(ctx context.Context, objKeys []PersonKey) (deletedKeys []PersonKey, err error) { // MARKER: BulkDelete
	_in := BulkDeleteIn{ObjKeys: objKeys}
	_out := BulkDeleteOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, BulkDelete.Method, BulkDelete.Route, &_in, &_out)
	return _out.DeletedKeys, err // No trace
}

/*
BulkCreate creates multiple persons, returning their keys.
*/
func (_c Client) BulkCreate(ctx context.Context, objs []*Person) (objKeys []PersonKey, err error) { // MARKER: BulkCreate
	_in := BulkCreateIn{Objs: objs}
	_out := BulkCreateOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, BulkCreate.Method, BulkCreate.Route, &_in, &_out)
	return _out.ObjKeys, err // No trace
}

/*
BulkStore updates multiple persons, returning the keys of the stored persons.
*/
func (_c Client) BulkStore(ctx context.Context, objs []*Person) (storedKeys []PersonKey, err error) { // MARKER: BulkStore
	_in := BulkStoreIn{Objs: objs}
	_out := BulkStoreOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, BulkStore.Method, BulkStore.Route, &_in, &_out)
	return _out.StoredKeys, err // No trace
}

/*
BulkRevise updates multiple persons only if the revisions match, returning the keys of the revised persons.
*/
func (_c Client) BulkRevise(ctx context.Context, objs []*Person) (revisedKeys []PersonKey, err error) { // MARKER: BulkRevise
	_in := BulkReviseIn{Objs: objs}
	_out := BulkReviseOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, BulkRevise.Method, BulkRevise.Route, &_in, &_out)
	return _out.RevisedKeys, err // No trace
}

/*
Purge deletes all persons matching the query, returning the keys of the deleted persons.
*/
func (_c Client) Purge(ctx context.Context, query Query) (deletedKeys []PersonKey, err error) { // MARKER: Purge
	_in := PurgeIn{Query: query}
	_out := PurgeOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, Purge.Method, Purge.Route, &_in, &_out)
	return _out.DeletedKeys, err // No trace
}

/*
Count returns the number of persons matching the query.
*/
func (_c Client) Count(ctx context.Context, query Query) (count int, err error) { // MARKER: Count
	_in := CountIn{Query: query}
	_out := CountOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, Count.Method, Count.Route, &_in, &_out)
	return _out.Count, err // No trace
}

/*
CreateREST creates a new person via REST, returning its key.
*/
func (_c Client) CreateREST(ctx context.Context, httpRequestBody *Person) (objKey PersonKey, httpStatusCode int, err error) { // MARKER: CreateREST
	_in := CreateRESTIn{HTTPRequestBody: httpRequestBody}
	_out := CreateRESTOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, CreateREST.Method, CreateREST.Route, &_in, &_out)
	return _out.ObjKey, _out.HTTPStatusCode, err // No trace
}

/*
StoreREST updates an existing person via REST.
*/
func (_c Client) StoreREST(ctx context.Context, key PersonKey, httpRequestBody *Person) (httpStatusCode int, err error) { // MARKER: StoreREST
	_in := StoreRESTIn{Key: key, HTTPRequestBody: httpRequestBody}
	_out := StoreRESTOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, StoreREST.Method, StoreREST.Route, &_in, &_out)
	return _out.HTTPStatusCode, err // No trace
}

/*
DeleteREST deletes an existing person via REST.
*/
func (_c Client) DeleteREST(ctx context.Context, key PersonKey) (httpStatusCode int, err error) { // MARKER: DeleteREST
	_in := DeleteRESTIn{Key: key}
	_out := DeleteRESTOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, DeleteREST.Method, DeleteREST.Route, &_in, &_out)
	return _out.HTTPStatusCode, err // No trace
}

/*
LoadREST loads a person by key via REST.
*/
func (_c Client) LoadREST(ctx context.Context, key PersonKey) (httpResponseBody *Person, httpStatusCode int, err error) { // MARKER: LoadREST
	_in := LoadRESTIn{Key: key}
	_out := LoadRESTOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, LoadREST.Method, LoadREST.Route, &_in, &_out)
	return _out.HTTPResponseBody, _out.HTTPStatusCode, err // No trace
}

/*
ListREST lists persons matching the query via REST.
*/
func (_c Client) ListREST(ctx context.Context, q Query) (httpResponseBody []*Person, httpStatusCode int, err error) { // MARKER: ListREST
	_in := ListRESTIn{Q: q}
	_out := ListRESTOut{}
	err = marshalRequest(ctx, _c.svc, _c.opts, _c.host, ListREST.Method, ListREST.Route, &_in, &_out)
	return _out.HTTPResponseBody, _out.HTTPStatusCode, err // No trace
}

/*
WebUI provides a form for making web requests to the CRUD endpoints.

If a URL is provided, it is resolved relative to the URL of the endpoint.
If the body is of type io.Reader, []byte or string, it is serialized in binary form.
If it is of type url.Values, it is serialized as form data. All other types are serialized as JSON.
*/
func (_c Client) WebUI(ctx context.Context, method string, relativeURL string, body any) (res *http.Response, err error) { // MARKER: WebUI
	if method == "" {
		method = "POST"
	}
	return _c.svc.Request(
		ctx,
		pub.Method(method),
		pub.URL(httpx.JoinHostAndPath(_c.host, RouteOfWebUI)),
		pub.RelativeURL(relativeURL),
		pub.Body(body),
		pub.Options(_c.opts...),
	)
}

// marshalFunction handled marshaling for functional endpoints.
func marshalFunction(w http.ResponseWriter, r *http.Request, route string, in any, out any, execute func(in any, out any) error) error {
	err := httpx.ReadInputPayload(r, route, in)
	if err != nil {
		return errors.Trace(err)
	}
	err = execute(in, out)
	if err != nil {
		return err // No trace
	}
	err = httpx.WriteOutputPayload(w, out)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

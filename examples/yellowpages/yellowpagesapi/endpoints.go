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
	"time"

	"github.com/microbus-io/fabric/httpx"
)

// Hostname is the default hostname of the microservice.
const Hostname = "yellowpages.example"

// Def is the routing identity of an endpoint exposed by this microservice.
type Def struct {
	Method string
	Route  string
}

// URL is the full URL of the endpoint, joined with the package-level Hostname.
func (d Def) URL() string {
	return httpx.JoinHostAndPath(Hostname, d.Route)
}

// CreateIn are the input arguments of Create.
type CreateIn struct { // MARKER: Create
	Obj *Person `json:"obj,omitzero"`
}

// CreateOut are the output arguments of Create.
type CreateOut struct { // MARKER: Create
	ObjKey PersonKey `json:"objKey,omitzero"`
}

// StoreIn are the input arguments of Store.
type StoreIn struct { // MARKER: Store
	Obj *Person `json:"obj,omitzero"`
}

// StoreOut are the output arguments of Store.
type StoreOut struct { // MARKER: Store
	Stored bool `json:"stored,omitzero"`
}

// MustStoreIn are the input arguments of MustStore.
type MustStoreIn struct { // MARKER: MustStore
	Obj *Person `json:"obj,omitzero"`
}

// MustStoreOut are the output arguments of MustStore.
type MustStoreOut struct { // MARKER: MustStore
}

// ReviseIn are the input arguments of Revise.
type ReviseIn struct { // MARKER: Revise
	Obj *Person `json:"obj,omitzero"`
}

// ReviseOut are the output arguments of Revise.
type ReviseOut struct { // MARKER: Revise
	Revised bool `json:"revised,omitzero"`
}

// MustReviseIn are the input arguments of MustRevise.
type MustReviseIn struct { // MARKER: MustRevise
	Obj *Person `json:"obj,omitzero"`
}

// MustReviseOut are the output arguments of MustRevise.
type MustReviseOut struct { // MARKER: MustRevise
}

// DeleteIn are the input arguments of Delete.
type DeleteIn struct { // MARKER: Delete
	ObjKey PersonKey `json:"objKey,omitzero"`
}

// DeleteOut are the output arguments of Delete.
type DeleteOut struct { // MARKER: Delete
	Deleted bool `json:"deleted,omitzero"`
}

// MustDeleteIn are the input arguments of MustDelete.
type MustDeleteIn struct { // MARKER: MustDelete
	ObjKey PersonKey `json:"objKey,omitzero"`
}

// MustDeleteOut are the output arguments of MustDelete.
type MustDeleteOut struct { // MARKER: MustDelete
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

// LookupIn are the input arguments of Lookup.
type LookupIn struct { // MARKER: Lookup
	Query Query `json:"query,omitzero"`
}

// LookupOut are the output arguments of Lookup.
type LookupOut struct { // MARKER: Lookup
	Obj   *Person `json:"obj,omitzero"`
	Found bool    `json:"found,omitzero"`
}

// MustLookupIn are the input arguments of MustLookup.
type MustLookupIn struct { // MARKER: MustLookup
	Query Query `json:"query,omitzero"`
}

// MustLookupOut are the output arguments of MustLookup.
type MustLookupOut struct { // MARKER: MustLookup
	Obj *Person `json:"obj,omitzero"`
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

// MustLoadIn are the input arguments of MustLoad.
type MustLoadIn struct { // MARKER: MustLoad
	ObjKey PersonKey `json:"objKey,omitzero"`
}

// MustLoadOut are the output arguments of MustLoad.
type MustLoadOut struct { // MARKER: MustLoad
	Obj *Person `json:"obj,omitzero"`
}

// BulkLoadIn are the input arguments of BulkLoad.
type BulkLoadIn struct { // MARKER: BulkLoad
	ObjKeys []PersonKey `json:"objKeys,omitzero"`
}

// BulkLoadOut are the output arguments of BulkLoad.
type BulkLoadOut struct { // MARKER: BulkLoad
	Objs []*Person `json:"objs,omitzero"`
}

// BulkDeleteIn are the input arguments of BulkDelete.
type BulkDeleteIn struct { // MARKER: BulkDelete
	ObjKeys []PersonKey `json:"objKeys,omitzero"`
}

// BulkDeleteOut are the output arguments of BulkDelete.
type BulkDeleteOut struct { // MARKER: BulkDelete
	DeletedKeys []PersonKey `json:"deletedKeys,omitzero"`
}

// BulkCreateIn are the input arguments of BulkCreate.
type BulkCreateIn struct { // MARKER: BulkCreate
	Objs []*Person `json:"objs,omitzero"`
}

// BulkCreateOut are the output arguments of BulkCreate.
type BulkCreateOut struct { // MARKER: BulkCreate
	ObjKeys []PersonKey `json:"objKeys,omitzero"`
}

// BulkStoreIn are the input arguments of BulkStore.
type BulkStoreIn struct { // MARKER: BulkStore
	Objs []*Person `json:"objs,omitzero"`
}

// BulkStoreOut are the output arguments of BulkStore.
type BulkStoreOut struct { // MARKER: BulkStore
	StoredKeys []PersonKey `json:"storedKeys,omitzero"`
}

// BulkReviseIn are the input arguments of BulkRevise.
type BulkReviseIn struct { // MARKER: BulkRevise
	Objs []*Person `json:"objs,omitzero"`
}

// BulkReviseOut are the output arguments of BulkRevise.
type BulkReviseOut struct { // MARKER: BulkRevise
	RevisedKeys []PersonKey `json:"revisedKeys,omitzero"`
}

// PurgeIn are the input arguments of Purge.
type PurgeIn struct { // MARKER: Purge
	Query Query `json:"query,omitzero"`
}

// PurgeOut are the output arguments of Purge.
type PurgeOut struct { // MARKER: Purge
	DeletedKeys []PersonKey `json:"deletedKeys,omitzero"`
}

// CountIn are the input arguments of Count.
type CountIn struct { // MARKER: Count
	Query Query `json:"query,omitzero"`
}

// CountOut are the output arguments of Count.
type CountOut struct { // MARKER: Count
	Count int `json:"count,omitzero"`
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

// StoreRESTIn are the input arguments of StoreREST.
type StoreRESTIn struct { // MARKER: StoreREST
	Key             PersonKey `json:"key,omitzero"`
	HTTPRequestBody *Person   `json:"-"`
}

// StoreRESTOut are the output arguments of StoreREST.
type StoreRESTOut struct { // MARKER: StoreREST
	HTTPStatusCode int `json:"-"`
}

// DeleteRESTIn are the input arguments of DeleteREST.
type DeleteRESTIn struct { // MARKER: DeleteREST
	Key PersonKey `json:"key,omitzero"`
}

// DeleteRESTOut are the output arguments of DeleteREST.
type DeleteRESTOut struct { // MARKER: DeleteREST
	HTTPStatusCode int `json:"-"`
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

// ListRESTIn are the input arguments of ListREST.
type ListRESTIn struct { // MARKER: ListREST
	Q Query `json:"q,omitzero"`
}

// ListRESTOut are the output arguments of ListREST.
type ListRESTOut struct { // MARKER: ListREST
	HTTPResponseBody []*Person `json:"-"`
	HTTPStatusCode   int       `json:"-"`
}

// TryReserveIn are the input arguments of TryReserve.
type TryReserveIn struct { // MARKER: TryReserve
	ObjKey PersonKey     `json:"objKey,omitzero"`
	Dur    time.Duration `json:"dur,omitzero"`
}

// TryReserveOut are the output arguments of TryReserve.
type TryReserveOut struct { // MARKER: TryReserve
	Reserved bool `json:"reserved,omitzero"`
}

// TryBulkReserveIn are the input arguments of TryBulkReserve.
type TryBulkReserveIn struct { // MARKER: TryBulkReserve
	ObjKeys []PersonKey   `json:"objKeys,omitzero"`
	Dur     time.Duration `json:"dur,omitzero"`
}

// TryBulkReserveOut are the output arguments of TryBulkReserve.
type TryBulkReserveOut struct { // MARKER: TryBulkReserve
	ReservedKeys []PersonKey `json:"reservedKeys,omitzero"`
}

// ReserveIn are the input arguments of Reserve.
type ReserveIn struct { // MARKER: Reserve
	ObjKey PersonKey     `json:"objKey,omitzero"`
	Dur    time.Duration `json:"dur,omitzero"`
}

// ReserveOut are the output arguments of Reserve.
type ReserveOut struct { // MARKER: Reserve
	Reserved bool `json:"reserved,omitzero"`
}

// BulkReserveIn are the input arguments of BulkReserve.
type BulkReserveIn struct { // MARKER: BulkReserve
	ObjKeys []PersonKey   `json:"objKeys,omitzero"`
	Dur     time.Duration `json:"dur,omitzero"`
}

// BulkReserveOut are the output arguments of BulkReserve.
type BulkReserveOut struct { // MARKER: BulkReserve
	ReservedKeys []PersonKey `json:"reservedKeys,omitzero"`
}

// OnPersonCreatedIn are the input arguments of OnPersonCreated.
type OnPersonCreatedIn struct { // MARKER: OnPersonCreated
	ObjKeys []PersonKey `json:"objKeys,omitzero"`
}

// OnPersonCreatedOut are the output arguments of OnPersonCreated.
type OnPersonCreatedOut struct { // MARKER: OnPersonCreated
}

// OnPersonStoredIn are the input arguments of OnPersonStored.
type OnPersonStoredIn struct { // MARKER: OnPersonStored
	ObjKeys []PersonKey `json:"objKeys,omitzero"`
}

// OnPersonStoredOut are the output arguments of OnPersonStored.
type OnPersonStoredOut struct { // MARKER: OnPersonStored
}

// OnPersonDeletedIn are the input arguments of OnPersonDeleted.
type OnPersonDeletedIn struct { // MARKER: OnPersonDeleted
	ObjKeys []PersonKey `json:"objKeys,omitzero"`
}

// OnPersonDeletedOut are the output arguments of OnPersonDeleted.
type OnPersonDeletedOut struct { // MARKER: OnPersonDeleted
}

var (
	// HINT: Insert endpoint definitions here
	Create          = Def{Method: "ANY", Route: "/create"}                 // MARKER: Create
	Store           = Def{Method: "ANY", Route: "/store"}                  // MARKER: Store
	MustStore       = Def{Method: "ANY", Route: "/must-store"}             // MARKER: MustStore
	Revise          = Def{Method: "ANY", Route: "/revise"}                 // MARKER: Revise
	MustRevise      = Def{Method: "ANY", Route: "/must-revise"}            // MARKER: MustRevise
	Delete          = Def{Method: "ANY", Route: "/delete"}                 // MARKER: Delete
	MustDelete      = Def{Method: "ANY", Route: "/must-delete"}            // MARKER: MustDelete
	List            = Def{Method: "ANY", Route: "/list"}                   // MARKER: List
	Lookup          = Def{Method: "ANY", Route: "/lookup"}                 // MARKER: Lookup
	MustLookup      = Def{Method: "ANY", Route: "/must-lookup"}            // MARKER: MustLookup
	Load            = Def{Method: "ANY", Route: "/load"}                   // MARKER: Load
	MustLoad        = Def{Method: "ANY", Route: "/must-load"}              // MARKER: MustLoad
	BulkLoad        = Def{Method: "ANY", Route: "/bulk-load"}              // MARKER: BulkLoad
	BulkDelete      = Def{Method: "ANY", Route: "/bulk-delete"}            // MARKER: BulkDelete
	BulkCreate      = Def{Method: "ANY", Route: "/bulk-create"}            // MARKER: BulkCreate
	BulkStore       = Def{Method: "ANY", Route: "/bulk-store"}             // MARKER: BulkStore
	BulkRevise      = Def{Method: "ANY", Route: "/bulk-revise"}            // MARKER: BulkRevise
	Purge           = Def{Method: "ANY", Route: "/purge"}                  // MARKER: Purge
	Count           = Def{Method: "ANY", Route: "/count"}                  // MARKER: Count
	CreateREST      = Def{Method: "POST", Route: "/persons"}               // MARKER: CreateREST
	StoreREST       = Def{Method: "PUT", Route: "/persons/{key}"}          // MARKER: StoreREST
	DeleteREST      = Def{Method: "DELETE", Route: "/persons/{key}"}       // MARKER: DeleteREST
	LoadREST        = Def{Method: "GET", Route: "/persons/{key}"}          // MARKER: LoadREST
	ListREST        = Def{Method: "GET", Route: "/persons"}                // MARKER: ListREST
	TryReserve      = Def{Method: "ANY", Route: "/try-reserve"}            // MARKER: TryReserve
	TryBulkReserve  = Def{Method: "ANY", Route: "/try-bulk-reserve"}       // MARKER: TryBulkReserve
	Reserve         = Def{Method: "ANY", Route: "/reserve"}                // MARKER: Reserve
	BulkReserve     = Def{Method: "ANY", Route: "/bulk-reserve"}           // MARKER: BulkReserve
	Demo            = Def{Method: "ANY", Route: "/demo"}                   // MARKER: Demo
	OnPersonCreated = Def{Method: "POST", Route: ":417/on-person-created"} // MARKER: OnPersonCreated
	OnPersonStored  = Def{Method: "POST", Route: ":417/on-person-stored"}  // MARKER: OnPersonStored
	OnPersonDeleted = Def{Method: "POST", Route: ":417/on-person-deleted"} // MARKER: OnPersonDeleted
)

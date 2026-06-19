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
	"github.com/microbus-io/fabric/define"
	"time"
)

// HINT: This file is the single source of truth for the microservice's API. After editing it, run
// cmd/genservice on the microservice's directory (the parent of this api package) to regenerate client.go,
// intermediate.go, mock.go, mock_test.go, and manifest.yaml. Do not hand-edit those generated files.

// Hostname is the default hostname of the microservice.
const Hostname = "yellowpages.example"

// Name is the decorative PascalCase name of the microservice.
const Name = "Person"

// Version is the major version of the microservice's public API.
const Version = 11

// Description is the human-readable summary of the microservice, surfaced in OpenAPI and discovery.
const Description = `Person persists persons in a SQL database.`

// SQLDataSourceName is the connection string of the SQL database.
var SQLDataSourceName = define.Config{ // MARKER: SQLDataSourceName
	Value:  string(""),
	Secret: true,
}

// OnPersonCreated is triggered when persons are created.
var OnPersonCreated = define.OutboundEvent{ // MARKER: OnPersonCreated
	Host: Hostname, Method: "POST", Route: ":417/on-person-created",
	In: OnPersonCreatedIn{}, Out: OnPersonCreatedOut{},
}

// OnPersonCreatedIn are the input arguments of OnPersonCreated.
type OnPersonCreatedIn struct { // MARKER: OnPersonCreated
	ObjKeys []PersonKey `json:"objKeys,omitzero"`
}

// OnPersonCreatedOut are the output arguments of OnPersonCreated.
type OnPersonCreatedOut struct { // MARKER: OnPersonCreated
}

// OnPersonDeleted is triggered when persons are deleted.
var OnPersonDeleted = define.OutboundEvent{ // MARKER: OnPersonDeleted
	Host: Hostname, Method: "POST", Route: ":417/on-person-deleted",
	In: OnPersonDeletedIn{}, Out: OnPersonDeletedOut{},
}

// OnPersonDeletedIn are the input arguments of OnPersonDeleted.
type OnPersonDeletedIn struct { // MARKER: OnPersonDeleted
	ObjKeys []PersonKey `json:"objKeys,omitzero"`
}

// OnPersonDeletedOut are the output arguments of OnPersonDeleted.
type OnPersonDeletedOut struct { // MARKER: OnPersonDeleted
}

// OnPersonStored is triggered when persons are stored.
var OnPersonStored = define.OutboundEvent{ // MARKER: OnPersonStored
	Host: Hostname, Method: "POST", Route: ":417/on-person-stored",
	In: OnPersonStoredIn{}, Out: OnPersonStoredOut{},
}

// OnPersonStoredIn are the input arguments of OnPersonStored.
type OnPersonStoredIn struct { // MARKER: OnPersonStored
	ObjKeys []PersonKey `json:"objKeys,omitzero"`
}

// OnPersonStoredOut are the output arguments of OnPersonStored.
type OnPersonStoredOut struct { // MARKER: OnPersonStored
}

// Create creates a new object, returning its key.
var Create = define.Function{ // MARKER: Create
	Host: Hostname, Method: "ANY", Route: "/create",
	In: CreateIn{}, Out: CreateOut{},
}

// CreateIn are the input arguments of Create.
type CreateIn struct { // MARKER: Create
	Obj *Person `json:"obj,omitzero"`
}

// CreateOut are the output arguments of Create.
type CreateOut struct { // MARKER: Create
	ObjKey PersonKey `json:"objKey,omitzero"`
}

// Store updates the object.
var Store = define.Function{ // MARKER: Store
	Host: Hostname, Method: "ANY", Route: "/store",
	In: StoreIn{}, Out: StoreOut{},
}

// StoreIn are the input arguments of Store.
type StoreIn struct { // MARKER: Store
	Obj *Person `json:"obj,omitzero"`
}

// StoreOut are the output arguments of Store.
type StoreOut struct { // MARKER: Store
	Stored bool `json:"stored,omitzero"`
}

// MustStore updates the object.
var MustStore = define.Function{ // MARKER: MustStore
	Host: Hostname, Method: "ANY", Route: "/must-store",
	In: MustStoreIn{}, Out: MustStoreOut{},
}

// MustStoreIn are the input arguments of MustStore.
type MustStoreIn struct { // MARKER: MustStore
	Obj *Person `json:"obj,omitzero"`
}

// MustStoreOut are the output arguments of MustStore.
type MustStoreOut struct { // MARKER: MustStore
}

// Revise updates the object only if the revision matches.
var Revise = define.Function{ // MARKER: Revise
	Host: Hostname, Method: "ANY", Route: "/revise",
	In: ReviseIn{}, Out: ReviseOut{},
}

// ReviseIn are the input arguments of Revise.
type ReviseIn struct { // MARKER: Revise
	Obj *Person `json:"obj,omitzero"`
}

// ReviseOut are the output arguments of Revise.
type ReviseOut struct { // MARKER: Revise
	Revised bool `json:"revised,omitzero"`
}

// MustRevise updates the object only if the revision matches.
var MustRevise = define.Function{ // MARKER: MustRevise
	Host: Hostname, Method: "ANY", Route: "/must-revise",
	In: MustReviseIn{}, Out: MustReviseOut{},
}

// MustReviseIn are the input arguments of MustRevise.
type MustReviseIn struct { // MARKER: MustRevise
	Obj *Person `json:"obj,omitzero"`
}

// MustReviseOut are the output arguments of MustRevise.
type MustReviseOut struct { // MARKER: MustRevise
}

// Delete deletes the object.
var Delete = define.Function{ // MARKER: Delete
	Host: Hostname, Method: "ANY", Route: "/delete",
	In: DeleteIn{}, Out: DeleteOut{},
}

// DeleteIn are the input arguments of Delete.
type DeleteIn struct { // MARKER: Delete
	ObjKey PersonKey `json:"objKey,omitzero"`
}

// DeleteOut are the output arguments of Delete.
type DeleteOut struct { // MARKER: Delete
	Deleted bool `json:"deleted,omitzero"`
}

// MustDelete deletes the object.
var MustDelete = define.Function{ // MARKER: MustDelete
	Host: Hostname, Method: "ANY", Route: "/must-delete",
	In: MustDeleteIn{}, Out: MustDeleteOut{},
}

// MustDeleteIn are the input arguments of MustDelete.
type MustDeleteIn struct { // MARKER: MustDelete
	ObjKey PersonKey `json:"objKey,omitzero"`
}

// MustDeleteOut are the output arguments of MustDelete.
type MustDeleteOut struct { // MARKER: MustDelete
}

// List returns the objects matching the query, and the total count of matches regardless of the limit.
var List = define.Function{ // MARKER: List
	Host: Hostname, Method: "ANY", Route: "/list",
	In: ListIn{}, Out: ListOut{},
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

// Lookup returns the single object matching the query. It errors if more than one object matches the query.
var Lookup = define.Function{ // MARKER: Lookup
	Host: Hostname, Method: "ANY", Route: "/lookup",
	In: LookupIn{}, Out: LookupOut{},
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

// MustLookup returns the single object matching the query. It errors unless exactly one object matches the query.
var MustLookup = define.Function{ // MARKER: MustLookup
	Host: Hostname, Method: "ANY", Route: "/must-lookup",
	In: MustLookupIn{}, Out: MustLookupOut{},
}

// MustLookupIn are the input arguments of MustLookup.
type MustLookupIn struct { // MARKER: MustLookup
	Query Query `json:"query,omitzero"`
}

// MustLookupOut are the output arguments of MustLookup.
type MustLookupOut struct { // MARKER: MustLookup
	Obj *Person `json:"obj,omitzero"`
}

// Load returns the object associated with the key.
var Load = define.Function{ // MARKER: Load
	Host: Hostname, Method: "ANY", Route: "/load",
	In: LoadIn{}, Out: LoadOut{},
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

// MustLoad returns the object associated with the key. It errors if the object is not found.
var MustLoad = define.Function{ // MARKER: MustLoad
	Host: Hostname, Method: "ANY", Route: "/must-load",
	In: MustLoadIn{}, Out: MustLoadOut{},
}

// MustLoadIn are the input arguments of MustLoad.
type MustLoadIn struct { // MARKER: MustLoad
	ObjKey PersonKey `json:"objKey,omitzero"`
}

// MustLoadOut are the output arguments of MustLoad.
type MustLoadOut struct { // MARKER: MustLoad
	Obj *Person `json:"obj,omitzero"`
}

// BulkLoad returns the objects matching the keys.
var BulkLoad = define.Function{ // MARKER: BulkLoad
	Host: Hostname, Method: "ANY", Route: "/bulk-load",
	In: BulkLoadIn{}, Out: BulkLoadOut{},
}

// BulkLoadIn are the input arguments of BulkLoad.
type BulkLoadIn struct { // MARKER: BulkLoad
	ObjKeys []PersonKey `json:"objKeys,omitzero"`
}

// BulkLoadOut are the output arguments of BulkLoad.
type BulkLoadOut struct { // MARKER: BulkLoad
	Objs []*Person `json:"objs,omitzero"`
}

// BulkDelete deletes the objects matching the keys, returning the keys of the deleted objects.
var BulkDelete = define.Function{ // MARKER: BulkDelete
	Host: Hostname, Method: "ANY", Route: "/bulk-delete",
	In: BulkDeleteIn{}, Out: BulkDeleteOut{},
}

// BulkDeleteIn are the input arguments of BulkDelete.
type BulkDeleteIn struct { // MARKER: BulkDelete
	ObjKeys []PersonKey `json:"objKeys,omitzero"`
}

// BulkDeleteOut are the output arguments of BulkDelete.
type BulkDeleteOut struct { // MARKER: BulkDelete
	DeletedKeys []PersonKey `json:"deletedKeys,omitzero"`
}

// BulkCreate creates multiple objects, returning their keys.
var BulkCreate = define.Function{ // MARKER: BulkCreate
	Host: Hostname, Method: "ANY", Route: "/bulk-create",
	In: BulkCreateIn{}, Out: BulkCreateOut{},
}

// BulkCreateIn are the input arguments of BulkCreate.
type BulkCreateIn struct { // MARKER: BulkCreate
	Objs []*Person `json:"objs,omitzero"`
}

// BulkCreateOut are the output arguments of BulkCreate.
type BulkCreateOut struct { // MARKER: BulkCreate
	ObjKeys []PersonKey `json:"objKeys,omitzero"`
}

// BulkStore updates multiple objects, returning the keys of the stored objects.
var BulkStore = define.Function{ // MARKER: BulkStore
	Host: Hostname, Method: "ANY", Route: "/bulk-store",
	In: BulkStoreIn{}, Out: BulkStoreOut{},
}

// BulkStoreIn are the input arguments of BulkStore.
type BulkStoreIn struct { // MARKER: BulkStore
	Objs []*Person `json:"objs,omitzero"`
}

// BulkStoreOut are the output arguments of BulkStore.
type BulkStoreOut struct { // MARKER: BulkStore
	StoredKeys []PersonKey `json:"storedKeys,omitzero"`
}

// BulkRevise updates multiple objects, returning the number of rows affected.
// Only rows with matching revisions are updated.
var BulkRevise = define.Function{ // MARKER: BulkRevise
	Host: Hostname, Method: "ANY", Route: "/bulk-revise",
	In: BulkReviseIn{}, Out: BulkReviseOut{},
}

// BulkReviseIn are the input arguments of BulkRevise.
type BulkReviseIn struct { // MARKER: BulkRevise
	Objs []*Person `json:"objs,omitzero"`
}

// BulkReviseOut are the output arguments of BulkRevise.
type BulkReviseOut struct { // MARKER: BulkRevise
	RevisedKeys []PersonKey `json:"revisedKeys,omitzero"`
}

// Purge deletes all objects matching the query, returning the keys of the deleted objects.
var Purge = define.Function{ // MARKER: Purge
	Host: Hostname, Method: "ANY", Route: "/purge",
	In: PurgeIn{}, Out: PurgeOut{},
}

// PurgeIn are the input arguments of Purge.
type PurgeIn struct { // MARKER: Purge
	Query Query `json:"query,omitzero"`
}

// PurgeOut are the output arguments of Purge.
type PurgeOut struct { // MARKER: Purge
	DeletedKeys []PersonKey `json:"deletedKeys,omitzero"`
}

// Count returns the number of objects matching the query, disregarding pagination.
var Count = define.Function{ // MARKER: Count
	Host: Hostname, Method: "ANY", Route: "/count",
	In: CountIn{}, Out: CountOut{},
}

// CountIn are the input arguments of Count.
type CountIn struct { // MARKER: Count
	Query Query `json:"query,omitzero"`
}

// CountOut are the output arguments of Count.
type CountOut struct { // MARKER: Count
	Count int `json:"count,omitzero"`
}

// CreateREST creates a new person via REST, returning its key.
var CreateREST = define.Function{ // MARKER: CreateREST
	Host: Hostname, Method: "POST", Route: "/persons",
	In: CreateRESTIn{}, Out: CreateRESTOut{},
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

// StoreREST updates an existing person via REST.
var StoreREST = define.Function{ // MARKER: StoreREST
	Host: Hostname, Method: "PUT", Route: "/persons/{key}",
	In: StoreRESTIn{}, Out: StoreRESTOut{},
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

// DeleteREST deletes an existing person via REST.
var DeleteREST = define.Function{ // MARKER: DeleteREST
	Host: Hostname, Method: "DELETE", Route: "/persons/{key}",
	In: DeleteRESTIn{}, Out: DeleteRESTOut{},
}

// DeleteRESTIn are the input arguments of DeleteREST.
type DeleteRESTIn struct { // MARKER: DeleteREST
	Key PersonKey `json:"key,omitzero"`
}

// DeleteRESTOut are the output arguments of DeleteREST.
type DeleteRESTOut struct { // MARKER: DeleteREST
	HTTPStatusCode int `json:"-"`
}

// LoadREST loads a person by key via REST.
var LoadREST = define.Function{ // MARKER: LoadREST
	Host: Hostname, Method: "GET", Route: "/persons/{key}",
	In: LoadRESTIn{}, Out: LoadRESTOut{},
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

// ListREST lists persons matching the query via REST.
var ListREST = define.Function{ // MARKER: ListREST
	Host: Hostname, Method: "GET", Route: "/persons",
	In: ListRESTIn{}, Out: ListRESTOut{},
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

// TryReserve attempts to reserve a person for the given duration, returning true if successful.
var TryReserve = define.Function{ // MARKER: TryReserve
	Host: Hostname, Method: "ANY", Route: "/try-reserve",
	In: TryReserveIn{}, Out: TryReserveOut{},
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

// TryBulkReserve attempts to reserve persons for the given duration, returning the keys of those successfully reserved.
// Only persons whose reservation has expired (reserved_before < NOW) are reserved.
var TryBulkReserve = define.Function{ // MARKER: TryBulkReserve
	Host: Hostname, Method: "ANY", Route: "/try-bulk-reserve",
	In: TryBulkReserveIn{}, Out: TryBulkReserveOut{},
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

// Reserve unconditionally reserves a person for the given duration, returning true if the person exists.
var Reserve = define.Function{ // MARKER: Reserve
	Host: Hostname, Method: "ANY", Route: "/reserve",
	In: ReserveIn{}, Out: ReserveOut{},
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

// BulkReserve unconditionally reserves persons for the given duration, returning the keys of those that exist.
var BulkReserve = define.Function{ // MARKER: BulkReserve
	Host: Hostname, Method: "ANY", Route: "/bulk-reserve",
	In: BulkReserveIn{}, Out: BulkReserveOut{},
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

// Demo serves the web user interface for managing persons.
var Demo = define.Web{ // MARKER: Demo
	Host: Hostname, Method: "ANY", Route: "/demo",
}

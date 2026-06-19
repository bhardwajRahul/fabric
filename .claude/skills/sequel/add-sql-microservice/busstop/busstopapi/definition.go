package busstopapi

import (
	"github.com/microbus-io/fabric/define"
	"time"
)

// HINT: This file is the single source of truth for the microservice's API. After editing it, run
// cmd/genservice on the microservice's directory (the parent of this api package) to regenerate client.go,
// intermediate.go, mock.go, mock_test.go, and manifest.yaml. Do not hand-edit those generated files.

// Hostname is the default hostname of the microservice.
const Hostname = "busstop.hostname"

// Name is the decorative PascalCase name of the microservice.
const Name = "BusStop"

// Version is the major version of the microservice's public API.
const Version = 2

// Description is the human-readable summary of the microservice, surfaced in OpenAPI and discovery.
const Description = `BusStop persists bus stops in a SQL database.`

// SQLDataSourceName is the connection string of the SQL database.
var SQLDataSourceName = define.Config{ // MARKER: SQLDataSourceName
	Value:  string(""),
	Secret: true,
}

// OnBusStopCreated is triggered when bus stops are created.
var OnBusStopCreated = define.OutboundEvent{ // MARKER: OnBusStopCreated
	Host: Hostname, Method: "POST", Route: ":417/on-bus-stop-created",
	In: OnBusStopCreatedIn{}, Out: OnBusStopCreatedOut{},
}

// OnBusStopCreatedIn are the input arguments of OnBusStopCreated.
type OnBusStopCreatedIn struct { // MARKER: OnBusStopCreated
	ObjKeys []BusStopKey `json:"objKeys,omitzero"`
}

// OnBusStopCreatedOut are the output arguments of OnBusStopCreated.
type OnBusStopCreatedOut struct { // MARKER: OnBusStopCreated
}

// OnBusStopStored is triggered when bus stops are stored.
var OnBusStopStored = define.OutboundEvent{ // MARKER: OnBusStopStored
	Host: Hostname, Method: "POST", Route: ":417/on-bus-stop-stored",
	In: OnBusStopStoredIn{}, Out: OnBusStopStoredOut{},
}

// OnBusStopStoredIn are the input arguments of OnBusStopStored.
type OnBusStopStoredIn struct { // MARKER: OnBusStopStored
	ObjKeys []BusStopKey `json:"objKeys,omitzero"`
}

// OnBusStopStoredOut are the output arguments of OnBusStopStored.
type OnBusStopStoredOut struct { // MARKER: OnBusStopStored
}

// OnBusStopDeleted is triggered when bus stops are deleted.
var OnBusStopDeleted = define.OutboundEvent{ // MARKER: OnBusStopDeleted
	Host: Hostname, Method: "POST", Route: ":417/on-bus-stop-deleted",
	In: OnBusStopDeletedIn{}, Out: OnBusStopDeletedOut{},
}

// OnBusStopDeletedIn are the input arguments of OnBusStopDeleted.
type OnBusStopDeletedIn struct { // MARKER: OnBusStopDeleted
	ObjKeys []BusStopKey `json:"objKeys,omitzero"`
}

// OnBusStopDeletedOut are the output arguments of OnBusStopDeleted.
type OnBusStopDeletedOut struct { // MARKER: OnBusStopDeleted
}

// Create creates a new bus stop, returning its key.
var Create = define.Function{ // MARKER: Create
	Host: Hostname, Method: "ANY", Route: "/create",
	In: CreateIn{}, Out: CreateOut{},
}

// CreateIn are the input arguments of Create.
type CreateIn struct { // MARKER: Create
	Obj *BusStop `json:"obj,omitzero"`
}

// CreateOut are the output arguments of Create.
type CreateOut struct { // MARKER: Create
	ObjKey BusStopKey `json:"objKey,omitzero"`
}

// Store updates the bus stop.
var Store = define.Function{ // MARKER: Store
	Host: Hostname, Method: "ANY", Route: "/store",
	In: StoreIn{}, Out: StoreOut{},
}

// StoreIn are the input arguments of Store.
type StoreIn struct { // MARKER: Store
	Obj *BusStop `json:"obj,omitzero"`
}

// StoreOut are the output arguments of Store.
type StoreOut struct { // MARKER: Store
	Stored bool `json:"stored,omitzero"`
}

// MustStore updates the bus stop, erroring if not found.
var MustStore = define.Function{ // MARKER: MustStore
	Host: Hostname, Method: "ANY", Route: "/must-store",
	In: MustStoreIn{}, Out: MustStoreOut{},
}

// MustStoreIn are the input arguments of MustStore.
type MustStoreIn struct { // MARKER: MustStore
	Obj *BusStop `json:"obj,omitzero"`
}

// MustStoreOut are the output arguments of MustStore.
type MustStoreOut struct { // MARKER: MustStore
}

// Revise updates the bus stop only if the revision matches.
var Revise = define.Function{ // MARKER: Revise
	Host: Hostname, Method: "ANY", Route: "/revise",
	In: ReviseIn{}, Out: ReviseOut{},
}

// ReviseIn are the input arguments of Revise.
type ReviseIn struct { // MARKER: Revise
	Obj *BusStop `json:"obj,omitzero"`
}

// ReviseOut are the output arguments of Revise.
type ReviseOut struct { // MARKER: Revise
	Revised bool `json:"revised,omitzero"`
}

// MustRevise updates the bus stop only if the revision matches, erroring on conflict.
var MustRevise = define.Function{ // MARKER: MustRevise
	Host: Hostname, Method: "ANY", Route: "/must-revise",
	In: MustReviseIn{}, Out: MustReviseOut{},
}

// MustReviseIn are the input arguments of MustRevise.
type MustReviseIn struct { // MARKER: MustRevise
	Obj *BusStop `json:"obj,omitzero"`
}

// MustReviseOut are the output arguments of MustRevise.
type MustReviseOut struct { // MARKER: MustRevise
}

// Delete deletes the bus stop.
var Delete = define.Function{ // MARKER: Delete
	Host: Hostname, Method: "ANY", Route: "/delete",
	In: DeleteIn{}, Out: DeleteOut{},
}

// DeleteIn are the input arguments of Delete.
type DeleteIn struct { // MARKER: Delete
	ObjKey BusStopKey `json:"objKey,omitzero"`
}

// DeleteOut are the output arguments of Delete.
type DeleteOut struct { // MARKER: Delete
	Deleted bool `json:"deleted,omitzero"`
}

// MustDelete deletes the bus stop, erroring if not found.
var MustDelete = define.Function{ // MARKER: MustDelete
	Host: Hostname, Method: "ANY", Route: "/must-delete",
	In: MustDeleteIn{}, Out: MustDeleteOut{},
}

// MustDeleteIn are the input arguments of MustDelete.
type MustDeleteIn struct { // MARKER: MustDelete
	ObjKey BusStopKey `json:"objKey,omitzero"`
}

// MustDeleteOut are the output arguments of MustDelete.
type MustDeleteOut struct { // MARKER: MustDelete
}

// List returns the bus stops matching the query, and the total count of matches regardless of the limit.
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
	Objs       []*BusStop `json:"objs,omitzero"`
	TotalCount int        `json:"totalCount,omitzero"`
}

// Lookup returns the single bus stop matching the query.
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
	Obj   *BusStop `json:"obj,omitzero"`
	Found bool     `json:"found,omitzero"`
}

// MustLookup returns the single bus stop matching the query. It errors unless exactly one bus stop matches the query.
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
	Obj *BusStop `json:"obj,omitzero"`
}

// Load returns the bus stop associated with the key.
var Load = define.Function{ // MARKER: Load
	Host: Hostname, Method: "ANY", Route: "/load",
	In: LoadIn{}, Out: LoadOut{},
}

// LoadIn are the input arguments of Load.
type LoadIn struct { // MARKER: Load
	ObjKey BusStopKey `json:"objKey,omitzero"`
}

// LoadOut are the output arguments of Load.
type LoadOut struct { // MARKER: Load
	Obj   *BusStop `json:"obj,omitzero"`
	Found bool     `json:"found,omitzero"`
}

// MustLoad returns the bus stop associated with the key, erroring if not found.
var MustLoad = define.Function{ // MARKER: MustLoad
	Host: Hostname, Method: "ANY", Route: "/must-load",
	In: MustLoadIn{}, Out: MustLoadOut{},
}

// MustLoadIn are the input arguments of MustLoad.
type MustLoadIn struct { // MARKER: MustLoad
	ObjKey BusStopKey `json:"objKey,omitzero"`
}

// MustLoadOut are the output arguments of MustLoad.
type MustLoadOut struct { // MARKER: MustLoad
	Obj *BusStop `json:"obj,omitzero"`
}

// BulkLoad returns the bus stops matching the keys.
var BulkLoad = define.Function{ // MARKER: BulkLoad
	Host: Hostname, Method: "ANY", Route: "/bulk-load",
	In: BulkLoadIn{}, Out: BulkLoadOut{},
}

// BulkLoadIn are the input arguments of BulkLoad.
type BulkLoadIn struct { // MARKER: BulkLoad
	ObjKeys []BusStopKey `json:"objKeys,omitzero"`
}

// BulkLoadOut are the output arguments of BulkLoad.
type BulkLoadOut struct { // MARKER: BulkLoad
	Objs []*BusStop `json:"objs,omitzero"`
}

// BulkDelete deletes the bus stops matching the keys, returning the keys of the deleted bus stops.
var BulkDelete = define.Function{ // MARKER: BulkDelete
	Host: Hostname, Method: "ANY", Route: "/bulk-delete",
	In: BulkDeleteIn{}, Out: BulkDeleteOut{},
}

// BulkDeleteIn are the input arguments of BulkDelete.
type BulkDeleteIn struct { // MARKER: BulkDelete
	ObjKeys []BusStopKey `json:"objKeys,omitzero"`
}

// BulkDeleteOut are the output arguments of BulkDelete.
type BulkDeleteOut struct { // MARKER: BulkDelete
	DeletedKeys []BusStopKey `json:"deletedKeys,omitzero"`
}

// BulkCreate creates multiple bus stops, returning their keys.
var BulkCreate = define.Function{ // MARKER: BulkCreate
	Host: Hostname, Method: "ANY", Route: "/bulk-create",
	In: BulkCreateIn{}, Out: BulkCreateOut{},
}

// BulkCreateIn are the input arguments of BulkCreate.
type BulkCreateIn struct { // MARKER: BulkCreate
	Objs []*BusStop `json:"objs,omitzero"`
}

// BulkCreateOut are the output arguments of BulkCreate.
type BulkCreateOut struct { // MARKER: BulkCreate
	ObjKeys []BusStopKey `json:"objKeys,omitzero"`
}

// BulkStore updates multiple bus stops, returning the keys of the stored bus stops.
var BulkStore = define.Function{ // MARKER: BulkStore
	Host: Hostname, Method: "ANY", Route: "/bulk-store",
	In: BulkStoreIn{}, Out: BulkStoreOut{},
}

// BulkStoreIn are the input arguments of BulkStore.
type BulkStoreIn struct { // MARKER: BulkStore
	Objs []*BusStop `json:"objs,omitzero"`
}

// BulkStoreOut are the output arguments of BulkStore.
type BulkStoreOut struct { // MARKER: BulkStore
	StoredKeys []BusStopKey `json:"storedKeys,omitzero"`
}

// BulkRevise updates multiple bus stops only if the revisions match, returning the keys of the revised bus stops.
var BulkRevise = define.Function{ // MARKER: BulkRevise
	Host: Hostname, Method: "ANY", Route: "/bulk-revise",
	In: BulkReviseIn{}, Out: BulkReviseOut{},
}

// BulkReviseIn are the input arguments of BulkRevise.
type BulkReviseIn struct { // MARKER: BulkRevise
	Objs []*BusStop `json:"objs,omitzero"`
}

// BulkReviseOut are the output arguments of BulkRevise.
type BulkReviseOut struct { // MARKER: BulkRevise
	RevisedKeys []BusStopKey `json:"revisedKeys,omitzero"`
}

// Purge deletes all bus stops matching the query, returning the keys of the deleted bus stops.
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
	DeletedKeys []BusStopKey `json:"deletedKeys,omitzero"`
}

// Count returns the number of bus stops matching the query.
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

// CreateREST creates a new bus stop via REST, returning its key.
var CreateREST = define.Function{ // MARKER: CreateREST
	Host: Hostname, Method: "POST", Route: "/bus-stops",
	In: CreateRESTIn{}, Out: CreateRESTOut{},
}

// CreateRESTIn are the input arguments of CreateREST.
type CreateRESTIn struct { // MARKER: CreateREST
	HTTPRequestBody *BusStop `json:"-"`
}

// CreateRESTOut are the output arguments of CreateREST.
type CreateRESTOut struct { // MARKER: CreateREST
	ObjKey         BusStopKey `json:"objKey,omitzero"`
	HTTPStatusCode int        `json:"-"`
}

// StoreREST updates an existing bus stop via REST.
var StoreREST = define.Function{ // MARKER: StoreREST
	Host: Hostname, Method: "PUT", Route: "/bus-stops/{key}",
	In: StoreRESTIn{}, Out: StoreRESTOut{},
}

// StoreRESTIn are the input arguments of StoreREST.
type StoreRESTIn struct { // MARKER: StoreREST
	Key             BusStopKey `json:"key,omitzero"`
	HTTPRequestBody *BusStop   `json:"-"`
}

// StoreRESTOut are the output arguments of StoreREST.
type StoreRESTOut struct { // MARKER: StoreREST
	HTTPStatusCode int `json:"-"`
}

// DeleteREST deletes an existing bus stop via REST.
var DeleteREST = define.Function{ // MARKER: DeleteREST
	Host: Hostname, Method: "DELETE", Route: "/bus-stops/{key}",
	In: DeleteRESTIn{}, Out: DeleteRESTOut{},
}

// DeleteRESTIn are the input arguments of DeleteREST.
type DeleteRESTIn struct { // MARKER: DeleteREST
	Key BusStopKey `json:"key,omitzero"`
}

// DeleteRESTOut are the output arguments of DeleteREST.
type DeleteRESTOut struct { // MARKER: DeleteREST
	HTTPStatusCode int `json:"-"`
}

// LoadREST loads a bus stop by key via REST.
var LoadREST = define.Function{ // MARKER: LoadREST
	Host: Hostname, Method: "GET", Route: "/bus-stops/{key}",
	In: LoadRESTIn{}, Out: LoadRESTOut{},
}

// LoadRESTIn are the input arguments of LoadREST.
type LoadRESTIn struct { // MARKER: LoadREST
	Key BusStopKey `json:"key,omitzero"`
}

// LoadRESTOut are the output arguments of LoadREST.
type LoadRESTOut struct { // MARKER: LoadREST
	HTTPResponseBody *BusStop `json:"-"`
	HTTPStatusCode   int      `json:"-"`
}

// ListREST lists bus stops matching the query via REST.
var ListREST = define.Function{ // MARKER: ListREST
	Host: Hostname, Method: "GET", Route: "/bus-stops",
	In: ListRESTIn{}, Out: ListRESTOut{},
}

// ListRESTIn are the input arguments of ListREST.
type ListRESTIn struct { // MARKER: ListREST
	Q Query `json:"q,omitzero"`
}

// ListRESTOut are the output arguments of ListREST.
type ListRESTOut struct { // MARKER: ListREST
	HTTPResponseBody []*BusStop `json:"-"`
	HTTPStatusCode   int        `json:"-"`
}

// TryReserve attempts to reserve a bus stop for the given duration, returning true if successful.
var TryReserve = define.Function{ // MARKER: TryReserve
	Host: Hostname, Method: "ANY", Route: "/try-reserve",
	In: TryReserveIn{}, Out: TryReserveOut{},
}

// TryReserveIn are the input arguments of TryReserve.
type TryReserveIn struct { // MARKER: TryReserve
	ObjKey BusStopKey    `json:"objKey,omitzero"`
	Dur    time.Duration `json:"dur,omitzero"`
}

// TryReserveOut are the output arguments of TryReserve.
type TryReserveOut struct { // MARKER: TryReserve
	Reserved bool `json:"reserved,omitzero"`
}

// TryBulkReserve attempts to reserve bus stops for the given duration, returning the keys of those successfully reserved.
var TryBulkReserve = define.Function{ // MARKER: TryBulkReserve
	Host: Hostname, Method: "ANY", Route: "/try-bulk-reserve",
	In: TryBulkReserveIn{}, Out: TryBulkReserveOut{},
}

// TryBulkReserveIn are the input arguments of TryBulkReserve.
type TryBulkReserveIn struct { // MARKER: TryBulkReserve
	ObjKeys []BusStopKey  `json:"objKeys,omitzero"`
	Dur     time.Duration `json:"dur,omitzero"`
}

// TryBulkReserveOut are the output arguments of TryBulkReserve.
type TryBulkReserveOut struct { // MARKER: TryBulkReserve
	ReservedKeys []BusStopKey `json:"reservedKeys,omitzero"`
}

// Reserve unconditionally reserves a bus stop for the given duration, returning true if the bus stop exists.
var Reserve = define.Function{ // MARKER: Reserve
	Host: Hostname, Method: "ANY", Route: "/reserve",
	In: ReserveIn{}, Out: ReserveOut{},
}

// ReserveIn are the input arguments of Reserve.
type ReserveIn struct { // MARKER: Reserve
	ObjKey BusStopKey    `json:"objKey,omitzero"`
	Dur    time.Duration `json:"dur,omitzero"`
}

// ReserveOut are the output arguments of Reserve.
type ReserveOut struct { // MARKER: Reserve
	Reserved bool `json:"reserved,omitzero"`
}

// BulkReserve unconditionally reserves bus stops for the given duration, returning the keys of those that exist.
var BulkReserve = define.Function{ // MARKER: BulkReserve
	Host: Hostname, Method: "ANY", Route: "/bulk-reserve",
	In: BulkReserveIn{}, Out: BulkReserveOut{},
}

// BulkReserveIn are the input arguments of BulkReserve.
type BulkReserveIn struct { // MARKER: BulkReserve
	ObjKeys []BusStopKey  `json:"objKeys,omitzero"`
	Dur     time.Duration `json:"dur,omitzero"`
}

// BulkReserveOut are the output arguments of BulkReserve.
type BulkReserveOut struct { // MARKER: BulkReserve
	ReservedKeys []BusStopKey `json:"reservedKeys,omitzero"`
}

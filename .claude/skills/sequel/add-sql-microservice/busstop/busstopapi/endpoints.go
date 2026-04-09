package busstopapi

import (
	"time"

	"github.com/microbus-io/fabric/httpx"
)

// Hostname is the default hostname of the microservice.
const Hostname = "busstop.hostname"

// Def is the routing identity of an endpoint exposed by this microservice.
type Def struct {
	Method string
	Route  string
}

// URL is the full URL of the endpoint, joined with the package-level Hostname.
func (d Def) URL() string {
	return httpx.JoinHostAndPath(Hostname, d.Route)
}

var _ time.Duration

// CreateIn are the input arguments of Create.
type CreateIn struct { // MARKER: Create
	Obj *BusStop `json:"obj,omitzero"`
}

// CreateOut are the output arguments of Create.
type CreateOut struct { // MARKER: Create
	ObjKey BusStopKey `json:"objKey,omitzero"`
}

// StoreIn are the input arguments of Store.
type StoreIn struct { // MARKER: Store
	Obj *BusStop `json:"obj,omitzero"`
}

// StoreOut are the output arguments of Store.
type StoreOut struct { // MARKER: Store
	Stored bool `json:"stored,omitzero"`
}

// MustStoreIn are the input arguments of MustStore.
type MustStoreIn struct { // MARKER: MustStore
	Obj *BusStop `json:"obj,omitzero"`
}

// MustStoreOut are the output arguments of MustStore.
type MustStoreOut struct { // MARKER: MustStore
}

// ReviseIn are the input arguments of Revise.
type ReviseIn struct { // MARKER: Revise
	Obj *BusStop `json:"obj,omitzero"`
}

// ReviseOut are the output arguments of Revise.
type ReviseOut struct { // MARKER: Revise
	Revised bool `json:"revised,omitzero"`
}

// MustReviseIn are the input arguments of MustRevise.
type MustReviseIn struct { // MARKER: MustRevise
	Obj *BusStop `json:"obj,omitzero"`
}

// MustReviseOut are the output arguments of MustRevise.
type MustReviseOut struct { // MARKER: MustRevise
}

// DeleteIn are the input arguments of Delete.
type DeleteIn struct { // MARKER: Delete
	ObjKey BusStopKey `json:"objKey,omitzero"`
}

// DeleteOut are the output arguments of Delete.
type DeleteOut struct { // MARKER: Delete
	Deleted bool `json:"deleted,omitzero"`
}

// MustDeleteIn are the input arguments of MustDelete.
type MustDeleteIn struct { // MARKER: MustDelete
	ObjKey BusStopKey `json:"objKey,omitzero"`
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
	Objs       []*BusStop `json:"objs,omitzero"`
	TotalCount int        `json:"totalCount,omitzero"`
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

// MustLookupIn are the input arguments of MustLookup.
type MustLookupIn struct { // MARKER: MustLookup
	Query Query `json:"query,omitzero"`
}

// MustLookupOut are the output arguments of MustLookup.
type MustLookupOut struct { // MARKER: MustLookup
	Obj *BusStop `json:"obj,omitzero"`
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

// MustLoadIn are the input arguments of MustLoad.
type MustLoadIn struct { // MARKER: MustLoad
	ObjKey BusStopKey `json:"objKey,omitzero"`
}

// MustLoadOut are the output arguments of MustLoad.
type MustLoadOut struct { // MARKER: MustLoad
	Obj *BusStop `json:"obj,omitzero"`
}

// BulkLoadIn are the input arguments of BulkLoad.
type BulkLoadIn struct { // MARKER: BulkLoad
	ObjKeys []BusStopKey `json:"objKeys,omitzero"`
}

// BulkLoadOut are the output arguments of BulkLoad.
type BulkLoadOut struct { // MARKER: BulkLoad
	Objs []*BusStop `json:"objs,omitzero"`
}

// BulkDeleteIn are the input arguments of BulkDelete.
type BulkDeleteIn struct { // MARKER: BulkDelete
	ObjKeys []BusStopKey `json:"objKeys,omitzero"`
}

// BulkDeleteOut are the output arguments of BulkDelete.
type BulkDeleteOut struct { // MARKER: BulkDelete
	DeletedKeys []BusStopKey `json:"deletedKeys,omitzero"`
}

// BulkCreateIn are the input arguments of BulkCreate.
type BulkCreateIn struct { // MARKER: BulkCreate
	Objs []*BusStop `json:"objs,omitzero"`
}

// BulkCreateOut are the output arguments of BulkCreate.
type BulkCreateOut struct { // MARKER: BulkCreate
	ObjKeys []BusStopKey `json:"objKeys,omitzero"`
}

// BulkStoreIn are the input arguments of BulkStore.
type BulkStoreIn struct { // MARKER: BulkStore
	Objs []*BusStop `json:"objs,omitzero"`
}

// BulkStoreOut are the output arguments of BulkStore.
type BulkStoreOut struct { // MARKER: BulkStore
	StoredKeys []BusStopKey `json:"storedKeys,omitzero"`
}

// BulkReviseIn are the input arguments of BulkRevise.
type BulkReviseIn struct { // MARKER: BulkRevise
	Objs []*BusStop `json:"objs,omitzero"`
}

// BulkReviseOut are the output arguments of BulkRevise.
type BulkReviseOut struct { // MARKER: BulkRevise
	RevisedKeys []BusStopKey `json:"revisedKeys,omitzero"`
}

// PurgeIn are the input arguments of Purge.
type PurgeIn struct { // MARKER: Purge
	Query Query `json:"query,omitzero"`
}

// PurgeOut are the output arguments of Purge.
type PurgeOut struct { // MARKER: Purge
	DeletedKeys []BusStopKey `json:"deletedKeys,omitzero"`
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
	HTTPRequestBody *BusStop `json:"-"`
}

// CreateRESTOut are the output arguments of CreateREST.
type CreateRESTOut struct { // MARKER: CreateREST
	ObjKey         BusStopKey `json:"objKey,omitzero"`
	HTTPStatusCode int        `json:"-"`
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

// DeleteRESTIn are the input arguments of DeleteREST.
type DeleteRESTIn struct { // MARKER: DeleteREST
	Key BusStopKey `json:"key,omitzero"`
}

// DeleteRESTOut are the output arguments of DeleteREST.
type DeleteRESTOut struct { // MARKER: DeleteREST
	HTTPStatusCode int `json:"-"`
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

// ListRESTIn are the input arguments of ListREST.
type ListRESTIn struct { // MARKER: ListREST
	Q Query `json:"q,omitzero"`
}

// ListRESTOut are the output arguments of ListREST.
type ListRESTOut struct { // MARKER: ListREST
	HTTPResponseBody []*BusStop `json:"-"`
	HTTPStatusCode   int        `json:"-"`
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

// TryBulkReserveIn are the input arguments of TryBulkReserve.
type TryBulkReserveIn struct { // MARKER: TryBulkReserve
	ObjKeys []BusStopKey  `json:"objKeys,omitzero"`
	Dur     time.Duration `json:"dur,omitzero"`
}

// TryBulkReserveOut are the output arguments of TryBulkReserve.
type TryBulkReserveOut struct { // MARKER: TryBulkReserve
	ReservedKeys []BusStopKey `json:"reservedKeys,omitzero"`
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

// BulkReserveIn are the input arguments of BulkReserve.
type BulkReserveIn struct { // MARKER: BulkReserve
	ObjKeys []BusStopKey  `json:"objKeys,omitzero"`
	Dur     time.Duration `json:"dur,omitzero"`
}

// BulkReserveOut are the output arguments of BulkReserve.
type BulkReserveOut struct { // MARKER: BulkReserve
	ReservedKeys []BusStopKey `json:"reservedKeys,omitzero"`
}

// OnBusStopCreatedIn are the input arguments of OnBusStopCreated.
type OnBusStopCreatedIn struct { // MARKER: OnBusStopCreated
	ObjKeys []BusStopKey `json:"objKeys,omitzero"`
}

// OnBusStopCreatedOut are the output arguments of OnBusStopCreated.
type OnBusStopCreatedOut struct { // MARKER: OnBusStopCreated
}

// OnBusStopStoredIn are the input arguments of OnBusStopStored.
type OnBusStopStoredIn struct { // MARKER: OnBusStopStored
	ObjKeys []BusStopKey `json:"objKeys,omitzero"`
}

// OnBusStopStoredOut are the output arguments of OnBusStopStored.
type OnBusStopStoredOut struct { // MARKER: OnBusStopStored
}

// OnBusStopDeletedIn are the input arguments of OnBusStopDeleted.
type OnBusStopDeletedIn struct { // MARKER: OnBusStopDeleted
	ObjKeys []BusStopKey `json:"objKeys,omitzero"`
}

// OnBusStopDeletedOut are the output arguments of OnBusStopDeleted.
type OnBusStopDeletedOut struct { // MARKER: OnBusStopDeleted
}

var (
	// HINT: Insert endpoint definitions here
	Create           = Def{Method: "ANY", Route: ":443/create"}                       // MARKER: Create
	Store            = Def{Method: "ANY", Route: ":443/store"}                        // MARKER: Store
	MustStore        = Def{Method: "ANY", Route: ":443/must-store"}                   // MARKER: MustStore
	Revise           = Def{Method: "ANY", Route: ":443/revise"}                       // MARKER: Revise
	MustRevise       = Def{Method: "ANY", Route: ":443/must-revise"}                  // MARKER: MustRevise
	Delete           = Def{Method: "ANY", Route: ":443/delete"}                       // MARKER: Delete
	MustDelete       = Def{Method: "ANY", Route: ":443/must-delete"}                  // MARKER: MustDelete
	List             = Def{Method: "ANY", Route: ":443/list"}                         // MARKER: List
	Lookup           = Def{Method: "ANY", Route: ":443/lookup"}                       // MARKER: Lookup
	MustLookup       = Def{Method: "ANY", Route: ":443/must-lookup"}                  // MARKER: MustLookup
	Load             = Def{Method: "ANY", Route: ":443/load"}                         // MARKER: Load
	MustLoad         = Def{Method: "ANY", Route: ":443/must-load"}                    // MARKER: MustLoad
	BulkLoad         = Def{Method: "ANY", Route: ":443/bulk-load"}                    // MARKER: BulkLoad
	BulkDelete       = Def{Method: "ANY", Route: ":443/bulk-delete"}                  // MARKER: BulkDelete
	BulkCreate       = Def{Method: "ANY", Route: ":443/bulk-create"}                  // MARKER: BulkCreate
	BulkStore        = Def{Method: "ANY", Route: ":443/bulk-store"}                   // MARKER: BulkStore
	BulkRevise       = Def{Method: "ANY", Route: ":443/bulk-revise"}                  // MARKER: BulkRevise
	Purge            = Def{Method: "ANY", Route: ":443/purge"}                        // MARKER: Purge
	Count            = Def{Method: "ANY", Route: ":443/count"}                        // MARKER: Count
	CreateREST       = Def{Method: "POST", Route: ":443/bus-stops"}                   // MARKER: CreateREST
	StoreREST        = Def{Method: "PUT", Route: ":443/bus-stops/{key}"}              // MARKER: StoreREST
	DeleteREST       = Def{Method: "DELETE", Route: ":443/bus-stops/{key}"}           // MARKER: DeleteREST
	LoadREST         = Def{Method: "GET", Route: ":443/bus-stops/{key}"}              // MARKER: LoadREST
	ListREST         = Def{Method: "GET", Route: ":443/bus-stops"}                    // MARKER: ListREST
	TryReserve       = Def{Method: "ANY", Route: ":443/try-reserve"}                  // MARKER: TryReserve
	TryBulkReserve   = Def{Method: "ANY", Route: ":443/try-bulk-reserve"}             // MARKER: TryBulkReserve
	Reserve          = Def{Method: "ANY", Route: ":443/reserve"}                      // MARKER: Reserve
	BulkReserve      = Def{Method: "ANY", Route: ":443/bulk-reserve"}                 // MARKER: BulkReserve
	OnBusStopCreated = Def{Method: "POST", Route: ":417/on-bus-stop-created"}         // MARKER: OnBusStopCreated
	OnBusStopStored  = Def{Method: "POST", Route: ":417/on-bus-stop-stored"}          // MARKER: OnBusStopStored
	OnBusStopDeleted = Def{Method: "POST", Route: ":417/on-bus-stop-deleted"}         // MARKER: OnBusStopDeleted
)

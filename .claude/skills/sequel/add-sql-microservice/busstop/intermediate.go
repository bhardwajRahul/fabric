package busstop

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/cfg"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/fabric/sub"
	"github.com/microbus-io/fabric/utils"
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/busstop/busstopapi"
	"github.com/microbus-io/fabric/busstop/resources"
)

var (
	_ context.Context
	_ json.Encoder
	_ http.Request
	_ strconv.NumError
	_ time.Duration
	_ errors.TracedError
	_ cfg.Option
	_ httpx.BodyReader
	_ sub.Option
	_ utils.SyncMap[string, string]
	_ busstopapi.Client
	_ *workflow.Flow
)

const (
	Hostname = busstopapi.Hostname
	Version  = 2
)

// ToDo is implemented by the service or mock.
// The intermediate delegates handling to this interface.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
	Create(ctx context.Context, obj *busstopapi.BusStop) (objKey busstopapi.BusStopKey, err error)                                            // MARKER: Create
	Store(ctx context.Context, obj *busstopapi.BusStop) (stored bool, err error)                                                              // MARKER: Store
	MustStore(ctx context.Context, obj *busstopapi.BusStop) (err error)                                                                       // MARKER: MustStore
	Revise(ctx context.Context, obj *busstopapi.BusStop) (revised bool, err error)                                                            // MARKER: Revise
	MustRevise(ctx context.Context, obj *busstopapi.BusStop) (err error)                                                                      // MARKER: MustRevise
	Delete(ctx context.Context, objKey busstopapi.BusStopKey) (deleted bool, err error)                                                       // MARKER: Delete
	MustDelete(ctx context.Context, objKey busstopapi.BusStopKey) (err error)                                                                 // MARKER: MustDelete
	List(ctx context.Context, query busstopapi.Query) (objs []*busstopapi.BusStop, totalCount int, err error)                                 // MARKER: List
	Lookup(ctx context.Context, query busstopapi.Query) (obj *busstopapi.BusStop, found bool, err error)                                      // MARKER: Lookup
	MustLookup(ctx context.Context, query busstopapi.Query) (obj *busstopapi.BusStop, err error)                                              // MARKER: MustLookup
	Load(ctx context.Context, objKey busstopapi.BusStopKey) (obj *busstopapi.BusStop, found bool, err error)                                  // MARKER: Load
	MustLoad(ctx context.Context, objKey busstopapi.BusStopKey) (obj *busstopapi.BusStop, err error)                                          // MARKER: MustLoad
	BulkLoad(ctx context.Context, objKeys []busstopapi.BusStopKey) (objs []*busstopapi.BusStop, err error)                                    // MARKER: BulkLoad
	BulkDelete(ctx context.Context, objKeys []busstopapi.BusStopKey) (deletedKeys []busstopapi.BusStopKey, err error)                         // MARKER: BulkDelete
	BulkCreate(ctx context.Context, objs []*busstopapi.BusStop) (objKeys []busstopapi.BusStopKey, err error)                                  // MARKER: BulkCreate
	BulkStore(ctx context.Context, objs []*busstopapi.BusStop) (storedKeys []busstopapi.BusStopKey, err error)                                // MARKER: BulkStore
	BulkRevise(ctx context.Context, objs []*busstopapi.BusStop) (revisedKeys []busstopapi.BusStopKey, err error)                              // MARKER: BulkRevise
	Purge(ctx context.Context, query busstopapi.Query) (deletedKeys []busstopapi.BusStopKey, err error)                                       // MARKER: Purge
	Count(ctx context.Context, query busstopapi.Query) (count int, err error)                                                                 // MARKER: Count
	CreateREST(ctx context.Context, httpRequestBody *busstopapi.BusStop) (objKey busstopapi.BusStopKey, httpStatusCode int, err error)        // MARKER: CreateREST
	StoreREST(ctx context.Context, key busstopapi.BusStopKey, httpRequestBody *busstopapi.BusStop) (httpStatusCode int, err error)            // MARKER: StoreREST
	DeleteREST(ctx context.Context, key busstopapi.BusStopKey) (httpStatusCode int, err error)                                                // MARKER: DeleteREST
	LoadREST(ctx context.Context, key busstopapi.BusStopKey) (httpResponseBody *busstopapi.BusStop, httpStatusCode int, err error)            // MARKER: LoadREST
	ListREST(ctx context.Context, q busstopapi.Query) (httpResponseBody []*busstopapi.BusStop, httpStatusCode int, err error)                 // MARKER: ListREST
	TryReserve(ctx context.Context, objKey busstopapi.BusStopKey, dur time.Duration) (reserved bool, err error)                               // MARKER: TryReserve
	TryBulkReserve(ctx context.Context, objKeys []busstopapi.BusStopKey, dur time.Duration) (reservedKeys []busstopapi.BusStopKey, err error) // MARKER: TryBulkReserve
	Reserve(ctx context.Context, objKey busstopapi.BusStopKey, dur time.Duration) (reserved bool, err error)                                  // MARKER: Reserve
	BulkReserve(ctx context.Context, objKeys []busstopapi.BusStopKey, dur time.Duration) (reservedKeys []busstopapi.BusStopKey, err error)    // MARKER: BulkReserve
}

// NewService creates a new instance of the microservice.
func NewService() *Service {
	svc := &Service{}
	svc.Intermediate = NewIntermediate(svc)
	return svc
}

// Init enables a single-statement pattern for initializing the microservice.
func (svc *Service) Init(initializer func(svc *Service) (err error)) *Service {
	svc.Connector.Init(func(_ *connector.Connector) (err error) {
		return initializer(svc)
	})
	return svc
}

// Intermediate extends and customizes the generic base connector.
type Intermediate struct {
	*connector.Connector
	ToDo
}

// NewIntermediate creates a new instance of the intermediate.
func NewIntermediate(impl ToDo) *Intermediate {
	svc := &Intermediate{
		Connector: connector.New(Hostname),
		ToDo:      impl,
	}
	svc.SetVersion(Version)
	svc.SetDescription(`BusStop persists bus stops in a SQL database.`)
	svc.SetOnStartup(svc.OnStartup)
	svc.SetOnShutdown(svc.OnShutdown)
	svc.SetResFS(resources.FS)
	svc.SetOnObserveMetrics(svc.doOnObserveMetrics)
	svc.SetOnConfigChanged(svc.doOnConfigChanged)

	// Functional endpoints
	svc.Subscribe( // MARKER: Create
		"Create", svc.doCreate,
		sub.At(busstopapi.Create.Method, busstopapi.Create.Route),
		sub.Description(`Create creates a new object, returning its key.`),
		sub.Function(busstopapi.CreateIn{}, busstopapi.CreateOut{}),
	)
	svc.Subscribe( // MARKER: Store
		"Store", svc.doStore,
		sub.At(busstopapi.Store.Method, busstopapi.Store.Route),
		sub.Description(`Store updates the object.`),
		sub.Function(busstopapi.StoreIn{}, busstopapi.StoreOut{}),
	)
	svc.Subscribe( // MARKER: MustStore
		"MustStore", svc.doMustStore,
		sub.At(busstopapi.MustStore.Method, busstopapi.MustStore.Route),
		sub.Description(`MustStore updates the object.`),
		sub.Function(busstopapi.MustStoreIn{}, busstopapi.MustStoreOut{}),
	)
	svc.Subscribe( // MARKER: Revise
		"Revise", svc.doRevise,
		sub.At(busstopapi.Revise.Method, busstopapi.Revise.Route),
		sub.Description(`Revise updates the object only if the revision matches.`),
		sub.Function(busstopapi.ReviseIn{}, busstopapi.ReviseOut{}),
	)
	svc.Subscribe( // MARKER: MustRevise
		"MustRevise", svc.doMustRevise,
		sub.At(busstopapi.MustRevise.Method, busstopapi.MustRevise.Route),
		sub.Description(`MustRevise updates the object only if the revision matches.`),
		sub.Function(busstopapi.MustReviseIn{}, busstopapi.MustReviseOut{}),
	)
	svc.Subscribe( // MARKER: Delete
		"Delete", svc.doDelete,
		sub.At(busstopapi.Delete.Method, busstopapi.Delete.Route),
		sub.Description(`Delete deletes the object.`),
		sub.Function(busstopapi.DeleteIn{}, busstopapi.DeleteOut{}),
	)
	svc.Subscribe( // MARKER: MustDelete
		"MustDelete", svc.doMustDelete,
		sub.At(busstopapi.MustDelete.Method, busstopapi.MustDelete.Route),
		sub.Description(`MustDelete deletes the object.`),
		sub.Function(busstopapi.MustDeleteIn{}, busstopapi.MustDeleteOut{}),
	)
	svc.Subscribe( // MARKER: List
		"List", svc.doList,
		sub.At(busstopapi.List.Method, busstopapi.List.Route),
		sub.Description(`List returns the objects matching the query, and the total count of matches regardless of the limit.`),
		sub.Function(busstopapi.ListIn{}, busstopapi.ListOut{}),
	)
	svc.Subscribe( // MARKER: Lookup
		"Lookup", svc.doLookup,
		sub.At(busstopapi.Lookup.Method, busstopapi.Lookup.Route),
		sub.Description(`Lookup returns the single object matching the query. It errors if more than one object matches the query.`),
		sub.Function(busstopapi.LookupIn{}, busstopapi.LookupOut{}),
	)
	svc.Subscribe( // MARKER: MustLookup
		"MustLookup", svc.doMustLookup,
		sub.At(busstopapi.MustLookup.Method, busstopapi.MustLookup.Route),
		sub.Description(`MustLookup returns the single object matching the query. It errors unless exactly one object matches the query.`),
		sub.Function(busstopapi.MustLookupIn{}, busstopapi.MustLookupOut{}),
	)
	svc.Subscribe( // MARKER: Load
		"Load", svc.doLoad,
		sub.At(busstopapi.Load.Method, busstopapi.Load.Route),
		sub.Description(`Load returns the object associated with the key.`),
		sub.Function(busstopapi.LoadIn{}, busstopapi.LoadOut{}),
	)
	svc.Subscribe( // MARKER: MustLoad
		"MustLoad", svc.doMustLoad,
		sub.At(busstopapi.MustLoad.Method, busstopapi.MustLoad.Route),
		sub.Description(`MustLoad returns the object associated with the key. It errors if the object is not found.`),
		sub.Function(busstopapi.MustLoadIn{}, busstopapi.MustLoadOut{}),
	)
	svc.Subscribe( // MARKER: BulkLoad
		"BulkLoad", svc.doBulkLoad,
		sub.At(busstopapi.BulkLoad.Method, busstopapi.BulkLoad.Route),
		sub.Description(`BulkLoad returns the objects matching the keys.`),
		sub.Function(busstopapi.BulkLoadIn{}, busstopapi.BulkLoadOut{}),
	)
	svc.Subscribe( // MARKER: BulkDelete
		"BulkDelete", svc.doBulkDelete,
		sub.At(busstopapi.BulkDelete.Method, busstopapi.BulkDelete.Route),
		sub.Description(`BulkDelete deletes the objects matching the keys, returning the keys of the deleted objects.`),
		sub.Function(busstopapi.BulkDeleteIn{}, busstopapi.BulkDeleteOut{}),
	)
	svc.Subscribe( // MARKER: BulkCreate
		"BulkCreate", svc.doBulkCreate,
		sub.At(busstopapi.BulkCreate.Method, busstopapi.BulkCreate.Route),
		sub.Description(`BulkCreate creates multiple objects, returning their keys.`),
		sub.Function(busstopapi.BulkCreateIn{}, busstopapi.BulkCreateOut{}),
	)
	svc.Subscribe( // MARKER: BulkStore
		"BulkStore", svc.doBulkStore,
		sub.At(busstopapi.BulkStore.Method, busstopapi.BulkStore.Route),
		sub.Description(`BulkStore updates multiple objects, returning the keys of the stored objects.`),
		sub.Function(busstopapi.BulkStoreIn{}, busstopapi.BulkStoreOut{}),
	)
	svc.Subscribe( // MARKER: BulkRevise
		"BulkRevise", svc.doBulkRevise,
		sub.At(busstopapi.BulkRevise.Method, busstopapi.BulkRevise.Route),
		sub.Description(`BulkRevise updates multiple objects, returning the number of rows affected.
Only rows with matching revisions are updated.`),
		sub.Function(busstopapi.BulkReviseIn{}, busstopapi.BulkReviseOut{}),
	)
	svc.Subscribe( // MARKER: Purge
		"Purge", svc.doPurge,
		sub.At(busstopapi.Purge.Method, busstopapi.Purge.Route),
		sub.Description(`Purge deletes all objects matching the query, returning the keys of the deleted objects.`),
		sub.Function(busstopapi.PurgeIn{}, busstopapi.PurgeOut{}),
	)
	svc.Subscribe( // MARKER: Count
		"Count", svc.doCount,
		sub.At(busstopapi.Count.Method, busstopapi.Count.Route),
		sub.Description(`Count returns the number of objects matching the query, disregarding pagination.`),
		sub.Function(busstopapi.CountIn{}, busstopapi.CountOut{}),
	)
	svc.Subscribe( // MARKER: CreateREST
		"CreateREST", svc.doCreateREST,
		sub.At(busstopapi.CreateREST.Method, busstopapi.CreateREST.Route),
		sub.Description(`CreateREST creates a new bus stop via REST, returning its key.`),
		sub.Function(busstopapi.CreateRESTIn{}, busstopapi.CreateRESTOut{}),
	)
	svc.Subscribe( // MARKER: StoreREST
		"StoreREST", svc.doStoreREST,
		sub.At(busstopapi.StoreREST.Method, busstopapi.StoreREST.Route),
		sub.Description(`StoreREST updates an existing bus stop via REST.`),
		sub.Function(busstopapi.StoreRESTIn{}, busstopapi.StoreRESTOut{}),
	)
	svc.Subscribe( // MARKER: DeleteREST
		"DeleteREST", svc.doDeleteREST,
		sub.At(busstopapi.DeleteREST.Method, busstopapi.DeleteREST.Route),
		sub.Description(`DeleteREST deletes an existing bus stop via REST.`),
		sub.Function(busstopapi.DeleteRESTIn{}, busstopapi.DeleteRESTOut{}),
	)
	svc.Subscribe( // MARKER: LoadREST
		"LoadREST", svc.doLoadREST,
		sub.At(busstopapi.LoadREST.Method, busstopapi.LoadREST.Route),
		sub.Description(`LoadREST loads a bus stop by key via REST.`),
		sub.Function(busstopapi.LoadRESTIn{}, busstopapi.LoadRESTOut{}),
	)
	svc.Subscribe( // MARKER: ListREST
		"ListREST", svc.doListREST,
		sub.At(busstopapi.ListREST.Method, busstopapi.ListREST.Route),
		sub.Description(`ListREST lists bus stops matching the query via REST.`),
		sub.Function(busstopapi.ListRESTIn{}, busstopapi.ListRESTOut{}),
	)
	svc.Subscribe( // MARKER: TryReserve
		"TryReserve", svc.doTryReserve,
		sub.At(busstopapi.TryReserve.Method, busstopapi.TryReserve.Route),
		sub.Description(`TryReserve attempts to reserve a bus stop for the given duration, returning true if successful.`),
		sub.Function(busstopapi.TryReserveIn{}, busstopapi.TryReserveOut{}),
	)
	svc.Subscribe( // MARKER: TryBulkReserve
		"TryBulkReserve", svc.doTryBulkReserve,
		sub.At(busstopapi.TryBulkReserve.Method, busstopapi.TryBulkReserve.Route),
		sub.Description(`TryBulkReserve attempts to reserve bus stops for the given duration, returning the keys of those successfully reserved.
Only bus stops whose reservation has expired (reserved_before < NOW) are reserved.`),
		sub.Function(busstopapi.TryBulkReserveIn{}, busstopapi.TryBulkReserveOut{}),
	)
	svc.Subscribe( // MARKER: Reserve
		"Reserve", svc.doReserve,
		sub.At(busstopapi.Reserve.Method, busstopapi.Reserve.Route),
		sub.Description(`Reserve unconditionally reserves a bus stop for the given duration, returning true if the bus stop exists.`),
		sub.Function(busstopapi.ReserveIn{}, busstopapi.ReserveOut{}),
	)
	svc.Subscribe( // MARKER: BulkReserve
		"BulkReserve", svc.doBulkReserve,
		sub.At(busstopapi.BulkReserve.Method, busstopapi.BulkReserve.Route),
		sub.Description(`BulkReserve unconditionally reserves bus stops for the given duration, returning the keys of those that exist.`),
		sub.Function(busstopapi.BulkReserveIn{}, busstopapi.BulkReserveOut{}),
	)

	// HINT: Add web endpoints here

	// HINT: Add metrics here

	// HINT: Add tickers here

	// HINT: Add configs here
	svc.DefineConfig( // MARKER: SQLDataSourceName
		"SQLDataSourceName",
		cfg.Description(`SQLDataSourceName is the connection string of the SQL database.`),
		cfg.Secret(),
	)

	// HINT: Add inbound event sinks here

	// HINT: Add task endpoints here

	// HINT: Add graph endpoints here

	_ = marshalFunction
	return svc
}

// doOnObserveMetrics is called when metrics are produced.
func (svc *Intermediate) doOnObserveMetrics(ctx context.Context) (err error) {
	return svc.Parallel(
	// HINT: Call JIT observers to record the metric here
	)
}

// doOnConfigChanged is called when the config of the microservice changes.
func (svc *Intermediate) doOnConfigChanged(ctx context.Context, changed func(string) bool) (err error) {
	// HINT: Call named callbacks here
	return nil
}

/*
SQLDataSourceName is the connection string of the SQL database.
*/
func (svc *Intermediate) SQLDataSourceName() (value string) { // MARKER: SQLDataSourceName
	return svc.Config("SQLDataSourceName")
}

/*
SetSQLDataSourceName sets the value of the configuration property.
*/
func (svc *Intermediate) SetSQLDataSourceName(value string) (err error) { // MARKER: SQLDataSourceName
	return svc.SetConfig("SQLDataSourceName", value)
}

// doCreate handles marshaling for Create.
func (svc *Intermediate) doCreate(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Create
	var in busstopapi.CreateIn
	var out busstopapi.CreateOut
	err = marshalFunction(w, r, busstopapi.Create.Route, &in, &out, func(_ any, _ any) error {
		out.ObjKey, err = svc.Create(r.Context(), in.Obj)
		return err
	})
	return err // No trace
}

// doStore handles marshaling for Store.
func (svc *Intermediate) doStore(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Store
	var in busstopapi.StoreIn
	var out busstopapi.StoreOut
	err = marshalFunction(w, r, busstopapi.Store.Route, &in, &out, func(_ any, _ any) error {
		out.Stored, err = svc.Store(r.Context(), in.Obj)
		return err
	})
	return err // No trace
}

// doMustStore handles marshaling for MustStore.
func (svc *Intermediate) doMustStore(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: MustStore
	var in busstopapi.MustStoreIn
	var out busstopapi.MustStoreOut
	err = marshalFunction(w, r, busstopapi.MustStore.Route, &in, &out, func(_ any, _ any) error {
		err = svc.MustStore(r.Context(), in.Obj)
		return err
	})
	return err // No trace
}

// doRevise handles marshaling for Revise.
func (svc *Intermediate) doRevise(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Revise
	var in busstopapi.ReviseIn
	var out busstopapi.ReviseOut
	err = marshalFunction(w, r, busstopapi.Revise.Route, &in, &out, func(_ any, _ any) error {
		out.Revised, err = svc.Revise(r.Context(), in.Obj)
		return err
	})
	return err // No trace
}

// doMustRevise handles marshaling for MustRevise.
func (svc *Intermediate) doMustRevise(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: MustRevise
	var in busstopapi.MustReviseIn
	var out busstopapi.MustReviseOut
	err = marshalFunction(w, r, busstopapi.MustRevise.Route, &in, &out, func(_ any, _ any) error {
		err = svc.MustRevise(r.Context(), in.Obj)
		return err
	})
	return err // No trace
}

// doDelete handles marshaling for Delete.
func (svc *Intermediate) doDelete(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Delete
	var in busstopapi.DeleteIn
	var out busstopapi.DeleteOut
	err = marshalFunction(w, r, busstopapi.Delete.Route, &in, &out, func(_ any, _ any) error {
		out.Deleted, err = svc.Delete(r.Context(), in.ObjKey)
		return err
	})
	return err // No trace
}

// doMustDelete handles marshaling for MustDelete.
func (svc *Intermediate) doMustDelete(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: MustDelete
	var in busstopapi.MustDeleteIn
	var out busstopapi.MustDeleteOut
	err = marshalFunction(w, r, busstopapi.MustDelete.Route, &in, &out, func(_ any, _ any) error {
		err = svc.MustDelete(r.Context(), in.ObjKey)
		return err
	})
	return err // No trace
}

// doList handles marshaling for List.
func (svc *Intermediate) doList(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: List
	var in busstopapi.ListIn
	var out busstopapi.ListOut
	err = marshalFunction(w, r, busstopapi.List.Route, &in, &out, func(_ any, _ any) error {
		out.Objs, out.TotalCount, err = svc.List(r.Context(), in.Query)
		return err
	})
	return err // No trace
}

// doLookup handles marshaling for Lookup.
func (svc *Intermediate) doLookup(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Lookup
	var in busstopapi.LookupIn
	var out busstopapi.LookupOut
	err = marshalFunction(w, r, busstopapi.Lookup.Route, &in, &out, func(_ any, _ any) error {
		out.Obj, out.Found, err = svc.Lookup(r.Context(), in.Query)
		return err
	})
	return err // No trace
}

// doMustLookup handles marshaling for MustLookup.
func (svc *Intermediate) doMustLookup(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: MustLookup
	var in busstopapi.MustLookupIn
	var out busstopapi.MustLookupOut
	err = marshalFunction(w, r, busstopapi.MustLookup.Route, &in, &out, func(_ any, _ any) error {
		out.Obj, err = svc.MustLookup(r.Context(), in.Query)
		return err
	})
	return err // No trace
}

// doLoad handles marshaling for Load.
func (svc *Intermediate) doLoad(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Load
	var in busstopapi.LoadIn
	var out busstopapi.LoadOut
	err = marshalFunction(w, r, busstopapi.Load.Route, &in, &out, func(_ any, _ any) error {
		out.Obj, out.Found, err = svc.Load(r.Context(), in.ObjKey)
		return err
	})
	return err // No trace
}

// doMustLoad handles marshaling for MustLoad.
func (svc *Intermediate) doMustLoad(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: MustLoad
	var in busstopapi.MustLoadIn
	var out busstopapi.MustLoadOut
	err = marshalFunction(w, r, busstopapi.MustLoad.Route, &in, &out, func(_ any, _ any) error {
		out.Obj, err = svc.MustLoad(r.Context(), in.ObjKey)
		return err
	})
	return err // No trace
}

// doBulkLoad handles marshaling for BulkLoad.
func (svc *Intermediate) doBulkLoad(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: BulkLoad
	var in busstopapi.BulkLoadIn
	var out busstopapi.BulkLoadOut
	err = marshalFunction(w, r, busstopapi.BulkLoad.Route, &in, &out, func(_ any, _ any) error {
		out.Objs, err = svc.BulkLoad(r.Context(), in.ObjKeys)
		return err
	})
	return err // No trace
}

// doBulkDelete handles marshaling for BulkDelete.
func (svc *Intermediate) doBulkDelete(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: BulkDelete
	var in busstopapi.BulkDeleteIn
	var out busstopapi.BulkDeleteOut
	err = marshalFunction(w, r, busstopapi.BulkDelete.Route, &in, &out, func(_ any, _ any) error {
		out.DeletedKeys, err = svc.BulkDelete(r.Context(), in.ObjKeys)
		return err
	})
	return err // No trace
}

// doBulkCreate handles marshaling for BulkCreate.
func (svc *Intermediate) doBulkCreate(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: BulkCreate
	var in busstopapi.BulkCreateIn
	var out busstopapi.BulkCreateOut
	err = marshalFunction(w, r, busstopapi.BulkCreate.Route, &in, &out, func(_ any, _ any) error {
		out.ObjKeys, err = svc.BulkCreate(r.Context(), in.Objs)
		return err
	})
	return err // No trace
}

// doBulkStore handles marshaling for BulkStore.
func (svc *Intermediate) doBulkStore(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: BulkStore
	var in busstopapi.BulkStoreIn
	var out busstopapi.BulkStoreOut
	err = marshalFunction(w, r, busstopapi.BulkStore.Route, &in, &out, func(_ any, _ any) error {
		out.StoredKeys, err = svc.BulkStore(r.Context(), in.Objs)
		return err
	})
	return err // No trace
}

// doBulkRevise handles marshaling for BulkRevise.
func (svc *Intermediate) doBulkRevise(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: BulkRevise
	var in busstopapi.BulkReviseIn
	var out busstopapi.BulkReviseOut
	err = marshalFunction(w, r, busstopapi.BulkRevise.Route, &in, &out, func(_ any, _ any) error {
		out.RevisedKeys, err = svc.BulkRevise(r.Context(), in.Objs)
		return err
	})
	return err // No trace
}

// doPurge handles marshaling for Purge.
func (svc *Intermediate) doPurge(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Purge
	var in busstopapi.PurgeIn
	var out busstopapi.PurgeOut
	err = marshalFunction(w, r, busstopapi.Purge.Route, &in, &out, func(_ any, _ any) error {
		out.DeletedKeys, err = svc.Purge(r.Context(), in.Query)
		return err
	})
	return err // No trace
}

// doCount handles marshaling for Count.
func (svc *Intermediate) doCount(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Count
	var in busstopapi.CountIn
	var out busstopapi.CountOut
	err = marshalFunction(w, r, busstopapi.Count.Route, &in, &out, func(_ any, _ any) error {
		out.Count, err = svc.Count(r.Context(), in.Query)
		return err
	})
	return err // No trace
}

// doCreateREST handles marshaling for CreateREST.
func (svc *Intermediate) doCreateREST(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: CreateREST
	var in busstopapi.CreateRESTIn
	var out busstopapi.CreateRESTOut
	err = marshalFunction(w, r, busstopapi.CreateREST.Route, &in, &out, func(_ any, _ any) error {
		out.ObjKey, out.HTTPStatusCode, err = svc.CreateREST(r.Context(), in.HTTPRequestBody)
		return err
	})
	return err // No trace
}

// doStoreREST handles marshaling for StoreREST.
func (svc *Intermediate) doStoreREST(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: StoreREST
	var in busstopapi.StoreRESTIn
	var out busstopapi.StoreRESTOut
	err = marshalFunction(w, r, busstopapi.StoreREST.Route, &in, &out, func(_ any, _ any) error {
		out.HTTPStatusCode, err = svc.StoreREST(r.Context(), in.Key, in.HTTPRequestBody)
		return err
	})
	return err // No trace
}

// doDeleteREST handles marshaling for DeleteREST.
func (svc *Intermediate) doDeleteREST(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: DeleteREST
	var in busstopapi.DeleteRESTIn
	var out busstopapi.DeleteRESTOut
	err = marshalFunction(w, r, busstopapi.DeleteREST.Route, &in, &out, func(_ any, _ any) error {
		out.HTTPStatusCode, err = svc.DeleteREST(r.Context(), in.Key)
		return err
	})
	return err // No trace
}

// doLoadREST handles marshaling for LoadREST.
func (svc *Intermediate) doLoadREST(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: LoadREST
	var in busstopapi.LoadRESTIn
	var out busstopapi.LoadRESTOut
	err = marshalFunction(w, r, busstopapi.LoadREST.Route, &in, &out, func(_ any, _ any) error {
		out.HTTPResponseBody, out.HTTPStatusCode, err = svc.LoadREST(r.Context(), in.Key)
		return err
	})
	return err // No trace
}

// doListREST handles marshaling for ListREST.
func (svc *Intermediate) doListREST(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: ListREST
	var in busstopapi.ListRESTIn
	var out busstopapi.ListRESTOut
	err = marshalFunction(w, r, busstopapi.ListREST.Route, &in, &out, func(_ any, _ any) error {
		out.HTTPResponseBody, out.HTTPStatusCode, err = svc.ListREST(r.Context(), in.Q)
		return err
	})
	return err // No trace
}

// doTryReserve handles marshaling for TryReserve.
func (svc *Intermediate) doTryReserve(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: TryReserve
	var in busstopapi.TryReserveIn
	var out busstopapi.TryReserveOut
	err = marshalFunction(w, r, busstopapi.TryReserve.Route, &in, &out, func(_ any, _ any) error {
		out.Reserved, err = svc.TryReserve(r.Context(), in.ObjKey, in.Dur)
		return err
	})
	return err // No trace
}

// doTryBulkReserve handles marshaling for TryBulkReserve.
func (svc *Intermediate) doTryBulkReserve(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: TryBulkReserve
	var in busstopapi.TryBulkReserveIn
	var out busstopapi.TryBulkReserveOut
	err = marshalFunction(w, r, busstopapi.TryBulkReserve.Route, &in, &out, func(_ any, _ any) error {
		out.ReservedKeys, err = svc.TryBulkReserve(r.Context(), in.ObjKeys, in.Dur)
		return err
	})
	return err // No trace
}

// doReserve handles marshaling for Reserve.
func (svc *Intermediate) doReserve(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Reserve
	var in busstopapi.ReserveIn
	var out busstopapi.ReserveOut
	err = marshalFunction(w, r, busstopapi.Reserve.Route, &in, &out, func(_ any, _ any) error {
		out.Reserved, err = svc.Reserve(r.Context(), in.ObjKey, in.Dur)
		return err
	})
	return err // No trace
}

// doBulkReserve handles marshaling for BulkReserve.
func (svc *Intermediate) doBulkReserve(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: BulkReserve
	var in busstopapi.BulkReserveIn
	var out busstopapi.BulkReserveOut
	err = marshalFunction(w, r, busstopapi.BulkReserve.Route, &in, &out, func(_ any, _ any) error {
		out.ReservedKeys, err = svc.BulkReserve(r.Context(), in.ObjKeys, in.Dur)
		return err
	})
	return err // No trace
}

// marshalFunction handles marshaling for functional endpoints.
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

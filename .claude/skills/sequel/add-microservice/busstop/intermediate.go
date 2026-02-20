package busstop

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/cfg"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/fabric/openapi"
	"github.com/microbus-io/fabric/sub"
	"github.com/microbus-io/fabric/utils"

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
)

const (
	Hostname = busstopapi.Hostname
	Version  = 7
)

// ToDo is implemented by the service or mock.
// The intermediate delegates handling to this interface.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
	Create(ctx context.Context, obj *busstopapi.BusStop) (objKey busstopapi.BusStopKey, err error)                                     // MARKER: Create
	Store(ctx context.Context, obj *busstopapi.BusStop) (stored bool, err error)                                                       // MARKER: Store
	MustStore(ctx context.Context, obj *busstopapi.BusStop) (err error)                                                                // MARKER: MustStore
	Revise(ctx context.Context, obj *busstopapi.BusStop) (revised bool, err error)                                                     // MARKER: Revise
	MustRevise(ctx context.Context, obj *busstopapi.BusStop) (err error)                                                               // MARKER: MustRevise
	Delete(ctx context.Context, objKey busstopapi.BusStopKey) (deleted bool, err error)                                                // MARKER: Delete
	MustDelete(ctx context.Context, objKey busstopapi.BusStopKey) (err error)                                                          // MARKER: MustDelete
	List(ctx context.Context, query busstopapi.Query) (objs []*busstopapi.BusStop, totalCount int, err error)                          // MARKER: List
	Lookup(ctx context.Context, query busstopapi.Query) (obj *busstopapi.BusStop, found bool, err error)                               // MARKER: Lookup
	MustLookup(ctx context.Context, query busstopapi.Query) (obj *busstopapi.BusStop, err error)                                       // MARKER: MustLookup
	Load(ctx context.Context, objKey busstopapi.BusStopKey) (obj *busstopapi.BusStop, found bool, err error)                           // MARKER: Load
	MustLoad(ctx context.Context, objKey busstopapi.BusStopKey) (obj *busstopapi.BusStop, err error)                                   // MARKER: MustLoad
	BulkLoad(ctx context.Context, objKeys []busstopapi.BusStopKey) (objs []*busstopapi.BusStop, err error)                             // MARKER: BulkLoad
	BulkDelete(ctx context.Context, objKeys []busstopapi.BusStopKey) (deletedKeys []busstopapi.BusStopKey, err error)                  // MARKER: BulkDelete
	BulkCreate(ctx context.Context, objs []*busstopapi.BusStop) (objKeys []busstopapi.BusStopKey, err error)                           // MARKER: BulkCreate
	BulkStore(ctx context.Context, objs []*busstopapi.BusStop) (storedKeys []busstopapi.BusStopKey, err error)                         // MARKER: BulkStore
	BulkRevise(ctx context.Context, objs []*busstopapi.BusStop) (revisedKeys []busstopapi.BusStopKey, err error)                       // MARKER: BulkRevise
	Purge(ctx context.Context, query busstopapi.Query) (deletedKeys []busstopapi.BusStopKey, err error)                                // MARKER: Purge
	Count(ctx context.Context, query busstopapi.Query) (count int, err error)                                                          // MARKER: Count
	CreateREST(ctx context.Context, httpRequestBody *busstopapi.BusStop) (objKey busstopapi.BusStopKey, httpStatusCode int, err error) // MARKER: CreateREST
	StoreREST(ctx context.Context, key busstopapi.BusStopKey, httpRequestBody *busstopapi.BusStop) (httpStatusCode int, err error)     // MARKER: StoreREST
	DeleteREST(ctx context.Context, key busstopapi.BusStopKey) (httpStatusCode int, err error)                                         // MARKER: DeleteREST
	LoadREST(ctx context.Context, key busstopapi.BusStopKey) (httpResponseBody *busstopapi.BusStop, httpStatusCode int, err error)     // MARKER: LoadREST
	ListREST(ctx context.Context, q busstopapi.Query) (httpResponseBody []*busstopapi.BusStop, httpStatusCode int, err error)          // MARKER: ListREST
	TryReserve(ctx context.Context, objKey busstopapi.BusStopKey, dur time.Duration) (reserved bool, err error)                              // MARKER: TryReserve
	TryBulkReserve(ctx context.Context, objKeys []busstopapi.BusStopKey, dur time.Duration) (reservedKeys []busstopapi.BusStopKey, err error) // MARKER: TryBulkReserve
	Reserve(ctx context.Context, objKey busstopapi.BusStopKey, dur time.Duration) (reserved bool, err error)                                 // MARKER: Reserve
	BulkReserve(ctx context.Context, objKeys []busstopapi.BusStopKey, dur time.Duration) (reservedKeys []busstopapi.BusStopKey, err error)   // MARKER: BulkReserve
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
	svc.Subscribe("GET", `:0/openapi.json`, svc.doOpenAPI)
	svc.SetResFS(resources.FS)
	svc.SetOnObserveMetrics(svc.doOnObserveMetrics)
	svc.SetOnConfigChanged(svc.doOnConfigChanged)

	// HINT: Add functional endpoints here
	svc.Subscribe(busstopapi.Create.Method, busstopapi.Create.Route, svc.doCreate)            // MARKER: Create
	svc.Subscribe(busstopapi.Store.Method, busstopapi.Store.Route, svc.doStore)                // MARKER: Store
	svc.Subscribe(busstopapi.MustStore.Method, busstopapi.MustStore.Route, svc.doMustStore)    // MARKER: MustStore
	svc.Subscribe(busstopapi.Revise.Method, busstopapi.Revise.Route, svc.doRevise)             // MARKER: Revise
	svc.Subscribe(busstopapi.MustRevise.Method, busstopapi.MustRevise.Route, svc.doMustRevise) // MARKER: MustRevise
	svc.Subscribe(busstopapi.Delete.Method, busstopapi.Delete.Route, svc.doDelete)             // MARKER: Delete
	svc.Subscribe(busstopapi.MustDelete.Method, busstopapi.MustDelete.Route, svc.doMustDelete) // MARKER: MustDelete
	svc.Subscribe(busstopapi.List.Method, busstopapi.List.Route, svc.doList)                   // MARKER: List
	svc.Subscribe(busstopapi.Lookup.Method, busstopapi.Lookup.Route, svc.doLookup)             // MARKER: Lookup
	svc.Subscribe(busstopapi.MustLookup.Method, busstopapi.MustLookup.Route, svc.doMustLookup) // MARKER: MustLookup
	svc.Subscribe(busstopapi.Load.Method, busstopapi.Load.Route, svc.doLoad)                   // MARKER: Load
	svc.Subscribe(busstopapi.MustLoad.Method, busstopapi.MustLoad.Route, svc.doMustLoad)       // MARKER: MustLoad
	svc.Subscribe(busstopapi.BulkLoad.Method, busstopapi.BulkLoad.Route, svc.doBulkLoad)       // MARKER: BulkLoad
	svc.Subscribe(busstopapi.BulkDelete.Method, busstopapi.BulkDelete.Route, svc.doBulkDelete) // MARKER: BulkDelete
	svc.Subscribe(busstopapi.BulkCreate.Method, busstopapi.BulkCreate.Route, svc.doBulkCreate) // MARKER: BulkCreate
	svc.Subscribe(busstopapi.BulkStore.Method, busstopapi.BulkStore.Route, svc.doBulkStore)    // MARKER: BulkStore
	svc.Subscribe(busstopapi.BulkRevise.Method, busstopapi.BulkRevise.Route, svc.doBulkRevise) // MARKER: BulkRevise
	svc.Subscribe(busstopapi.Purge.Method, busstopapi.Purge.Route, svc.doPurge)                // MARKER: Purge
	svc.Subscribe(busstopapi.Count.Method, busstopapi.Count.Route, svc.doCount)                // MARKER: Count
	svc.Subscribe(busstopapi.CreateREST.Method, busstopapi.CreateREST.Route, svc.doCreateREST) // MARKER: CreateREST
	svc.Subscribe(busstopapi.StoreREST.Method, busstopapi.StoreREST.Route, svc.doStoreREST)   // MARKER: StoreREST
	svc.Subscribe(busstopapi.DeleteREST.Method, busstopapi.DeleteREST.Route, svc.doDeleteREST) // MARKER: DeleteREST
	svc.Subscribe(busstopapi.LoadREST.Method, busstopapi.LoadREST.Route, svc.doLoadREST)       // MARKER: LoadREST
	svc.Subscribe(busstopapi.ListREST.Method, busstopapi.ListREST.Route, svc.doListREST)                      // MARKER: ListREST
	svc.Subscribe(busstopapi.TryReserve.Method, busstopapi.TryReserve.Route, svc.doTryReserve)             // MARKER: TryReserve
	svc.Subscribe(busstopapi.TryBulkReserve.Method, busstopapi.TryBulkReserve.Route, svc.doTryBulkReserve) // MARKER: TryBulkReserve
	svc.Subscribe(busstopapi.Reserve.Method, busstopapi.Reserve.Route, svc.doReserve)                   // MARKER: Reserve
	svc.Subscribe(busstopapi.BulkReserve.Method, busstopapi.BulkReserve.Route, svc.doBulkReserve)       // MARKER: BulkReserve

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

	_ = marshalFunction
	return svc
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

// doOpenAPI renders the OpenAPI document of the microservice.
func (svc *Intermediate) doOpenAPI(w http.ResponseWriter, r *http.Request) (err error) {
	oapiSvc := openapi.Service{
		ServiceName: svc.Hostname(),
		Description: svc.Description(),
		Version:     svc.Version(),
		Endpoints:   []*openapi.Endpoint{},
		RemoteURI:   frame.Of(r).XForwardedFullURL(),
	}

	endpoints := []*openapi.Endpoint{
		{ // MARKER: Create
			Type:        "function",
			Name:        "Create",
			Method:      busstopapi.Create.Method,
			Route:       busstopapi.Create.Route,
			Summary:     "Create(obj *BusStop) (objKey BusStopKey)",
			Description: `Create creates a new object, returning its key.`,
			InputArgs:   busstopapi.CreateIn{},
			OutputArgs:  busstopapi.CreateOut{},
		},
		{ // MARKER: Store
			Type:        "function",
			Name:        "Store",
			Method:      busstopapi.Store.Method,
			Route:       busstopapi.Store.Route,
			Summary:     "Store(obj *BusStop) (stored bool)",
			Description: `Store updates the object.`,
			InputArgs:   busstopapi.StoreIn{},
			OutputArgs:  busstopapi.StoreOut{},
		},
		{ // MARKER: MustStore
			Type:        "function",
			Name:        "MustStore",
			Method:      busstopapi.MustStore.Method,
			Route:       busstopapi.MustStore.Route,
			Summary:     "MustStore(obj *BusStop)",
			Description: `MustStore updates the object, erroring if not found.`,
			InputArgs:   busstopapi.MustStoreIn{},
			OutputArgs:  busstopapi.MustStoreOut{},
		},
		{ // MARKER: Revise
			Type:        "function",
			Name:        "Revise",
			Method:      busstopapi.Revise.Method,
			Route:       busstopapi.Revise.Route,
			Summary:     "Revise(obj *BusStop) (revised bool)",
			Description: `Revise updates the object only if the revision matches.`,
			InputArgs:   busstopapi.ReviseIn{},
			OutputArgs:  busstopapi.ReviseOut{},
		},
		{ // MARKER: MustRevise
			Type:        "function",
			Name:        "MustRevise",
			Method:      busstopapi.MustRevise.Method,
			Route:       busstopapi.MustRevise.Route,
			Summary:     "MustRevise(obj *BusStop)",
			Description: `MustRevise updates the object only if the revision matches, erroring on conflict.`,
			InputArgs:   busstopapi.MustReviseIn{},
			OutputArgs:  busstopapi.MustReviseOut{},
		},
		{ // MARKER: Delete
			Type:        "function",
			Name:        "Delete",
			Method:      busstopapi.Delete.Method,
			Route:       busstopapi.Delete.Route,
			Summary:     "Delete(objKey BusStopKey) (deleted bool)",
			Description: `Delete deletes the object.`,
			InputArgs:   busstopapi.DeleteIn{},
			OutputArgs:  busstopapi.DeleteOut{},
		},
		{ // MARKER: MustDelete
			Type:        "function",
			Name:        "MustDelete",
			Method:      busstopapi.MustDelete.Method,
			Route:       busstopapi.MustDelete.Route,
			Summary:     "MustDelete(objKey BusStopKey)",
			Description: `MustDelete deletes the object, erroring if not found.`,
			InputArgs:   busstopapi.MustDeleteIn{},
			OutputArgs:  busstopapi.MustDeleteOut{},
		},
		{ // MARKER: List
			Type:        "function",
			Name:        "List",
			Method:      busstopapi.List.Method,
			Route:       busstopapi.List.Route,
			Summary:     "List(query Query) (objs []*BusStop, totalCount int)",
			Description: `List returns the objects matching the query, and the total count of matches regardless of the limit.`,
			InputArgs:   busstopapi.ListIn{},
			OutputArgs:  busstopapi.ListOut{},
		},
		{ // MARKER: Lookup
			Type:        "function",
			Name:        "Lookup",
			Method:      busstopapi.Lookup.Method,
			Route:       busstopapi.Lookup.Route,
			Summary:     "Lookup(query Query) (obj *BusStop, found bool)",
			Description: `Lookup returns the single object matching the query. It errors if more than one object matches the query.`,
			InputArgs:   busstopapi.LookupIn{},
			OutputArgs:  busstopapi.LookupOut{},
		},
		{ // MARKER: MustLookup
			Type:        "function",
			Name:        "MustLookup",
			Method:      busstopapi.MustLookup.Method,
			Route:       busstopapi.MustLookup.Route,
			Summary:     "MustLookup(query Query) (obj *BusStop)",
			Description: `MustLookup returns the single object matching the query. It errors unless exactly one object matches the query.`,
			InputArgs:   busstopapi.MustLookupIn{},
			OutputArgs:  busstopapi.MustLookupOut{},
		},
		{ // MARKER: Load
			Type:        "function",
			Name:        "Load",
			Method:      busstopapi.Load.Method,
			Route:       busstopapi.Load.Route,
			Summary:     "Load(objKey BusStopKey) (obj *BusStop, found bool)",
			Description: `Load returns the object associated with the key.`,
			InputArgs:   busstopapi.LoadIn{},
			OutputArgs:  busstopapi.LoadOut{},
		},
		{ // MARKER: MustLoad
			Type:        "function",
			Name:        "MustLoad",
			Method:      busstopapi.MustLoad.Method,
			Route:       busstopapi.MustLoad.Route,
			Summary:     "MustLoad(objKey BusStopKey) (obj *BusStop)",
			Description: `MustLoad returns the object associated with the key. It errors if the object is not found.`,
			InputArgs:   busstopapi.MustLoadIn{},
			OutputArgs:  busstopapi.MustLoadOut{},
		},
		{ // MARKER: BulkLoad
			Type:        "function",
			Name:        "BulkLoad",
			Method:      busstopapi.BulkLoad.Method,
			Route:       busstopapi.BulkLoad.Route,
			Summary:     "BulkLoad(objKeys []BusStopKey) (objs []*BusStop)",
			Description: `BulkLoad returns the objects matching the keys.`,
			InputArgs:   busstopapi.BulkLoadIn{},
			OutputArgs:  busstopapi.BulkLoadOut{},
		},
		{ // MARKER: BulkDelete
			Type:        "function",
			Name:        "BulkDelete",
			Method:      busstopapi.BulkDelete.Method,
			Route:       busstopapi.BulkDelete.Route,
			Summary:     "BulkDelete(objKeys []BusStopKey) (deletedKeys []BusStopKey)",
			Description: `BulkDelete deletes the objects matching the keys, returning the keys of the deleted objects.`,
			InputArgs:   busstopapi.BulkDeleteIn{},
			OutputArgs:  busstopapi.BulkDeleteOut{},
		},
		{ // MARKER: BulkCreate
			Type:        "function",
			Name:        "BulkCreate",
			Method:      busstopapi.BulkCreate.Method,
			Route:       busstopapi.BulkCreate.Route,
			Summary:     "BulkCreate(objs []*BusStop) (objKeys []BusStopKey)",
			Description: `BulkCreate creates multiple objects, returning their keys.`,
			InputArgs:   busstopapi.BulkCreateIn{},
			OutputArgs:  busstopapi.BulkCreateOut{},
		},
		{ // MARKER: BulkStore
			Type:        "function",
			Name:        "BulkStore",
			Method:      busstopapi.BulkStore.Method,
			Route:       busstopapi.BulkStore.Route,
			Summary:     "BulkStore(objs []*BusStop) (storedKeys []BusStopKey)",
			Description: `BulkStore updates multiple objects, returning the keys of the stored objects.`,
			InputArgs:   busstopapi.BulkStoreIn{},
			OutputArgs:  busstopapi.BulkStoreOut{},
		},
		{ // MARKER: BulkRevise
			Type:        "function",
			Name:        "BulkRevise",
			Method:      busstopapi.BulkRevise.Method,
			Route:       busstopapi.BulkRevise.Route,
			Summary:     "BulkRevise(objs []*BusStop) (revisedKeys []BusStopKey)",
			Description: `BulkRevise updates multiple objects. Only rows with matching revisions are updated.`,
			InputArgs:   busstopapi.BulkReviseIn{},
			OutputArgs:  busstopapi.BulkReviseOut{},
		},
		{ // MARKER: Purge
			Type:        "function",
			Name:        "Purge",
			Method:      busstopapi.Purge.Method,
			Route:       busstopapi.Purge.Route,
			Summary:     "Purge(query Query) (deletedKeys []BusStopKey)",
			Description: `Purge deletes all objects matching the query, returning the keys of the deleted objects.`,
			InputArgs:   busstopapi.PurgeIn{},
			OutputArgs:  busstopapi.PurgeOut{},
		},
		{ // MARKER: Count
			Type:        "function",
			Name:        "Count",
			Method:      busstopapi.Count.Method,
			Route:       busstopapi.Count.Route,
			Summary:     "Count(query Query) (count int)",
			Description: `Count returns the number of objects matching the query, disregarding pagination.`,
			InputArgs:   busstopapi.CountIn{},
			OutputArgs:  busstopapi.CountOut{},
		},
		{ // MARKER: CreateREST
			Type:        "function",
			Name:        "CreateREST",
			Method:      busstopapi.CreateREST.Method,
			Route:       busstopapi.CreateREST.Route,
			Summary:     "CreateREST(*BusStop) (objKey BusStopKey)",
			Description: `CreateREST creates a new bus stop via REST, returning its key.`,
			InputArgs:   busstopapi.CreateRESTIn{},
			OutputArgs:  busstopapi.CreateRESTOut{},
		},
		{ // MARKER: StoreREST
			Type:        "function",
			Name:        "StoreREST",
			Method:      busstopapi.StoreREST.Method,
			Route:       busstopapi.StoreREST.Route,
			Summary:     "StoreREST(key BusStopKey, *BusStop)",
			Description: `StoreREST updates an existing bus stop via REST.`,
			InputArgs:   busstopapi.StoreRESTIn{},
			OutputArgs:  busstopapi.StoreRESTOut{},
		},
		{ // MARKER: DeleteREST
			Type:        "function",
			Name:        "DeleteREST",
			Method:      busstopapi.DeleteREST.Method,
			Route:       busstopapi.DeleteREST.Route,
			Summary:     "DeleteREST(key BusStopKey)",
			Description: `DeleteREST deletes an existing bus stop via REST.`,
			InputArgs:   busstopapi.DeleteRESTIn{},
			OutputArgs:  busstopapi.DeleteRESTOut{},
		},
		{ // MARKER: LoadREST
			Type:        "function",
			Name:        "LoadREST",
			Method:      busstopapi.LoadREST.Method,
			Route:       busstopapi.LoadREST.Route,
			Summary:     "LoadREST(key BusStopKey) (httpResponseBody *BusStop)",
			Description: `LoadREST loads a bus stop by key via REST.`,
			InputArgs:   busstopapi.LoadRESTIn{},
			OutputArgs:  busstopapi.LoadRESTOut{},
		},
		{ // MARKER: ListREST
			Type:        "function",
			Name:        "ListREST",
			Method:      busstopapi.ListREST.Method,
			Route:       busstopapi.ListREST.Route,
			Summary:     "ListREST(q Query) ([]*BusStop)",
			Description: `ListREST lists bus stops matching the query via REST.`,
			InputArgs:   busstopapi.ListRESTIn{},
			OutputArgs:  busstopapi.ListRESTOut{},
		},
		{ // MARKER: TryReserve
			Type:        "function",
			Name:        "TryReserve",
			Method:      busstopapi.TryReserve.Method,
			Route:       busstopapi.TryReserve.Route,
			Summary:     "TryReserve(objKey BusStopKey, dur time.Duration) (reserved bool)",
			Description: `TryReserve attempts to reserve a bus stop for the given duration, returning true if successful.`,
			InputArgs:   busstopapi.TryReserveIn{},
			OutputArgs:  busstopapi.TryReserveOut{},
		},
		{ // MARKER: TryBulkReserve
			Type:        "function",
			Name:        "TryBulkReserve",
			Method:      busstopapi.TryBulkReserve.Method,
			Route:       busstopapi.TryBulkReserve.Route,
			Summary:     "TryBulkReserve(objKeys []BusStopKey, dur time.Duration) (reservedKeys []BusStopKey)",
			Description: `TryBulkReserve attempts to reserve bus stops for the given duration, returning the keys of those successfully reserved.`,
			InputArgs:   busstopapi.TryBulkReserveIn{},
			OutputArgs:  busstopapi.TryBulkReserveOut{},
		},
		{ // MARKER: Reserve
			Type:        "function",
			Name:        "Reserve",
			Method:      busstopapi.Reserve.Method,
			Route:       busstopapi.Reserve.Route,
			Summary:     "Reserve(objKey BusStopKey, dur time.Duration) (reserved bool)",
			Description: `Reserve unconditionally reserves a bus stop for the given duration, returning true if the bus stop exists.`,
			InputArgs:   busstopapi.ReserveIn{},
			OutputArgs:  busstopapi.ReserveOut{},
		},
		{ // MARKER: BulkReserve
			Type:        "function",
			Name:        "BulkReserve",
			Method:      busstopapi.BulkReserve.Method,
			Route:       busstopapi.BulkReserve.Route,
			Summary:     "BulkReserve(objKeys []BusStopKey, dur time.Duration) (reservedKeys []BusStopKey)",
			Description: `BulkReserve unconditionally reserves bus stops for the given duration, returning the keys of those that exist.`,
			InputArgs:   busstopapi.BulkReserveIn{},
			OutputArgs:  busstopapi.BulkReserveOut{},
		},
		// HINT: Register web handlers and functional endpoints by adding them here
	}

	// Filter by the port of the request
	rePort := regexp.MustCompile(`:(` + regexp.QuoteMeta(r.URL.Port()) + `|0)(/|$)`)
	reAnyPort := regexp.MustCompile(`:[0-9]+(/|$)`)
	for _, ep := range endpoints {
		if rePort.MatchString(ep.Route) || r.URL.Port() == "443" && !reAnyPort.MatchString(ep.Route) {
			oapiSvc.Endpoints = append(oapiSvc.Endpoints, ep)
		}
	}
	if len(oapiSvc.Endpoints) == 0 {
		w.WriteHeader(http.StatusNotFound)
		return nil
	}
	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	if svc.Deployment() == connector.LOCAL {
		encoder.SetIndent("", "  ")
	}
	err = encoder.Encode(&oapiSvc)
	return errors.Trace(err)
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

package busstop

import (
	"context"
	"net/http"
	"time"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/connector"

	"github.com/microbus-io/fabric/busstop/busstopapi"
)

var (
	_ http.Request
	_ errors.TracedError
	_ busstopapi.Client
)

// Mock is a mockable version of the microservice, allowing functions, event sinks and web handlers to be mocked.
type Mock struct {
	*Intermediate
	mockCreate     func(ctx context.Context, obj *busstopapi.BusStop) (objKey busstopapi.BusStopKey, err error)                                     // MARKER: Create
	mockStore      func(ctx context.Context, obj *busstopapi.BusStop) (stored bool, err error)                                                       // MARKER: Store
	mockMustStore  func(ctx context.Context, obj *busstopapi.BusStop) (err error)                                                                    // MARKER: MustStore
	mockRevise     func(ctx context.Context, obj *busstopapi.BusStop) (revised bool, err error)                                                      // MARKER: Revise
	mockMustRevise func(ctx context.Context, obj *busstopapi.BusStop) (err error)                                                                    // MARKER: MustRevise
	mockDelete     func(ctx context.Context, objKey busstopapi.BusStopKey) (deleted bool, err error)                                                 // MARKER: Delete
	mockMustDelete func(ctx context.Context, objKey busstopapi.BusStopKey) (err error)                                                               // MARKER: MustDelete
	mockList       func(ctx context.Context, query busstopapi.Query) (objs []*busstopapi.BusStop, totalCount int, err error)                         // MARKER: List
	mockLookup     func(ctx context.Context, query busstopapi.Query) (obj *busstopapi.BusStop, found bool, err error)                                // MARKER: Lookup
	mockMustLookup func(ctx context.Context, query busstopapi.Query) (obj *busstopapi.BusStop, err error)                                            // MARKER: MustLookup
	mockLoad       func(ctx context.Context, objKey busstopapi.BusStopKey) (obj *busstopapi.BusStop, found bool, err error)                          // MARKER: Load
	mockMustLoad   func(ctx context.Context, objKey busstopapi.BusStopKey) (obj *busstopapi.BusStop, err error)                                      // MARKER: MustLoad
	mockBulkLoad   func(ctx context.Context, objKeys []busstopapi.BusStopKey) (objs []*busstopapi.BusStop, err error)                                // MARKER: BulkLoad
	mockBulkDelete func(ctx context.Context, objKeys []busstopapi.BusStopKey) (deletedKeys []busstopapi.BusStopKey, err error)                       // MARKER: BulkDelete
	mockBulkCreate func(ctx context.Context, objs []*busstopapi.BusStop) (objKeys []busstopapi.BusStopKey, err error)                                // MARKER: BulkCreate
	mockBulkStore  func(ctx context.Context, objs []*busstopapi.BusStop) (storedKeys []busstopapi.BusStopKey, err error)                             // MARKER: BulkStore
	mockBulkRevise func(ctx context.Context, objs []*busstopapi.BusStop) (revisedKeys []busstopapi.BusStopKey, err error)                            // MARKER: BulkRevise
	mockPurge      func(ctx context.Context, query busstopapi.Query) (deletedKeys []busstopapi.BusStopKey, err error)                                // MARKER: Purge
	mockCount      func(ctx context.Context, query busstopapi.Query) (count int, err error)                                                          // MARKER: Count
	mockCreateREST func(ctx context.Context, httpRequestBody *busstopapi.BusStop) (objKey busstopapi.BusStopKey, httpStatusCode int, err error)      // MARKER: CreateREST
	mockStoreREST  func(ctx context.Context, key busstopapi.BusStopKey, httpRequestBody *busstopapi.BusStop) (httpStatusCode int, err error)         // MARKER: StoreREST
	mockDeleteREST func(ctx context.Context, key busstopapi.BusStopKey) (httpStatusCode int, err error)                                              // MARKER: DeleteREST
	mockLoadREST   func(ctx context.Context, key busstopapi.BusStopKey) (httpResponseBody *busstopapi.BusStop, httpStatusCode int, err error)        // MARKER: LoadREST
	mockListREST       func(ctx context.Context, q busstopapi.Query) (httpResponseBody []*busstopapi.BusStop, httpStatusCode int, err error)             // MARKER: ListREST
	mockTryReserve     func(ctx context.Context, objKey busstopapi.BusStopKey, dur time.Duration) (reserved bool, err error)                            // MARKER: TryReserve
	mockTryBulkReserve func(ctx context.Context, objKeys []busstopapi.BusStopKey, dur time.Duration) (reservedKeys []busstopapi.BusStopKey, err error) // MARKER: TryBulkReserve
	mockReserve        func(ctx context.Context, objKey busstopapi.BusStopKey, dur time.Duration) (reserved bool, err error)                            // MARKER: Reserve
	mockBulkReserve    func(ctx context.Context, objKeys []busstopapi.BusStopKey, dur time.Duration) (reservedKeys []busstopapi.BusStopKey, err error) // MARKER: BulkReserve
}

// NewMock creates a new mockable version of the microservice.
func NewMock() *Mock {
	svc := &Mock{}
	svc.Intermediate = NewIntermediate(svc)
	svc.SetVersion(7357) // Stands for TEST
	return svc
}

// OnStartup is called when the microservice is started up.
func (svc *Mock) OnStartup(ctx context.Context) (err error) {
	if svc.Deployment() != connector.LOCAL && svc.Deployment() != connector.TESTING {
		return errors.New("mocking disallowed in %s deployment", svc.Deployment())
	}
	return nil
}

// OnShutdown is called when the microservice is shut down.
func (svc *Mock) OnShutdown(ctx context.Context) (err error) {
	return nil
}

// MockCreate sets up a mock handler for Create.
func (svc *Mock) MockCreate(handler func(ctx context.Context, obj *busstopapi.BusStop) (objKey busstopapi.BusStopKey, err error)) *Mock { // MARKER: Create
	svc.mockCreate = handler
	return svc
}

// Create executes the mock handler.
func (svc *Mock) Create(ctx context.Context, obj *busstopapi.BusStop) (objKey busstopapi.BusStopKey, err error) { // MARKER: Create
	if svc.mockCreate == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	objKey, err = svc.mockCreate(ctx, obj)
	return objKey, errors.Trace(err)
}

// MockStore sets up a mock handler for Store.
func (svc *Mock) MockStore(handler func(ctx context.Context, obj *busstopapi.BusStop) (stored bool, err error)) *Mock { // MARKER: Store
	svc.mockStore = handler
	return svc
}

// Store executes the mock handler.
func (svc *Mock) Store(ctx context.Context, obj *busstopapi.BusStop) (stored bool, err error) { // MARKER: Store
	if svc.mockStore == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	stored, err = svc.mockStore(ctx, obj)
	return stored, errors.Trace(err)
}

// MockMustStore sets up a mock handler for MustStore.
func (svc *Mock) MockMustStore(handler func(ctx context.Context, obj *busstopapi.BusStop) (err error)) *Mock { // MARKER: MustStore
	svc.mockMustStore = handler
	return svc
}

// MustStore executes the mock handler.
func (svc *Mock) MustStore(ctx context.Context, obj *busstopapi.BusStop) (err error) { // MARKER: MustStore
	if svc.mockMustStore == nil {
		return errors.New("mock not implemented", http.StatusNotImplemented)
	}
	return errors.Trace(svc.mockMustStore(ctx, obj))
}

// MockRevise sets up a mock handler for Revise.
func (svc *Mock) MockRevise(handler func(ctx context.Context, obj *busstopapi.BusStop) (revised bool, err error)) *Mock { // MARKER: Revise
	svc.mockRevise = handler
	return svc
}

// Revise executes the mock handler.
func (svc *Mock) Revise(ctx context.Context, obj *busstopapi.BusStop) (revised bool, err error) { // MARKER: Revise
	if svc.mockRevise == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	revised, err = svc.mockRevise(ctx, obj)
	return revised, errors.Trace(err)
}

// MockMustRevise sets up a mock handler for MustRevise.
func (svc *Mock) MockMustRevise(handler func(ctx context.Context, obj *busstopapi.BusStop) (err error)) *Mock { // MARKER: MustRevise
	svc.mockMustRevise = handler
	return svc
}

// MustRevise executes the mock handler.
func (svc *Mock) MustRevise(ctx context.Context, obj *busstopapi.BusStop) (err error) { // MARKER: MustRevise
	if svc.mockMustRevise == nil {
		return errors.New("mock not implemented", http.StatusNotImplemented)
	}
	return errors.Trace(svc.mockMustRevise(ctx, obj))
}

// MockDelete sets up a mock handler for Delete.
func (svc *Mock) MockDelete(handler func(ctx context.Context, objKey busstopapi.BusStopKey) (deleted bool, err error)) *Mock { // MARKER: Delete
	svc.mockDelete = handler
	return svc
}

// Delete executes the mock handler.
func (svc *Mock) Delete(ctx context.Context, objKey busstopapi.BusStopKey) (deleted bool, err error) { // MARKER: Delete
	if svc.mockDelete == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	deleted, err = svc.mockDelete(ctx, objKey)
	return deleted, errors.Trace(err)
}

// MockMustDelete sets up a mock handler for MustDelete.
func (svc *Mock) MockMustDelete(handler func(ctx context.Context, objKey busstopapi.BusStopKey) (err error)) *Mock { // MARKER: MustDelete
	svc.mockMustDelete = handler
	return svc
}

// MustDelete executes the mock handler.
func (svc *Mock) MustDelete(ctx context.Context, objKey busstopapi.BusStopKey) (err error) { // MARKER: MustDelete
	if svc.mockMustDelete == nil {
		return errors.New("mock not implemented", http.StatusNotImplemented)
	}
	return errors.Trace(svc.mockMustDelete(ctx, objKey))
}

// MockList sets up a mock handler for List.
func (svc *Mock) MockList(handler func(ctx context.Context, query busstopapi.Query) (objs []*busstopapi.BusStop, totalCount int, err error)) *Mock { // MARKER: List
	svc.mockList = handler
	return svc
}

// List executes the mock handler.
func (svc *Mock) List(ctx context.Context, query busstopapi.Query) (objs []*busstopapi.BusStop, totalCount int, err error) { // MARKER: List
	if svc.mockList == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	objs, totalCount, err = svc.mockList(ctx, query)
	return objs, totalCount, errors.Trace(err)
}

// MockLookup sets up a mock handler for Lookup.
func (svc *Mock) MockLookup(handler func(ctx context.Context, query busstopapi.Query) (obj *busstopapi.BusStop, found bool, err error)) *Mock { // MARKER: Lookup
	svc.mockLookup = handler
	return svc
}

// Lookup executes the mock handler.
func (svc *Mock) Lookup(ctx context.Context, query busstopapi.Query) (obj *busstopapi.BusStop, found bool, err error) { // MARKER: Lookup
	if svc.mockLookup == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	obj, found, err = svc.mockLookup(ctx, query)
	return obj, found, errors.Trace(err)
}

// MockMustLookup sets up a mock handler for MustLookup.
func (svc *Mock) MockMustLookup(handler func(ctx context.Context, query busstopapi.Query) (obj *busstopapi.BusStop, err error)) *Mock { // MARKER: MustLookup
	svc.mockMustLookup = handler
	return svc
}

// MustLookup executes the mock handler.
func (svc *Mock) MustLookup(ctx context.Context, query busstopapi.Query) (obj *busstopapi.BusStop, err error) { // MARKER: MustLookup
	if svc.mockMustLookup == nil {
		return nil, errors.New("mock not implemented", http.StatusNotImplemented)
	}
	obj, err = svc.mockMustLookup(ctx, query)
	return obj, errors.Trace(err)
}

// MockLoad sets up a mock handler for Load.
func (svc *Mock) MockLoad(handler func(ctx context.Context, objKey busstopapi.BusStopKey) (obj *busstopapi.BusStop, found bool, err error)) *Mock { // MARKER: Load
	svc.mockLoad = handler
	return svc
}

// Load executes the mock handler.
func (svc *Mock) Load(ctx context.Context, objKey busstopapi.BusStopKey) (obj *busstopapi.BusStop, found bool, err error) { // MARKER: Load
	if svc.mockLoad == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	obj, found, err = svc.mockLoad(ctx, objKey)
	return obj, found, errors.Trace(err)
}

// MockMustLoad sets up a mock handler for MustLoad.
func (svc *Mock) MockMustLoad(handler func(ctx context.Context, objKey busstopapi.BusStopKey) (obj *busstopapi.BusStop, err error)) *Mock { // MARKER: MustLoad
	svc.mockMustLoad = handler
	return svc
}

// MustLoad executes the mock handler.
func (svc *Mock) MustLoad(ctx context.Context, objKey busstopapi.BusStopKey) (obj *busstopapi.BusStop, err error) { // MARKER: MustLoad
	if svc.mockMustLoad == nil {
		return nil, errors.New("mock not implemented", http.StatusNotImplemented)
	}
	obj, err = svc.mockMustLoad(ctx, objKey)
	return obj, errors.Trace(err)
}

// MockBulkLoad sets up a mock handler for BulkLoad.
func (svc *Mock) MockBulkLoad(handler func(ctx context.Context, objKeys []busstopapi.BusStopKey) (objs []*busstopapi.BusStop, err error)) *Mock { // MARKER: BulkLoad
	svc.mockBulkLoad = handler
	return svc
}

// BulkLoad executes the mock handler.
func (svc *Mock) BulkLoad(ctx context.Context, objKeys []busstopapi.BusStopKey) (objs []*busstopapi.BusStop, err error) { // MARKER: BulkLoad
	if svc.mockBulkLoad == nil {
		return nil, errors.New("mock not implemented", http.StatusNotImplemented)
	}
	objs, err = svc.mockBulkLoad(ctx, objKeys)
	return objs, errors.Trace(err)
}

// MockBulkDelete sets up a mock handler for BulkDelete.
func (svc *Mock) MockBulkDelete(handler func(ctx context.Context, objKeys []busstopapi.BusStopKey) (deletedKeys []busstopapi.BusStopKey, err error)) *Mock { // MARKER: BulkDelete
	svc.mockBulkDelete = handler
	return svc
}

// BulkDelete executes the mock handler.
func (svc *Mock) BulkDelete(ctx context.Context, objKeys []busstopapi.BusStopKey) (deletedKeys []busstopapi.BusStopKey, err error) { // MARKER: BulkDelete
	if svc.mockBulkDelete == nil {
		return nil, errors.New("mock not implemented", http.StatusNotImplemented)
	}
	deletedKeys, err = svc.mockBulkDelete(ctx, objKeys)
	return deletedKeys, errors.Trace(err)
}

// MockBulkCreate sets up a mock handler for BulkCreate.
func (svc *Mock) MockBulkCreate(handler func(ctx context.Context, objs []*busstopapi.BusStop) (objKeys []busstopapi.BusStopKey, err error)) *Mock { // MARKER: BulkCreate
	svc.mockBulkCreate = handler
	return svc
}

// BulkCreate executes the mock handler.
func (svc *Mock) BulkCreate(ctx context.Context, objs []*busstopapi.BusStop) (objKeys []busstopapi.BusStopKey, err error) { // MARKER: BulkCreate
	if svc.mockBulkCreate == nil {
		return nil, errors.New("mock not implemented", http.StatusNotImplemented)
	}
	objKeys, err = svc.mockBulkCreate(ctx, objs)
	return objKeys, errors.Trace(err)
}

// MockBulkStore sets up a mock handler for BulkStore.
func (svc *Mock) MockBulkStore(handler func(ctx context.Context, objs []*busstopapi.BusStop) (storedKeys []busstopapi.BusStopKey, err error)) *Mock { // MARKER: BulkStore
	svc.mockBulkStore = handler
	return svc
}

// BulkStore executes the mock handler.
func (svc *Mock) BulkStore(ctx context.Context, objs []*busstopapi.BusStop) (storedKeys []busstopapi.BusStopKey, err error) { // MARKER: BulkStore
	if svc.mockBulkStore == nil {
		return nil, errors.New("mock not implemented", http.StatusNotImplemented)
	}
	storedKeys, err = svc.mockBulkStore(ctx, objs)
	return storedKeys, errors.Trace(err)
}

// MockBulkRevise sets up a mock handler for BulkRevise.
func (svc *Mock) MockBulkRevise(handler func(ctx context.Context, objs []*busstopapi.BusStop) (revisedKeys []busstopapi.BusStopKey, err error)) *Mock { // MARKER: BulkRevise
	svc.mockBulkRevise = handler
	return svc
}

// BulkRevise executes the mock handler.
func (svc *Mock) BulkRevise(ctx context.Context, objs []*busstopapi.BusStop) (revisedKeys []busstopapi.BusStopKey, err error) { // MARKER: BulkRevise
	if svc.mockBulkRevise == nil {
		return nil, errors.New("mock not implemented", http.StatusNotImplemented)
	}
	revisedKeys, err = svc.mockBulkRevise(ctx, objs)
	return revisedKeys, errors.Trace(err)
}

// MockPurge sets up a mock handler for Purge.
func (svc *Mock) MockPurge(handler func(ctx context.Context, query busstopapi.Query) (deletedKeys []busstopapi.BusStopKey, err error)) *Mock { // MARKER: Purge
	svc.mockPurge = handler
	return svc
}

// Purge executes the mock handler.
func (svc *Mock) Purge(ctx context.Context, query busstopapi.Query) (deletedKeys []busstopapi.BusStopKey, err error) { // MARKER: Purge
	if svc.mockPurge == nil {
		return nil, errors.New("mock not implemented", http.StatusNotImplemented)
	}
	deletedKeys, err = svc.mockPurge(ctx, query)
	return deletedKeys, errors.Trace(err)
}

// MockCount sets up a mock handler for Count.
func (svc *Mock) MockCount(handler func(ctx context.Context, query busstopapi.Query) (count int, err error)) *Mock { // MARKER: Count
	svc.mockCount = handler
	return svc
}

// Count executes the mock handler.
func (svc *Mock) Count(ctx context.Context, query busstopapi.Query) (count int, err error) { // MARKER: Count
	if svc.mockCount == nil {
		return 0, errors.New("mock not implemented", http.StatusNotImplemented)
	}
	count, err = svc.mockCount(ctx, query)
	return count, errors.Trace(err)
}

// MockCreateREST sets up a mock handler for CreateREST.
func (svc *Mock) MockCreateREST(handler func(ctx context.Context, httpRequestBody *busstopapi.BusStop) (objKey busstopapi.BusStopKey, httpStatusCode int, err error)) *Mock { // MARKER: CreateREST
	svc.mockCreateREST = handler
	return svc
}

// CreateREST executes the mock handler.
func (svc *Mock) CreateREST(ctx context.Context, httpRequestBody *busstopapi.BusStop) (objKey busstopapi.BusStopKey, httpStatusCode int, err error) { // MARKER: CreateREST
	if svc.mockCreateREST == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	objKey, httpStatusCode, err = svc.mockCreateREST(ctx, httpRequestBody)
	return objKey, httpStatusCode, errors.Trace(err)
}

// MockStoreREST sets up a mock handler for StoreREST.
func (svc *Mock) MockStoreREST(handler func(ctx context.Context, key busstopapi.BusStopKey, httpRequestBody *busstopapi.BusStop) (httpStatusCode int, err error)) *Mock { // MARKER: StoreREST
	svc.mockStoreREST = handler
	return svc
}

// StoreREST executes the mock handler.
func (svc *Mock) StoreREST(ctx context.Context, key busstopapi.BusStopKey, httpRequestBody *busstopapi.BusStop) (httpStatusCode int, err error) { // MARKER: StoreREST
	if svc.mockStoreREST == nil {
		return 0, errors.New("mock not implemented", http.StatusNotImplemented)
	}
	httpStatusCode, err = svc.mockStoreREST(ctx, key, httpRequestBody)
	return httpStatusCode, errors.Trace(err)
}

// MockDeleteREST sets up a mock handler for DeleteREST.
func (svc *Mock) MockDeleteREST(handler func(ctx context.Context, key busstopapi.BusStopKey) (httpStatusCode int, err error)) *Mock { // MARKER: DeleteREST
	svc.mockDeleteREST = handler
	return svc
}

// DeleteREST executes the mock handler.
func (svc *Mock) DeleteREST(ctx context.Context, key busstopapi.BusStopKey) (httpStatusCode int, err error) { // MARKER: DeleteREST
	if svc.mockDeleteREST == nil {
		return 0, errors.New("mock not implemented", http.StatusNotImplemented)
	}
	httpStatusCode, err = svc.mockDeleteREST(ctx, key)
	return httpStatusCode, errors.Trace(err)
}

// MockLoadREST sets up a mock handler for LoadREST.
func (svc *Mock) MockLoadREST(handler func(ctx context.Context, key busstopapi.BusStopKey) (httpResponseBody *busstopapi.BusStop, httpStatusCode int, err error)) *Mock { // MARKER: LoadREST
	svc.mockLoadREST = handler
	return svc
}

// LoadREST executes the mock handler.
func (svc *Mock) LoadREST(ctx context.Context, key busstopapi.BusStopKey) (httpResponseBody *busstopapi.BusStop, httpStatusCode int, err error) { // MARKER: LoadREST
	if svc.mockLoadREST == nil {
		return nil, 0, errors.New("mock not implemented", http.StatusNotImplemented)
	}
	httpResponseBody, httpStatusCode, err = svc.mockLoadREST(ctx, key)
	return httpResponseBody, httpStatusCode, errors.Trace(err)
}

// MockListREST sets up a mock handler for ListREST.
func (svc *Mock) MockListREST(handler func(ctx context.Context, q busstopapi.Query) (httpResponseBody []*busstopapi.BusStop, httpStatusCode int, err error)) *Mock { // MARKER: ListREST
	svc.mockListREST = handler
	return svc
}

// ListREST executes the mock handler.
func (svc *Mock) ListREST(ctx context.Context, q busstopapi.Query) (httpResponseBody []*busstopapi.BusStop, httpStatusCode int, err error) { // MARKER: ListREST
	if svc.mockListREST == nil {
		return nil, 0, errors.New("mock not implemented", http.StatusNotImplemented)
	}
	httpResponseBody, httpStatusCode, err = svc.mockListREST(ctx, q)
	return httpResponseBody, httpStatusCode, errors.Trace(err)
}

// MockTryReserve sets up a mock handler for TryReserve.
func (svc *Mock) MockTryReserve(handler func(ctx context.Context, objKey busstopapi.BusStopKey, dur time.Duration) (reserved bool, err error)) *Mock { // MARKER: TryReserve
	svc.mockTryReserve = handler
	return svc
}

// TryReserve executes the mock handler.
func (svc *Mock) TryReserve(ctx context.Context, objKey busstopapi.BusStopKey, dur time.Duration) (reserved bool, err error) { // MARKER: TryReserve
	if svc.mockTryReserve == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	reserved, err = svc.mockTryReserve(ctx, objKey, dur)
	return reserved, errors.Trace(err)
}

// MockTryBulkReserve sets up a mock handler for TryBulkReserve.
func (svc *Mock) MockTryBulkReserve(handler func(ctx context.Context, objKeys []busstopapi.BusStopKey, dur time.Duration) (reservedKeys []busstopapi.BusStopKey, err error)) *Mock { // MARKER: TryBulkReserve
	svc.mockTryBulkReserve = handler
	return svc
}

// TryBulkReserve executes the mock handler.
func (svc *Mock) TryBulkReserve(ctx context.Context, objKeys []busstopapi.BusStopKey, dur time.Duration) (reservedKeys []busstopapi.BusStopKey, err error) { // MARKER: TryBulkReserve
	if svc.mockTryBulkReserve == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	reservedKeys, err = svc.mockTryBulkReserve(ctx, objKeys, dur)
	return reservedKeys, errors.Trace(err)
}

// MockReserve sets up a mock handler for Reserve.
func (svc *Mock) MockReserve(handler func(ctx context.Context, objKey busstopapi.BusStopKey, dur time.Duration) (reserved bool, err error)) *Mock { // MARKER: Reserve
	svc.mockReserve = handler
	return svc
}

// Reserve executes the mock handler.
func (svc *Mock) Reserve(ctx context.Context, objKey busstopapi.BusStopKey, dur time.Duration) (reserved bool, err error) { // MARKER: Reserve
	if svc.mockReserve == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	reserved, err = svc.mockReserve(ctx, objKey, dur)
	return reserved, errors.Trace(err)
}

// MockBulkReserve sets up a mock handler for BulkReserve.
func (svc *Mock) MockBulkReserve(handler func(ctx context.Context, objKeys []busstopapi.BusStopKey, dur time.Duration) (reservedKeys []busstopapi.BusStopKey, err error)) *Mock { // MARKER: BulkReserve
	svc.mockBulkReserve = handler
	return svc
}

// BulkReserve executes the mock handler.
func (svc *Mock) BulkReserve(ctx context.Context, objKeys []busstopapi.BusStopKey, dur time.Duration) (reservedKeys []busstopapi.BusStopKey, err error) { // MARKER: BulkReserve
	if svc.mockBulkReserve == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	reservedKeys, err = svc.mockBulkReserve(ctx, objKeys, dur)
	return reservedKeys, errors.Trace(err)
}

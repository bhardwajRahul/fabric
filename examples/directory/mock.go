package directory

import (
	"context"
	"net/http"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/connector"

	"github.com/microbus-io/fabric/examples/directory/directoryapi"
)

var (
	_ http.Request
	_ errors.TracedError
	_ directoryapi.Client
)

// Mock is a mockable version of the microservice, allowing functions, event sinks and web handlers to be mocked.
type Mock struct {
	*Intermediate
	mockCreate      func(ctx context.Context, httpRequestBody directoryapi.Person) (key directoryapi.PersonKey, err error)             // MARKER: Create
	mockLoad        func(ctx context.Context, key directoryapi.PersonKey) (httpResponseBody directoryapi.Person, err error)             // MARKER: Load
	mockDelete      func(ctx context.Context, key directoryapi.PersonKey) (err error)                                                  // MARKER: Delete
	mockUpdate      func(ctx context.Context, key directoryapi.PersonKey, httpRequestBody directoryapi.Person) (err error)              // MARKER: Update
	mockLoadByEmail func(ctx context.Context, email string) (httpResponseBody directoryapi.Person, err error)                           // MARKER: LoadByEmail
	mockList        func(ctx context.Context) (httpResponseBody []directoryapi.PersonKey, err error)                                    // MARKER: List
	mockWebUI       func(w http.ResponseWriter, r *http.Request) (err error)                                                           // MARKER: WebUI
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
func (svc *Mock) MockCreate(handler func(ctx context.Context, httpRequestBody directoryapi.Person) (key directoryapi.PersonKey, err error)) *Mock { // MARKER: Create
	svc.mockCreate = handler
	return svc
}

// Create executes the mock handler.
func (svc *Mock) Create(ctx context.Context, httpRequestBody directoryapi.Person) (key directoryapi.PersonKey, err error) { // MARKER: Create
	if svc.mockCreate == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	key, err = svc.mockCreate(ctx, httpRequestBody)
	return key, errors.Trace(err)
}

// MockLoad sets up a mock handler for Load.
func (svc *Mock) MockLoad(handler func(ctx context.Context, key directoryapi.PersonKey) (httpResponseBody directoryapi.Person, err error)) *Mock { // MARKER: Load
	svc.mockLoad = handler
	return svc
}

// Load executes the mock handler.
func (svc *Mock) Load(ctx context.Context, key directoryapi.PersonKey) (httpResponseBody directoryapi.Person, err error) { // MARKER: Load
	if svc.mockLoad == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	httpResponseBody, err = svc.mockLoad(ctx, key)
	return httpResponseBody, errors.Trace(err)
}

// MockDelete sets up a mock handler for Delete.
func (svc *Mock) MockDelete(handler func(ctx context.Context, key directoryapi.PersonKey) (err error)) *Mock { // MARKER: Delete
	svc.mockDelete = handler
	return svc
}

// Delete executes the mock handler.
func (svc *Mock) Delete(ctx context.Context, key directoryapi.PersonKey) (err error) { // MARKER: Delete
	if svc.mockDelete == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	err = svc.mockDelete(ctx, key)
	return errors.Trace(err)
}

// MockUpdate sets up a mock handler for Update.
func (svc *Mock) MockUpdate(handler func(ctx context.Context, key directoryapi.PersonKey, httpRequestBody directoryapi.Person) (err error)) *Mock { // MARKER: Update
	svc.mockUpdate = handler
	return svc
}

// Update executes the mock handler.
func (svc *Mock) Update(ctx context.Context, key directoryapi.PersonKey, httpRequestBody directoryapi.Person) (err error) { // MARKER: Update
	if svc.mockUpdate == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	err = svc.mockUpdate(ctx, key, httpRequestBody)
	return errors.Trace(err)
}

// MockLoadByEmail sets up a mock handler for LoadByEmail.
func (svc *Mock) MockLoadByEmail(handler func(ctx context.Context, email string) (httpResponseBody directoryapi.Person, err error)) *Mock { // MARKER: LoadByEmail
	svc.mockLoadByEmail = handler
	return svc
}

// LoadByEmail executes the mock handler.
func (svc *Mock) LoadByEmail(ctx context.Context, email string) (httpResponseBody directoryapi.Person, err error) { // MARKER: LoadByEmail
	if svc.mockLoadByEmail == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	httpResponseBody, err = svc.mockLoadByEmail(ctx, email)
	return httpResponseBody, errors.Trace(err)
}

// MockList sets up a mock handler for List.
func (svc *Mock) MockList(handler func(ctx context.Context) (httpResponseBody []directoryapi.PersonKey, err error)) *Mock { // MARKER: List
	svc.mockList = handler
	return svc
}

// List executes the mock handler.
func (svc *Mock) List(ctx context.Context) (httpResponseBody []directoryapi.PersonKey, err error) { // MARKER: List
	if svc.mockList == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	httpResponseBody, err = svc.mockList(ctx)
	return httpResponseBody, errors.Trace(err)
}

// MockWebUI sets up a mock handler for WebUI.
func (svc *Mock) MockWebUI(handler func(w http.ResponseWriter, r *http.Request) (err error)) *Mock { // MARKER: WebUI
	svc.mockWebUI = handler
	return svc
}

// WebUI executes the mock handler.
func (svc *Mock) WebUI(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: WebUI
	if svc.mockWebUI == nil {
		return errors.New("mock not implemented", http.StatusNotImplemented)
	}
	err = svc.mockWebUI(w, r)
	return errors.Trace(err)
}

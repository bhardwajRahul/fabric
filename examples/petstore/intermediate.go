package petstore

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/microbus-io/dwarf/workflow"
	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/cfg"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/fabric/sub"
	"github.com/microbus-io/fabric/utils"

	"github.com/microbus-io/fabric/examples/petstore/petstoreapi"
	"github.com/microbus-io/fabric/examples/petstore/resources"
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
	_ petstoreapi.Client
	_ *workflow.Flow
)

const (
	Hostname = petstoreapi.Hostname
	Version  = 1
)

// ToDo is implemented by the service or mock.
// The intermediate delegates handling to this interface.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
	AddPet(ctx context.Context, httpRequestBody *petstoreapi.Pet) (httpResponseBody *petstoreapi.Pet, httpStatusCode int, err error) // MARKER: AddPet
	GetPetById(ctx context.Context, petId int64) (httpResponseBody *petstoreapi.Pet, httpStatusCode int, err error)                  // MARKER: GetPetById
	UploadFile(w http.ResponseWriter, r *http.Request) (err error)                                                                   // MARKER: UploadFile
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
	svc.SetDescription(`Petstore delegates to the Swagger Petstore API.

This is a sample Pet Store Server based on the OpenAPI 3.0 specification.  You can find out more about
Swagger at [https://swagger.io](https://swagger.io). In the third iteration of the pet store, we've switched to the design first approach!
You can now help us improve the API whether it's by making changes to the definition itself or to the code.
That way, with time, we can improve the API in general, and expose some of the new features in OAS3.

Some useful links:
- [The Pet Store repository](https://github.com/swagger-api/swagger-petstore)
- [The source API definition for the Pet Store](https://github.com/swagger-api/swagger-petstore/blob/master/src/main/resources/openapi.yaml)`)
	svc.SetOnStartup(svc.OnStartup)
	svc.SetOnShutdown(svc.OnShutdown)
	svc.SetResFS(resources.FS)
	svc.SetOnObserveMetrics(svc.doOnObserveMetrics)
	svc.SetOnConfigChanged(svc.doOnConfigChanged)

	// HINT: Add functional endpoints here
	svc.Subscribe( // MARKER: AddPet
		"AddPet", svc.doAddPet,
		sub.At(petstoreapi.AddPet.Method, petstoreapi.AddPet.Route),
		sub.Description(`AddPet adds a new pet to the store.

Input:
  - httpRequestBody: httpRequestBody is the pet to add

Output:
  - httpResponseBody: httpResponseBody is the created pet
  - httpStatusCode: httpStatusCode is the remote HTTP status code`),
		sub.Function(petstoreapi.AddPetIn{}, petstoreapi.AddPetOut{}),
	)
	svc.Subscribe( // MARKER: GetPetById
		"GetPetById", svc.doGetPetById,
		sub.At(petstoreapi.GetPetById.Method, petstoreapi.GetPetById.Route),
		sub.Description(`GetPetById returns a single pet.

Input:
  - petId: petId is the ID of the pet to return

Output:
  - httpResponseBody: httpResponseBody is the requested pet
  - httpStatusCode: httpStatusCode is the remote HTTP status code`),
		sub.Function(petstoreapi.GetPetByIdIn{}, petstoreapi.GetPetByIdOut{}),
	)

	// HINT: Add web endpoints here
	svc.Subscribe( // MARKER: UploadFile
		"UploadFile", svc.UploadFile,
		sub.At(petstoreapi.UploadFile.Method, petstoreapi.UploadFile.Route),
		sub.Description(`UploadFile uploads an image of the pet.`),
		sub.Web(),
	)

	// HINT: Add metrics here

	// HINT: Add tickers here

	// HINT: Add configs here
	svc.DefineConfig( // MARKER: RemoteBaseURL
		"RemoteBaseURL",
		cfg.Description(`RemoteBaseURL is the base URL of the remote Swagger Petstore API.`),
		cfg.DefaultValue("https://petstore3.swagger.io/api/v3"),
		cfg.Validation("url"),
	)
	svc.DefineConfig( // MARKER: BearerToken
		"BearerToken",
		cfg.Description(`BearerToken is the OAuth2 bearer token presented to the remote Swagger Petstore API.`),
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
RemoteBaseURL is the base URL of the remote Swagger Petstore API.
*/
func (svc *Intermediate) RemoteBaseURL() (value string) { // MARKER: RemoteBaseURL
	return svc.Config("RemoteBaseURL")
}

/*
SetRemoteBaseURL sets the value of the configuration property.
*/
func (svc *Intermediate) SetRemoteBaseURL(value string) (err error) { // MARKER: RemoteBaseURL
	return svc.SetConfig("RemoteBaseURL", value)
}

/*
BearerToken is the OAuth2 bearer token presented to the remote Swagger Petstore API.
*/
func (svc *Intermediate) BearerToken() (value string) { // MARKER: BearerToken
	return svc.Config("BearerToken")
}

/*
SetBearerToken sets the value of the configuration property.
*/
func (svc *Intermediate) SetBearerToken(value string) (err error) { // MARKER: BearerToken
	return svc.SetConfig("BearerToken", value)
}

// doAddPet handles marshaling for AddPet.
func (svc *Intermediate) doAddPet(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: AddPet
	var in petstoreapi.AddPetIn
	var out petstoreapi.AddPetOut
	err = marshalFunction(w, r, petstoreapi.AddPet.Route, &in, &out, func(_ any, _ any) error {
		out.HTTPResponseBody, out.HTTPStatusCode, err = svc.AddPet(r.Context(), in.HTTPRequestBody)
		return err // No trace
	})
	return err // No trace
}

// doGetPetById handles marshaling for GetPetById.
func (svc *Intermediate) doGetPetById(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: GetPetById
	var in petstoreapi.GetPetByIdIn
	var out petstoreapi.GetPetByIdOut
	err = marshalFunction(w, r, petstoreapi.GetPetById.Route, &in, &out, func(_ any, _ any) error {
		out.HTTPResponseBody, out.HTTPStatusCode, err = svc.GetPetById(r.Context(), in.PetId)
		return err // No trace
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

package petstore

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/microbus-io/dwarf/workflow"
	"github.com/microbus-io/errors"

	"github.com/microbus-io/fabric/coreservices/httpegress/httpegressapi"
	"github.com/microbus-io/fabric/examples/petstore/petstoreapi"
	"github.com/microbus-io/fabric/httpx"
)

var (
	_ context.Context
	_ http.Request
	_ errors.TracedError
	_ *workflow.Flow
	_ petstoreapi.Client
)

/*
Service implements petstore which delegates to the Swagger Petstore API.

This is a sample Pet Store Server based on the OpenAPI 3.0 specification.  You can find out more about
Swagger at [https://swagger.io](https://swagger.io). In the third iteration of the pet store, we've switched to the design first approach!
You can now help us improve the API whether it's by making changes to the definition itself or to the code.
That way, with time, we can improve the API in general, and expose some of the new features in OAS3.

Some useful links:
- [The Pet Store repository](https://github.com/swagger-api/swagger-petstore)
- [The source API definition for the Pet Store](https://github.com/swagger-api/swagger-petstore/blob/master/src/main/resources/openapi.yaml)
*/
type Service struct {
	*Intermediate // IMPORTANT: Do not remove

	// HINT: Add member variables here
}

// OnStartup is called when the microservice is started up.
func (svc *Service) OnStartup(ctx context.Context) (err error) {
	return nil
}

// OnShutdown is called when the microservice is shut down.
func (svc *Service) OnShutdown(ctx context.Context) (err error) {
	return nil
}

/*
AddPet adds a new pet to the store.

Input:
  - httpRequestBody: httpRequestBody is the pet to add

Output:
  - httpResponseBody: httpResponseBody is the created pet
  - httpStatusCode: httpStatusCode is the remote HTTP status code
*/
func (svc *Service) AddPet(ctx context.Context, httpRequestBody *petstoreapi.Pet) (httpResponseBody *petstoreapi.Pet, httpStatusCode int, err error) { // MARKER: AddPet
	httpStatusCode, err = svc.makeFunctionRequest(ctx, "POST", svc.remoteURL("/pet"), httpRequestBody, &httpResponseBody)
	return httpResponseBody, httpStatusCode, errors.Trace(err)
}

/*
GetPetById returns a single pet.

Input:
  - petId: petId is the ID of the pet to return

Output:
  - httpResponseBody: httpResponseBody is the requested pet
  - httpStatusCode: httpStatusCode is the remote HTTP status code
*/
func (svc *Service) GetPetById(ctx context.Context, petId int64) (httpResponseBody *petstoreapi.Pet, httpStatusCode int, err error) { // MARKER: GetPetById
	u, err := url.Parse(svc.remoteURL("/pet/" + url.PathEscape(fmt.Sprint(petId))))
	if err != nil {
		return nil, 0, errors.Trace(err)
	}
	httpStatusCode, err = svc.makeFunctionRequest(ctx, "GET", u.String(), nil, &httpResponseBody)
	return httpResponseBody, httpStatusCode, errors.Trace(err)
}

/*
UploadFile uploads an image of the pet.
*/
func (svc *Service) UploadFile(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: UploadFile
	u := svc.remoteURL("/pet/" + url.PathEscape(r.PathValue("petId")) + "/uploadImage")
	return svc.makeWebRequest(w, r, "POST", u)
}

// remoteURL joins the configured base with an operation path, tolerating a configured base that
// has (or lacks) a trailing slash. path must start with "/".
func (svc *Service) remoteURL(path string) string {
	return strings.TrimRight(svc.RemoteBaseURL(), "/") + path
}

// authenticate injects the remote credential per the specs remote.security.
func (svc *Service) authenticate(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+svc.BearerToken())
}

// makeFunctionRequest forwards a typed call to the remote API: it sends method+url with an optional
// JSON-encoded in body, decodes a JSON response into out when out is non-nil, and returns the remote
// status code unchanged.
func (svc *Service) makeFunctionRequest(ctx context.Context, method, rawURL string, in, out any) (status int, err error) {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return 0, errors.Trace(err)
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, rawURL, body)
	if err != nil {
		return 0, errors.Trace(err)
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	svc.authenticate(req)
	resp, err := httpegressapi.NewClient(svc).Do(ctx, req)
	if err != nil {
		return 0, errors.Trace(err)
	}
	defer resp.Body.Close()
	if out != nil {
		err = json.NewDecoder(resp.Body).Decode(out)
		if err != nil {
			return resp.StatusCode, errors.Trace(err)
		}
	}
	return resp.StatusCode, nil
}

// makeWebRequest forwards a raw web call (caller's query string and body included) and relays the
// remote response to w unchanged. httpx.Copy transfers the body buffer without copying its bytes.
func (svc *Service) makeWebRequest(w http.ResponseWriter, r *http.Request, method, rawURL string) (err error) {
	req, err := http.NewRequest(method, rawURL, r.Body)
	if err != nil {
		return errors.Trace(err)
	}
	req.URL.RawQuery = r.URL.RawQuery
	req.Header = r.Header.Clone()
	svc.authenticate(req)
	resp, err := httpegressapi.NewClient(svc).Do(r.Context(), req)
	if err != nil {
		return errors.Trace(err)
	}
	defer resp.Body.Close()
	return errors.Trace(httpx.Copy(w, resp))
}

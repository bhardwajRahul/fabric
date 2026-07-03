---
name: import-openapi-microservice
description: TRIGGER when the user asks to delegate to or integrate an external/third-party REST API, import an OpenAPI/Swagger document, or generate a microservice from an OpenAPI spec (by URL or pasted JSON/YAML). Produces a one-for-one delegating microservice and stores openapispecs.json in its directory so more endpoints can be imported later.
---

**CRITICAL**: This skill does NOT define the shape of generated code. It delegates every feature to the
`add-microservice`, `add-function`, `add-web`, and `add-config` skills, the single source of truth for code
shape. This skill only scaffolds the microservice, produces the specs file, picks endpoints, and fills the
one-line delegation body.

**CRITICAL**: The microservice is a faithful one-for-one delegator. Do NOT reshape the remote API: never
split a request body into separate arguments, rename routes, or collapse status codes. Body-bearing
operations use the magic HTTP arguments so the microservice is byte-transparent to its callers.

**CRITICAL**: `openapispecs.json` lives in the microservice's directory and is tracked in source control
(never git-ignored). It is the durable record of the remote API. Re-running this skill imports more
endpoints from the present specs without re-fetching, and skips endpoints already scaffolded.

## Workflow

Copy this checklist and track your progress:

```
- [ ] Step 1: Locate or scaffold the microservice
- [ ] Step 2: Acquire the OpenAPI document
- [ ] Step 3: Generate openapispecs.json
- [ ] Step 4: Add the base-URL and credential configs
- [ ] Step 5: Add the shared request helpers (once)
- [ ] Step 6: Select which endpoints to import
- [ ] Step 7: Generate the complex types
- [ ] Step 8: Declare the endpoints in definition.go
- [ ] Step 9: Generate the boilerplate
- [ ] Step 10: Implement the handler bodies
- [ ] Step 11: Wire the HTTP egress proxy into the app
- [ ] Step 12: Housekeeping
```

The placeholders `myservice`, `MyService`, `myserviceapi` stand for the microservice being generated.

#### Step 1: Locate or Scaffold the Microservice

If a delegator for this API already exists, its `openapispecs.json` and local `CLAUDE.md` record what is
already imported. If the user only wants *more* endpoints, skip to Step 6 (the specs are already present;
do not re-fetch unless the user wants a newer version of the document).

Otherwise follow the `add-microservice` skill. Name the microservice after the remote API itself
(`petstore`, not `petstorewrap`/`petstoreproxy`/`petstoreclient`): in the mesh this microservice *is* that
API's representation, and a suffix leaks an implementation detail into a hostname every caller sees.
"Delegator" describes what it does, never what it is named. Give it a one-line description
(`MyService delegates to the Petstore API.`); Step 3 appends the API's own overview.

#### Step 2: Acquire the OpenAPI Document

Place the document in the microservice directory as `myservice/openapi.src`:

- **By URL**: `curl -fsSL '<url>' -o myservice/openapi.src`. Remember the URL for Step 3.
- **Pasted**: write the pasted content verbatim to `myservice/openapi.src`.

Do not hand-edit it. JSON and YAML are both accepted.

#### Step 3: Generate `openapispecs.json`

```shell
go run github.com/microbus-io/fabric/cmd/genopenapispecs -base-url 'ORIGIN' < myservice/openapi.src > myservice/openapispecs.json
rm myservice/openapi.src
```

The tool is a pure offline filter (`stdin` to `stdout`; never touches the network).

`-base-url` is optional and defaults to the document's first `servers` URL. Real documents often declare a
*relative* server (e.g. `/api/v3`); the tool then warns on `stderr`. Set `-base-url` to that URL's origin
(`scheme://host`) joined with the relative server path, e.g. `https://petstore3.swagger.io/api/v3`. If the
document was pasted with no absolute server and you have no URL, ask the user for the base URL.

The specs file has `info`, `remote{baseURL,security}`, `types{}` (raw JSON Schema keyed by name), and
`endpoints[]` (each with `name, feature, method, route, summary, description, params, requestBody, response`).
`feature` is `function` (JSON or empty body) or `web` (non-JSON body); trust it unless an endpoint is obviously misclassified.

If the specs `info.description` is non-empty, append it to the `Service` godoc and `svc.SetDescription`
after the one-line summary from Step 1, separated by a blank line so it renders as a second godoc
paragraph.

#### Step 4: Add the Base-URL and Credential Configs

Follow the `add-config` skill:

1. **`RemoteBaseURL`**: default `remote.baseURL` from the specs, validation `url`.
2. **Credential**, `secret: true`, no default. Only when `remote.security` is present. Name it for the scheme: `APIKey` (`apiKey`), `BearerToken` (`http-bearer`/`oauth2`), or `BasicAuth` (`http-basic`, value is the `user:password` pair).

#### Step 5: Add the Shared Request Helpers (Once)

Add these helpers to `service.go` exactly once, not per endpoint. They are the only delegation logic;
every handler becomes a single call into them. Add imports `bytes`, `context`, `encoding/json`, `io`,
`net/http`, `strings`, `errors`, `github.com/microbus-io/fabric/httpx`, and the `httpegressapi` package.

```go
// remoteURL joins the configured base with an operation path, tolerating a configured base that
// has (or lacks) a trailing slash. path must start with "/".
func (svc *Service) remoteURL(path string) string {
	return strings.TrimRight(svc.RemoteBaseURL(), "/") + path
}

// authenticate injects the remote credential per the specs remote.security. Write its body once,
// keeping only the line for this API's scheme:
//   apiKey header -> req.Header.Set("<name>", svc.APIKey())
//   apiKey query  -> q := req.URL.Query(); q.Set("<name>", svc.APIKey()); req.URL.RawQuery = q.Encode()
//   http-bearer/oauth2 -> req.Header.Set("Authorization", "Bearer "+svc.BearerToken())
//   http-basic -> u, p, _ := strings.Cut(svc.BasicAuth(), ":"); req.SetBasicAuth(u, p)
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
```

The status code is passed through unchanged. Do not add retries, caching, or reshaping; cross-cutting
concerns belong in the caller.

#### Step 6: Select Which Endpoints to Import

Read the candidate endpoints from `myservice/openapispecs.json` (`endpoints[]`; each has `name`, `feature`,
`method`, `route`, `summary`). This is the entry point when Step 1 sent you straight here for an existing
delegator. Large APIs have hundreds of endpoints; import only the curated subset the project needs.

- **Agent-relevant** (default mid-task): select the endpoints the task requires, list them to the user with
  one-line justifications, and proceed. More can be added later from the same specs.
- **Interactive** (default for a bare "import this API"): present the endpoints grouped by route with
  `method`, `name`, `summary` and ask the user to choose. Offer "all" for small APIs.

Skip endpoints already scaffolded (their `// MARKER` comment, keyed by the endpoint name, is in
`myserviceapi/definition.go`); re-import is additive. Record the imported names under a `## Imported Endpoints` heading in the local `CLAUDE.md`.

#### Step 7: Generate the Complex Types

For every type a selected endpoint references via `$ref: 'Name'` in its `params`, `requestBody`, or
`response`, define a Go struct from `types['Name']` in the specs (a `$ref` value is the bare type name and
keys the `types` map directly; nested `$ref`s inside a type schema work the same way). Follow the
`add-function` skill's "Define Complex Types" conventions (one file per type under `myserviceapi/`, `json`
camelCase `omitzero`, `jsonschema_description:"..."` from the schema's `description`) for the Go mapping.
For `oneOf`/`anyOf`/`allOf` or free-form objects with no `properties`, fall back to `json.RawMessage` and
note the fallback in the type's godoc.

#### Step 8: Declare the Endpoints in `definition.go`

Declare every selected endpoint in `myserviceapi/definition.go` before generating, so a single genservice
pass in Step 9 scaffolds all their handlers. Do NOT hand-write any `service.go` handler here: genservice
projects each handler's signature and godoc from the declaration, and you fill only the body in Step 10.

Every endpoint's `method` and `route` come from the specs verbatim (already in Microbus `{arg}` syntax). The
endpoint godoc is the specs `summary`/`description` only.

**A `{arg}` must span a whole route segment.** Microbus rejects a route where an argument shares its segment
with literal text (e.g. `/{provider}.json`), failing at subscription time - not at `cmd/genservice` or
`go vet`. When a remote route has this shape, adjust only the Microbus-facing route so each argument is a full
segment, splitting the suffix off: `/specs/{provider}/{api}.json` -> `/specs/{provider}/{api}/api.json`. The
Step 10 handler still builds the original remote URL via `svc.remoteURL(...)`, so delegation stays
byte-transparent; only the route callers use to reach this microservice changes. This is the sole sanctioned
exception to "never rename routes".

**Security (decide per endpoint, do not skip).** This microservice injects a stored secret credential
server-side, so it is exactly the confused-deputy case in `microbus.md` (Ports, Authentication and
Authorization): default to closed. Per endpoint, gate it with `RequiredClaims` and/or an internal port
(prefix the route with `:444`); leave it open on the default `:443` without `RequiredClaims` only when the
endpoint is genuinely public. Both `RequiredClaims` and the port are fields of the declaration below, so
decide them now.

**Function endpoints** (`feature: function`): follow the `add-function` skill. Append the `define.Function`
var and its In/Out structs. The In struct holds one field per `params` entry (path, query, header alike;
spec name, first letter uppercased for the Go field, camelCase `json` tag) typed from the param's `goType`
when present, else its `schema` mapped per `add-function` conventions; a path-arg field's `json` tag matches
its `{arg}` segment. The Out struct returns `httpResponseBody` of the response type plus `httpStatusCode`.
Put per-argument descriptions on the In/Out fields as `jsonschema_description:"..."` tags (from the spec
param/property `description`). The magic Go field names `HTTPRequestBody`, `HTTPResponseBody`,
`HTTPStatusCode` are matched by reflection; renaming them silently disables the magic, and each takes a
`json:"-"` tag (types are bare because `definition.go` is in the `myserviceapi` package):

```go
/*
RemoteFunction does X.
*/
var RemoteFunction = define.Function{ // MARKER: RemoteFunction
	Host: Hostname, Method: "GET", Route: "/things/{id}",
	In: RemoteFunctionIn{}, Out: RemoteFunctionOut{},
}

// RemoteFunctionIn are the input arguments of RemoteFunction.
type RemoteFunctionIn struct { // MARKER: RemoteFunction
	ID     int64  `json:"id,omitzero" jsonschema_description:"ID is the thing identifier"`
	Status string `json:"status,omitzero" jsonschema_description:"Status filters the result"`
}

// RemoteFunctionOut are the output arguments of RemoteFunction.
type RemoteFunctionOut struct { // MARKER: RemoteFunction
	HTTPResponseBody []Thing `json:"-"`
	HTTPStatusCode   int     `json:"-"`
}
```

For a body-bearing operation (`requestBody` present, e.g. a POST create), the In struct instead carries an
`HTTPRequestBody` field of the body type (`HTTPRequestBody *Thing` with a `json:"-"` tag), alongside any path
or query fields; the Step 10 handler passes it as the `in` argument to `makeFunctionRequest`.

**Web endpoints** (`feature: web`): follow the `add-web` skill. Append the `define.Web` var (a raw web
handler has no In/Out structs):

```go
/*
RemoteWeb does X.
*/
var RemoteWeb = define.Web{ // MARKER: RemoteWeb
	Host: Hostname, Method: "POST", Route: "/things/{id}/upload",
}
```

#### Step 9: Generate the Boilerplate

From the microservice's directory, run the generator. It regenerates `myserviceapi/client.go`,
`intermediate.go`, `mock.go`, `mock_test.go`, and `manifest.yaml`, and scaffolds a placeholder handler in
`service.go` for every endpoint declared in Step 8 (its signature and godoc projected from `definition.go`,
holding a `// TODO` body).

```shell
go run github.com/microbus-io/fabric/cmd/genservice .
```

Then verify the microservice compiles with `go vet ./...` from the project root.

#### Step 10: Implement the Handler Bodies

For each scaffolded handler in `service.go`, replace only its `// TODO` body with a single delegation call.
Leave the generated signature and godoc as they are: they are the contract from `definition.go`, so if a
signature is wrong, fix `definition.go` and regenerate rather than editing `service.go`.

**Function handlers**: fill with a single `makeFunctionRequest` call. Build the URL with `svc.remoteURL(...)`,
the route with path arguments substituted, and any query parameters. Pass `httpRequestBody` as the `in`
argument for a body-bearing operation, or `nil` when there is none, and `nil` for `out` when there is no
typed response:

```go
func (svc *Service) RemoteFunction(ctx context.Context, id int64, status string) (httpResponseBody []myserviceapi.Thing, httpStatusCode int, err error) { // MARKER: RemoteFunction
	u, err := url.Parse(svc.remoteURL("/things/" + url.PathEscape(fmt.Sprint(id))))
	if err != nil {
		return nil, 0, errors.Trace(err)
	}
	q := u.Query()
	q.Set("status", status) // one line per query parameter
	u.RawQuery = q.Encode()

	httpStatusCode, err = svc.makeFunctionRequest(ctx, "GET", u.String(), nil, &httpResponseBody)
	return httpResponseBody, httpStatusCode, errors.Trace(err)
}
```

**Web handlers**: fill with a single `makeWebRequest` call:

```go
func (svc *Service) RemoteWeb(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: RemoteWeb
	return svc.makeWebRequest(w, r, "POST",
		svc.remoteURL("/things/"+url.PathEscape(r.PathValue("id"))+"/upload"))
}
```

#### Step 11: Wire the HTTP Egress Proxy Into the App

Ensure the HTTP egress proxy is in `main/main.go` (add the import and `app.Add` entry if missing). This
skill generates no endpoint tests, so there is no test app to wire: a delegator's behavioral tests would
only exercise the egress mock and assert a tautology. The `add-microservice` scaffold's `service_test.go`
stays as generated (import guards only).

#### Step 12: Housekeeping

Follow the `housekeeping` skill.

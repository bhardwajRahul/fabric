---
name: upgrade-v1-27-0
user-invocable: false
description: Called by upgrade-microbus. Upgrades the project from v1.26.x to the current latest layout. Replaces svc.Subscribe(...) calls in intermediate.go with svc.Subscribe("Name", handler, sub.Method/Route/Description/<Feature>), drops the per-service doOpenAPI handler (the connector now serves :0/openapi.json built-in), drops Test*_OpenAPI tests, and migrates LLM tool callers to the []string URL contract.
---

## Background

Two waves of changes have to land together when upgrading from v1.26.x:

**Wave 1:**
- The OpenAI provider service was renamed from `openaillm` (hostname `openai.llm.core`) to `chatgptllm` (hostname `chatgpt.llm.core`).

**Wave 2:**
- Subscriptions are registered through `svc.Subscribe("Name", handler, sub.At(..., ...), sub.Description(...) sub.<Feature>(In{}, Out{}))` instead of the untyped `svc.Subscribe(method, route, handler, opts...)`. The connector keeps an `Unsubscribe(name)` to undo a `Listen`.
- The per-service `doOpenAPI` handler is gone. The connector serves `/openapi.json` built-in (initially on `:0`; moved to `:888` in Wave 3 - see below): it walks its own subscription map, filters by feature type and actor claims, and renders the OpenAPI document directly. Microservices no longer import `openapi` or render anything themselves.
- The `openapi` package now contains both the registration types (`Service`, `Endpoint`, `Document`, …) and the `Render` function - the old `openapi/doc` subpackage is gone. `openapi.Doc` was renamed to `openapi.Document`.
- A typed `controlapi.OpenAPI` endpoint was added to the control core service. Callers can fetch any host's OpenAPI document via `controlapi.NewClient(svc).WithOptions(pub.URL("https://host:port/openapi.json")).OpenAPI(ctx)` and get back a `*controlapi.Document`. (Wave 3 simplifies this to `.ForHost(host).OpenAPI(ctx)` - see below.)
- The LLM `Chat` endpoint now takes `[]string` of canonical endpoint URLs (e.g. `calculatorapi.Arithmetic.URL()`). The LLM service fetches each host's OpenAPI document, scans for the requested URL, and converts the matching `Operation` into a callable tool. `llmapi.ToolsOf` and caller-built `llmapi.Tool` literals are gone.
- Generated `Test*_OpenAPI` tests were removed from every `service_test.go`. The connector-level test in `connector/control_test.go` covers the OpenAPI surface.

**Wave 3:**
- The connector's built-in OpenAPI handler moved from `:0/openapi.json` to `:888/openapi.json` (the control plane port). It now returns endpoints across every port the service exposes, filtered by claims; consumers (the portal, the MCP wrapper, the LLM service) apply any port-based filtering at their own ingress boundary. A parallel `//all:888/openapi.json` mirror is registered with default queue (one replica per service responds) so consumers can multicast and gather every service's doc in one bus call. Externally, the per-service `:0/openapi.json` URL no longer works - external consumers go through the OpenAPI portal instead.
- Schema component keys are now prefixed with the source service's hostname (dots → underscores, double-underscore boundary): a `Foo_OUT` schema becomes `myservice_example__Foo_OUT`. This keeps multi-service aggregation collision-free. The framework's standard error schema (`error_StreamedError`) is intentionally *not* prefixed - every service emits an identical entry and the aggregator dedupes via map merge. **Any downstream code that reads `components.schemas` by hard-coded key needs the new naming.** `$ref` strings inside the doc are rewritten consistently.
- `controlapi.OpenAPI` Def's route is now `:888/openapi.json`. Fetch shape simplifies from `WithOptions(pub.URL("https://host:port/openapi.json")).OpenAPI(ctx)` to `.ForHost(host).OpenAPI(ctx)` - the path is fixed at `:888/openapi.json` so only the host needs targeting.
- The OpenAPI portal was rewritten. Its old HTML `List` endpoint at `//openapi:0` is gone, replaced by:
    - `Document` at `//openapi.json:0` - JSON aggregate covering every service, with one `tag` per source service preserving `info.description`. With `?hostname=X` it proxies to that single service. The aggregate is cached in `DistribCache` keyed by `(claims-digest, request-port)`.
    - `Explorer` at `//openapi:0` - Swagger-UI-style HTML browser. Lists services or, with `?hostname=X`, expands one service's endpoints with parameter tables and sample payloads.
    - `openapiportalapi.Client.List(...)` is gone; use `Document(...)` (or `Explorer(...)`) instead.
- A new core microservice `mcp.core` (package `coreservices/mcpportal`) exposes the bus's tools to LLM clients via the [Model Context Protocol](https://modelcontextprotocol.io). It's optional; add it to `main/main.go` only if MCP integration is needed.
- The `openapi.Document` and `openapi.Operation` types gained a `Tags` field. Existing code is unaffected.
- Path parameters no longer carry `style: deepObject, explode: true` - those are query-only per OpenAPI 3.1, and the old emission failed validation against strict OpenAPI tooling. No code change needed downstream; the rendered doc just becomes spec-compliant.
- The connector's auth path for unsigned tokens in TESTING was tightened. Previously, an unsigned JWT (e.g. minted by `pub.Actor`) would pass any `RequiredClaims` gate without the claims actually being evaluated. Now claims are enforced for unsigned tokens too, matching the signed-token path. The "no token" case is unchanged (still 401 against a claim-gated endpoint). **If existing tests passed by setting an unsigned actor with empty claims against a claim-gated endpoint**, they need to mint an actor with the matching claims (e.g. `pub.Actor(jwt.MapClaims{"roles": map[string]any{"admin": true}})`) or remove the gate.

Most of the per-microservice work is mechanical and is performed by the existing `regenerate-boilerplate` skill, whose internal steps already emit the new layout because the underlying `add-*` skills were updated.

**CRITICAL**: Run the per-microservice regeneration inline in the current agent session. Do NOT spawn subagents to migrate individual microservices in parallel. Each subagent has to re-load `.claude/rules/microbus.md`, the full `regenerate-boilerplate` skill, and the referenced `add-*` skill templates from scratch before it can do any work - which dwarfs the cost of the actual file edits and adds up to roughly an order of magnitude more tokens across a project-wide upgrade. The inline agent already has this context loaded once, so migrate services sequentially in this session.

## Workflow

Copy this checklist and track your progress:

```
Upgrade a Microbus project to the current layout:
- [ ] Step 1: Find all microservices to upgrade
- [ ] Step 2: Regenerate boilerplate per microservice
- [ ] Step 3: Migrate hand-rolled Subscribe calls outside intermediate.go
- [ ] Step 4: Update LLM tool callers
- [ ] Step 5: Rename openaillm references to chatgptllm
- [ ] Step 6: Update manifests
- [ ] Step 7: Apply Wave 3 fixups
- [ ] Step 8: Vet and test
```

#### Step 1: Find All Microservices to Upgrade

Find all microservice directories in the project that contain a `*api/client.go` file. Exclude files under `.claude/skills/` - those templates are maintained separately. A microservice is already on the latest layout if its `intermediate.go` uses the named-Subscribe form (`svc.Subscribe("Name", handler, sub.At(...), sub.<Feature>(...))`) and contains no `doOpenAPI` method.

#### Step 2: Regenerate Boilerplate Per Microservice

For each microservice that needs upgrading, pick **one** of these two paths:

**Path A - Minimal patch (preferred when the microservice only needs the Listen reshape).** Edit in place - this is dramatically faster than a full regenerate. Five edits, in order:

1. **Rewrite each `svc.Subscribe(...)` call in `intermediate.go` as a `svc.Subscribe(...)` block**. For every subscription except the openapi one (handled in step 2), the new shape is:
   ```go
   svc.Subscribe( // MARKER: Foo
"Foo", svc.doFoo,
       sub.At(myserviceapi.Foo.Method, myserviceapi.Foo.Route),
sub.Description(`Foo does X.`),
       sub.Function(myserviceapi.FooIn{}, myserviceapi.FooOut{}),
   )
   ```
   The feature option to use:
   - `sub.Function(In{}, Out{})` for functional endpoints
   - `sub.Web()` for web handlers (no In/Out)
   - `sub.Task(In{}, Out{})` for task endpoints
   - `sub.Workflow(In{}, Out{})` for workflow graphs
   - Inbound events do NOT use `svc.Subscribe` directly - they go through the source service's `Hook` (which itself calls `Listen`/`Unsubscribe` internally).

   The `sub.Description(...)` text is the godoc on the handler method in `service.go`. Carry over multi-line descriptions verbatim, including any `Input:` / `Output:` sections. Carry over `sub.RequiredClaims(...)`, `sub.NoQueue()`, `sub.Queue(...)` from any options that were on the old `Subscribe` call.

2. **Delete the `:0/openapi.json` Subscribe call and the entire `doOpenAPI` method from `intermediate.go`.** Both lines are gone - the connector serves the route built-in. Drop the now-unused imports: `frame`, `openapi`, `openapi/doc`. Bump the `Version` const by 1.

3. **Update outbound-event `Hook` methods in `*api/client.go`**. The Hook generator now uses `Listen`/`Unsubscribe` instead of `Subscribe`. For each `func (c Hook) OnFoo(...)` method, replace the body with the new pattern:
   ```go
   func (c Hook) OnFoo(handler func(...)) (unsub func() error, err error) { // MARKER: OnFoo
       doOnFoo := func(w http.ResponseWriter, r *http.Request) error { /* unchanged */ }
       const name = "OnFoo"
       path := httpx.JoinHostAndPath(c.host, OnFoo.Route)
       subOpts := append([]sub.Option{
           sub.At(OnFoo.Method, path),
sub.InboundEvent(OnFooIn{}, OnFooOut{}),
       }, c.opts...)
       if err := c.svc.Subscribe(name, doOnFoo, subOpts...); err != nil {
           return nil, errors.Trace(err)
       }
       return func() error { return c.svc.Unsubscribe(name) }, nil
   }
   ```

4. **Remove `TestMyService_OpenAPI` from `service_test.go`** if present. Drop any imports that become unused (`io`, `net/http`, `httpx`, `pub`, `regexp` are common stragglers - keep them if other tests still reference them or if they appear in the lint-suppression `var (_ ...)` block).

5. **Bump `manifest.yaml`** `frameworkVersion` to the current target version and update `modifiedAt` to the current UTC timestamp.

`service.go`, `mock.go`, `AGENTS.md`, `CLAUDE.md`, `PROMPTS.md` stay untouched in Path A.

**Path B - Full regen.** If the microservice has drifted beyond a clean Listen reshape (e.g. missing mock guards, stale import sets, outdated stub signatures), run the `regenerate-boilerplate` skill following its full workflow. The skill deletes `endpoints.go` / `client.go` / `intermediate.go` / `mock.go` / `embed.go` and recreates them by re-running the appropriate `add-*` skill steps for every feature listed in `manifest.yaml`. Because the per-feature `add-*` skills already emit the new Listen+Def layout, regeneration produces the correct shape with no extra steps. **Do not** modify `service.go`, `service_test.go`, `manifest.yaml`, `AGENTS.md`, `CLAUDE.md`, or `PROMPTS.md` during a full regen - `regenerate-boilerplate` preserves them. Path B will however leave any `Test*_OpenAPI` function in `service_test.go` untouched; remove it manually and run `goimports`/`gofmt` if needed.

Across a project-wide upgrade, Path A lands most services in a few edits. Reserve Path B for services that genuinely need a rebuild.

#### Step 3: Migrate Hand-Rolled Subscribe Calls Outside intermediate.go

Step 2 only rewrites the generated `Subscribe` calls in `intermediate.go` and the `Hook` bodies in `*api/client.go`. Any hand-rolled `Subscribe` calls in `service.go`, `OnStartup`, integration tests, or framework-internal code still use the legacy positional API and will not compile against the new connector.

**Detect them with a grep for the two-return call form** (the legacy API returned `(unsub, err)`; the new API returns just `error`):

```
grep -rn '_\?,\s*err\s*[:=]\+\s*[a-zA-Z_][a-zA-Z0-9_.]*\.Subscribe(' --include='*.go'
grep -rn '\.Subscribe("(GET\|POST\|PUT\|DELETE\|PATCH\|HEAD\|OPTIONS\|ANY)"' --include='*.go'
```

The first finds any call site that destructures two return values from `Subscribe`. The second catches the legacy positional form where the first argument is an HTTP method literal. Either pattern is a definite signal of the legacy API - the new API's first argument is a Go identifier name (e.g. `"MyHandler"`), and a subscription name that matches an HTTP method like `"GET"` is rejected at runtime.

**Conversion recipe.** For each match:

1. **Pick a unique name** that's a valid Go identifier starting with an uppercase letter (e.g. `MyPath`, `RootPage`, `HealthCheck`). The name must be unique within the connector - `Subscribe` returns an error on collision.
2. **Move the name to the first argument**, before the handler.
3. **Wrap the method and route in `sub.At(method, route)`**.
4. **Add `sub.Web()`**. Hand-rolled subs are almost always raw `func(http.ResponseWriter, *http.Request) error` web handlers; the new API requires a feature option, and `sub.Web()` is the right choice unless the handler is a typed marshaler (rare outside generated code).
5. **Drop the unsub return**. If the call site uses the closure (e.g. `defer unsub()`), replace it with `svc.Unsubscribe("Name")` against the same name chosen in step 1.

Carry over any existing `sub.RequiredClaims(...)`, `sub.NoQueue()`, `sub.Queue(...)`, `sub.Description(...)` options unchanged.

**Before:**
```go
unsub, err := svc.Subscribe("GET", ":443/my-path", svc.handleMyPath, sub.NoQueue())
if err != nil {
    return errors.Trace(err)
}
defer unsub()
```

**After:**
```go
err := svc.Subscribe("MyPath", svc.handleMyPath,
    sub.At("GET", ":443/my-path"),
    sub.Web(),
    sub.NoQueue(),
)
if err != nil {
    return errors.Trace(err)
}
defer svc.Unsubscribe("MyPath")
```

#### Step 4: Update LLM Tool Callers

If the project calls the LLM `Chat` endpoint with tools, callers must be updated to the URL-based contract:

- The tools argument is now `[]string`, where each string is the canonical URL of a Microbus endpoint to expose to the LLM (e.g. `calculatorapi.Arithmetic.URL()`).
- `llmapi.ToolsOf(endpoints ...*openapi.Endpoint)` is gone. The LLM service builds tools internally by fetching each host's OpenAPI document.
- Caller-constructed `[]llmapi.Tool` literals are no longer accepted by `Chat`. (The `llmapi.Tool` type still exists, but it's an internal resolved-tool shape produced by the LLM service from the OpenAPI fetch.)

Search the project for these patterns and update accordingly:

- `llmapi.ToolsOf(downstreamapi.Foo, downstreamapi.Bar)` → `[]string{downstreamapi.Foo.URL(), downstreamapi.Bar.URL()}`
- Hand-built `[]llmapi.Tool{{...}}` → `[]string{...URL()}`
- `var tools []llmapi.Tool` → `var tools []string`
- The LLM service signature `Chat(ctx, messages, []llmapi.Tool)` → `Chat(ctx, messages, []string)` at every call site

#### Step 5: Rename openaillm References to chatgptllm

Skip this step if the project never used `openaillm`. Otherwise, update:

- Import path `github.com/microbus-io/fabric/coreservices/openaillm` → `github.com/microbus-io/fabric/coreservices/chatgptllm`
- Inner API import `.../openaillm/openaillmapi` → `.../chatgptllm/chatgptllmapi`
- Package identifier `openaillm` → `chatgptllm` and `openaillmapi` → `chatgptllmapi`
- Hostname literal `"openai.llm.core"` → `"chatgpt.llm.core"` (e.g. in config keys, `ForHost` calls, manifest `downstream` entries)
- Service registrations `openaillm.NewService()` → `chatgptllm.NewService()`

If the project's `llm.core` config sets `ProviderHostname: openai.llm.core`, update it to `chatgpt.llm.core`. The wire API the service speaks (OpenAI's Chat Completions API) is unchanged - only the Microbus identifiers move.

#### Step 6: Update Manifests

Update the `frameworkVersion` in all `manifest.yaml` files in the project to the current target version.

#### Step 7: Apply Wave 3 Fixups

These changes affect any code that talks to the connector's built-in OpenAPI handler or to the OpenAPI portal. Skip the bullets that don't match anything in the project.

**a. `controlapi.OpenAPI` callers**: the route moved to `:888/openapi.json` and the typed client should target the host directly. Search for the legacy override pattern:

```
grep -rn 'controlapi\.NewClient([^)]*)\.WithOptions(pub\.URL' --include='*.go'
```

Convert each match to `ForHost`:

**Before:**
```go
doc, _, err := controlapi.NewClient(svc).
    WithOptions(pub.URL("https://" + host + ":" + port + "/openapi.json")).
    OpenAPI(ctx)
```

**After:**
```go
doc, _, err := controlapi.NewClient(svc).ForHost(host).OpenAPI(ctx)
```

**b. Hard-coded `:0/openapi.json` URLs**: any code constructing `https://<host>:<port>/openapi.json` directly needs the new path. Search and replace:

```
grep -rn ':0/openapi\.json\|:[0-9]*/openapi\.json' --include='*.go'
```

For internal callers, switch to `controlapi.NewClient(svc).ForHost(host).OpenAPI(ctx)`. For external callers (e.g. test scripts hitting the ingress), route them through the OpenAPI portal at `/openapi.json` (aggregate) or `/openapi.json?hostname=X` (single host) instead.

**c. `openapiportalapi.Client.List` callers**: the HTML `List` endpoint is gone. Search:

```
grep -rn 'openapiportalapi\.[A-Za-z]*Client([^)]*)\.List(' --include='*.go'
```

Replace with `Document(...)` for JSON or `Explorer(...)` for HTML. Note the signature changed (no `method` arg on the new methods).

**d. `components.schemas` consumers**: any code reading the OpenAPI doc's `components.schemas` by hard-coded key (e.g. `Foo_OUT`) must now use the hostname-prefixed form (`myservice_example__Foo_OUT`). There's no mechanical rewrite - review each call site.

**e. `requiredClaims` expressions using the legacy `microbus://` issuer scheme**: the access-token and bearer-token services now mint `iss=https://...` (the `microbus://` prefix is accepted by the verifier for backward compatibility, but new tokens carry `https://`). Tokens themselves keep verifying, but `requiredClaims` predicates that pin `iss` via *equality* against the old scheme fail closed (403) on every authenticated call. Search:

```
grep -rn 'microbus://' --include='manifest.yaml' --include='*.go'
```

For each match that lives inside a `requiredClaims` field (in `manifest.yaml`) or a `sub.RequiredClaims(...)` argument (in `intermediate.go` or any hand-rolled `Subscribe` calls):

- **Equality form** (`iss=="microbus://access.token.core"`): rewrite to `iss=="https://access.token.core"`, or — preferred — migrate to the scheme-agnostic regex form `iss=~"access.token.core"` recommended by `.claude/rules/microbus.md`.
- **Scheme-pinning regex** (`iss=~"^microbus://"`): rewrite to `iss=~"^https://"`, or drop the scheme and match just the host: `iss=~"access.token.core"`.

Matches outside `requiredClaims` (e.g. comments, fixture tokens that hard-code an `iss` value) usually don't need a change — the verifier still accepts the legacy scheme. Update only if those values feed into a `requiredClaims` evaluation.

**f. Test fixtures with empty unsigned actor against claim-gated endpoints**: the connector now evaluates claims for unsigned tokens in TESTING. Any test that set a no-claim actor (e.g. `pub.Actor(jwt.MapClaims{})`) against a `RequiredClaims`-gated endpoint and expected success now gets 403. Mint matching claims (e.g. `pub.Actor(jwt.MapClaims{"roles": map[string]any{"admin": true}})`) or relax the gate.

**g. Optional: add `mcpportal`**: if the project wants MCP integration for LLM clients, add `mcpportal.NewService()` to `main/main.go` alongside `openapiportal.NewService()`.

#### Step 8: Vet and Test

Run `go vet ./...` and `go test ./...` on the project. Fix any compilation or test failures before finishing.

Common stragglers to grep for (post-upgrade, none of these should appear in production code):

- `func Test.*_OpenAPI(` - old per-service OpenAPI tests; should be removed
- `svc.Subscribe\(.*openapi` - old OpenAPI subscription line in `intermediate.go`
- `_\?,\s*err\s*[:=]\+\s*[a-zA-Z_][a-zA-Z0-9_.]*\.Subscribe(` - legacy two-return `Subscribe` form; the new API returns `error` only and unsub is by name (see Step 3)
- `\.Subscribe("(GET\|POST\|PUT\|DELETE\|PATCH\|HEAD\|OPTIONS\|ANY)"` - legacy positional form with HTTP method as the first argument (see Step 3)
- `func \(svc \*Intermediate\) doOpenAPI\(` - old per-service OpenAPI handler
- `openapi/doc"` - old `Render` import path; the renderer is now in `openapi` itself
- `openapi\.Doc\b` - renamed to `openapi.Document`
- `llmapi\.ToolsOf\b` - replaced by `[]string{...URL()}`
- `\[\]llmapi\.Tool\b` outside the `llmapi` package or LLM service internals - callers should use `[]string`
- `openaillm` / `openaillmapi` / `openai.llm.core` - should be `chatgptllm` / `chatgptllmapi` / `chatgpt.llm.core`
- `regexp.MustCompile.*reqPort` in `doOpenAPI` - old port-filter pattern, should not exist (whole method is gone)
- `WithOptions(pub.URL([^)]*openapi\.json` - Wave 3: legacy controlapi URL override; should be `.ForHost(host).OpenAPI(ctx)`
- `:0/openapi\.json` - Wave 3: old OpenAPI route; the connector now serves `:888/openapi.json`
- `openapiportalapi\.[A-Za-z]*Client([^)]*)\.List(` - Wave 3: removed; use `Document(...)` or `Explorer(...)`
- `iss[=~!]+["']?microbus://` - Wave 3: legacy issuer scheme inside `requiredClaims`; rewrite to `https://` or use the scheme-agnostic `iss=~"access.token.core"` form (see Step 7e)

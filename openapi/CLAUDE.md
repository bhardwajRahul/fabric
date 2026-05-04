## Design Rationale

### Two-level schema name scoping for aggregator-safe component keys

Schemas live under `Components.Schemas` keyed by:

- **Service prefix** — `ServiceName` with dots converted to underscores (e.g. `my.service` → `my_service`).
- **Endpoint key** — `<servicePrefix>__<EndpointName>` with a *double* underscore separating the two.
- **Final key** — `<endpointKey>_<role>` where role is `IN`, `OUT`, or a referenced type name.

So `my.service`'s endpoint `Foo` whose input references type `Bar` produces component keys `my_service__Foo_IN` and `my_service__Foo_Bar`.

A doc rendered by an individual microservice stands on its own; the per-service prefix exists for the OpenAPI portal, which aggregates docs from many services into one. Without prefixing, two services that both define a `Bar` type would collide on the component key. The double underscore between hostname and endpoint name is purely visual — it makes the host-vs-endpoint boundary obvious when scanning component keys.

### The error schema is *not* scoped, deliberately

Every Microbus service produces the same `errors.StreamedError` shape under the same component key (`error_ErrorResponse`). The renderer skips the `scopePrefix` for this one schema specifically.

Errors are framework-level: every endpoint of every service emits the same shape. Since collision is impossible — there's only one shape — the schema is keyed at the framework level so that when N services' docs are merged at the OpenAPI portal, the N identical error schemas collapse into a single entry. The portal is the primary beneficiary of this mixed approach (per-service prefixing for endpoints, framework-global keying for the error type).

If you change the error schema, you change it everywhere.

### Greedy path arguments lose the `...` in the rendered path

Microbus routes use `{name...}` for greedy capture (the parameter swallows the rest of the path), but OpenAPI 3.1's path templating syntax has no greedy form — only `{name}`. The renderer strips `...` from both the rendered path and the parameter name. Substitution is best-effort: many OpenAPI clients accept slashes in `{name}` values and route correctly, but the spec doesn't guarantee it. An OpenAPI consumer cannot tell from the document alone that a path argument is greedy. If that distinction matters to a downstream tool, it has to read the original Microbus route, not the rendered OpenAPI path.

### `$id` is stripped from reflected schemas

The `invopop/jsonschema` reflector sets `$id` from the Go package path on every reflected schema. OpenAPI validators reject a schema entry that has both `$id` and `$ref`, which is the wrapper pattern the renderer uses everywhere. `resolveRefs` zeroes out `$id` before the schema lands in `Components.Schemas`. If you upgrade `invopop/jsonschema` and start seeing validation failures, this is the first place to check.

### `$ref` rewriting from `#/$defs/` to `#/components/schemas/<endpoint>_`

`invopop/jsonschema` emits `$defs` for nested type definitions and references them as `#/$defs/TypeName`. OpenAPI organizes shared schemas under `#/components/schemas/` instead. `resolveRefs` walks every schema and rewrites these references in place, simultaneously promoting each `$defs` entry to a `Components.Schemas` entry under the endpoint-scoped key. After this pass, the `Definitions` field is nilled out so the rendered schema doesn't carry stale `$defs`.

### `HTTPRequestBody` / `HTTPResponseBody` shape the schema, not just the wire

The same magic field names that `httpx` recognizes for runtime body routing are also recognized here, but they shape *schema generation*:

- **`HTTPRequestBody`** on the input type causes the request body schema to be reflected from *that field's type alone*. Other fields of the input struct become query or path parameters. The body magic is gated on `methodHasBody(method)` — for `GET`/`DELETE`/etc. the field is silently ignored and no `requestBody` is emitted, but the "other fields become query params" effect still applies (because all input fields are query/path params for body-less methods anyway).
- **`HTTPResponseBody`** on the output type causes the response body schema to be reflected from that field's type alone, preempting all other return values.

Renaming these fields silently changes the schema shape. Same caveat as in `httpx/CLAUDE.md`.

### `deepObject` is query-only per OpenAPI 3.1

OpenAPI 3.1 forbids the `deepObject` style (and `explode: true`) on path parameters — only on query parameters. The renderer sets these only when `parameter.In == "query"`; path params use the default `simple` style. Setting them on path params would produce a doc that fails validation in tools like Spectral or Stoplight.

### Query parameters are not marked `Required`

Query parameters generated from typed function inputs are not flagged `required` even when the underlying Go field is non-pointer/non-omitempty. This is intentional: the framework wants endpoints to accept partial input and apply Go zero-value defaults, with runtime validation in the handler catching wrong values. Marking them required in the OpenAPI doc would cause spec-strict clients to reject valid omissions before the request ever leaves the wire. Path parameters, by contrast, are always `Required: true` because the path itself can't render without them.

### `x-feature-type` and `x-name` are consumed by tooling

Each rendered `Operation` carries two non-standard extensions:

- **`x-feature-type`** — `function`, `workflow`, or `web`. Tasks and outbound events are filtered out *before* reaching the renderer (in `connector/control.go`), so they never appear in the doc.
- **`x-name`** — the endpoint's original Go-side name (PascalCase, pre-kebab-conversion).

Three internal consumers read these fields today:

- `llm.core` reads `x-feature-type` to decide whether an operation is a regular tool call (function/web) or a dynamic subgraph dispatch (workflow), and uses `x-name` for tool naming.
- `mcpportal` uses `x-name` to match MCP tool calls back to operations.
- `openapiportal` carries `x-name` and `x-feature-type` through to its aggregated output.

Nothing prevents external callers from depending on these extensions if `mcpportal` or `openapiportal` exposes the doc to them. Treat them as load-bearing.

### Server URL is derived from `RemoteURI` if present

If `Service.RemoteURI` is set, the renderer searches it for `/<serviceName>/` or `/<serviceName>:` and takes everything *before* that match as the `servers[0].url`. This is what makes a rendered doc point at the actual external ingress host (e.g. `https://my.example.com/`) rather than `localhost`. The default is `https://localhost/` when no `RemoteURI` is set.

If `RemoteURI` doesn't contain the service name, the localhost default sticks. The connector's OpenAPI handler pulls `RemoteURI` from the request's `X-Forwarded-*` headers, so a doc fetched without those headers always reads as `localhost`.

### Method defaults differ per feature type

Functions and workflows default to `POST` when method is empty or `ANY`. Web endpoints default to `GET`. The default is applied both to the rendered operation's method *and* to the path-key entry, so the doc has a single `post` entry rather than `any`. If a consumer needs to know the operation accepts any method, they need to rely on the framework's external-but-not-OpenAPI behavior (which the OpenAPI doc cannot express).

### `OutboundEvent` has a constant but never renders

`FeatureOutboundEvent = "outboundevent"` exists in `feature.go` so the value can be set on a `sub.Type` and round-tripped through the framework, but the renderer's switch only handles `function`, `workflow`, and `web`. Outbound events are filtered upstream by `connector/control.go`'s `handleOpenAPI` before reaching `Render`, so this branch is effectively dead in normal use — the constant is there for completeness and external introspection.

### `Doc` is a transitional alias for `Document`

Pre-v1.27 code used `openapi.Doc`. The type was renamed to `Document` to match OpenAPI's own terminology; `type Doc = Document` is a transitional alias for source compatibility and will be removed in a future release. New code should use `Document`.

### JSON output is byte-stable across renders

Downstream cache mechanisms (notably Anthropic's prompt cache via `claudellm`) depend on the rendered JSON being byte-identical for the same input across multiple calls. Two facts make this hold today:

1. **`Document` is reflected from Go structs.** `encoding/json` marshals struct fields in declaration order, deterministically.
2. **Maps inside `Document`** (`Paths`, `Components.Schemas`, etc.) are marshaled with alphabetically-sorted keys by Go's `encoding/json` (since Go 1.12). So even though Go map iteration order is randomized, JSON output isn't.
3. **`invopop/jsonschema`** emits schema definitions whose property maps marshal alphabetically too, for the same reason.

If you change anything in this package that introduces a non-deterministic ordering — for example, replacing a map with a custom slice traversed in an order derived from `range` over another map without sorting — you can silently destroy cache hit rates in `claudellm` (and any future consumer that depends on byte-stable schemas). Tests would still pass; the failure would only surface as a worsening cache hit ratio in production.

### Godoc `Input:` / `Output:` sections become per-field descriptions

`parseParamDescriptions` scans the endpoint's `Description` for `Input:` and `Output:` sections, extracts bulleted lines of the form `- name: description`, and applies them to matching schema properties. Field name matching is by JSON tag (or struct field name as fallback). Existing descriptions are not overwritten — the godoc only fills *empty* property descriptions. So `jsonschema:"description=..."` tags on a struct take precedence over the godoc section.

A non-list, non-blank line ends the current section. Blank lines within a section are tolerated so godoc-style spacing works.

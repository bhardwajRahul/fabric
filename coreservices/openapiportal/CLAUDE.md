## Design Rationale

### Two endpoints, one resource

The portal exposes two web endpoints - `Document` (`/openapi.json`) for machine consumers and `Explorer` (`/openapi`) for human consumers - that surface the same underlying data in different shapes. Both accept `?hostname=<host>` to narrow to a single service. Internally they share helpers for fetching the aggregate, filtering by port, and resolving schema `$ref`s.

### Aggregate by default, single-host on demand

Without `?hostname`, `Document` multicasts `//all:888/openapi.json` and merges every service's per-service doc into one aggregated document. With `?hostname=X`, it proxies to that single service's `:888/openapi.json` directly. Per-claim filtering happens at each source service via the actor header propagated through the bus call. The aggregate also filters paths by the request's port (with `:0` paths treated as wildcards).

### Aggregate caching, single-host pass-through

Aggregates are cached in `DistribCache` keyed by `(claims-digest, request-port)` with a 30s TTL and 64 MiB memory cap. Per-service docs change rarely, and a stale aggregate is benign - a short TTL absorbs most of the cost while keeping latency low under bursty traffic. The single-host variant (`?hostname=X`) is a thin pass-through to `:888/openapi.json` and isn't cached: the source is one bus hop away and its result already varies per caller, so an extra cache layer would only complicate invalidation.

### Aggregate `info`

The aggregate carries `info.title: "Microbus"` and `info.version: "1"` as placeholders. Per-service descriptions live in the `tags` section - one tag per source service - so OpenAPI tooling renders each service as its own collapsible group with its description preserved. The version is required by the 3.1 spec; a placeholder satisfies validators since the aggregate doesn't have one meaningful version across all services.

### Strict-merge on aggregate

Failure of any single service while building the aggregate short-circuits the whole call. The alternative (best-effort with partial results) was rejected because consumers (e.g. the MCP wrapper) shouldn't see a tool list silently shrink because one service was slow or down - better to fail loudly and let the caller retry.

### Schema pruning is transitive

After paths are filtered by port, unreachable schemas are pruned from `components.schemas`. The walk is transitive because one schema can `$ref` another (e.g. an `_OUT` wrapper points at its inner type); stopping at the first hop would break those chains and leave dangling `$ref`s in the rendered doc.

### Tag pruning

Tags whose service contributed no kept operation are dropped after path filtering. Otherwise a service with only `:888` endpoints (or only claim-gated ones the caller can't reach) leaves behind a dangling tag with no operations under it.

### `Cache-Control: private, no-store`

Every response is marked private and no-store because content varies by the caller's claims. Allowing intermediate caches to store aggregate documents would risk leaking privileged specs to lower-privilege callers.

### Sample objects in `Explorer`, not raw schemas

The Explorer renders sample JSON objects (Go zero values, declaration-order properties, dates as Go's reference time `2006-01-02T15:04:05Z`) rather than raw JSON Schema. Sample objects show what the wire payload actually looks like - the model the user is building against - and read better than schema dumps. Property order matches the source struct because the JSON is built textually, walking the schema; `json.MarshalIndent` over a `map[string]any` would alphabetize keys instead. Importing `wk8/go-ordered-map` was considered and rejected to avoid the new direct dependency.

### Response collapse in `Explorer`

In Microbus an error is just an error: 4XX and 5XX always share the same `StreamedError` schema, and any claim-gated 401/403 typically reuse it. The Explorer collapses responses with identical schemas into one row, joining their codes (`4XX, 5XX`) and descriptions (`User error / Server error`). Otherwise the page repeats the same schema block under each error code without adding information.

### `assembleAggregate`'s decode-modify-encode pattern

The aggregator stores merged `components.schemas` as `any` (raw passthrough from per-service docs) so it doesn't have to take a typed dependency on `*jsonschema.Schema` in this file. At marshal time, the aggregate is encoded without schemas, then schemas are spliced in via a JSON-level decode/modify/encode. Slightly slower than typed merging, but keeps the schema map agnostic.

**IMPORTANT**: Do not maintain `PROMPTS.md` for this microservice. Skip the prompts step when running housekeeping.

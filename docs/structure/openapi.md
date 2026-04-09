# Package `openapi`

The `openapi` package models OpenAPI 3.1 documents and renders them to JSON. It is consumed by the connector's built-in `:888/openapi.json` handler, which walks the connector's subscription map and produces an OpenAPI document on the fly - there is no per-service `doOpenAPI` handler. The handler is registered alongside the other control endpoints on `:888` (with a parallel `//all:888/openapi.json` mirror for broadcast aggregation), filters by the caller's claims, and returns endpoints across every port; consumers (e.g. the OpenAPI portal) apply any port-based filtering at their ingress boundary.

Schema component keys (and `$ref` pointers) are prefixed with the source service's hostname (dots → underscores) so multiple per-service docs aggregate cleanly without component-key collisions. The framework's standard error schema is intentionally *not* prefixed: every service emits an identical entry, and the aggregator deduplicates it via map merge.

The package supports document generation by:

- Modeling the OpenAPI document with Go structs that marshal to JSON.
- Translating Go primitives to the corresponding OpenAPI types.
- Traversing the dependency tree of complex types (structs) using reflection and translating them into the corresponding OpenAPI components, honoring `jsonschema` description tags on each field.

The package exposes:

- `Document`, `Service`, `Endpoint`, `Operation`, `Parameter`, etc. - the structural types a caller (typically the connector) populates.
- `Render(svc *Service) *Document` - converts a populated `Service` into a fully-resolved `Document` ready for `json.Marshal`.
- `Feature*` string constants (`FeatureFunction`, `FeatureWeb`, `FeatureWorkflow`, `FeatureInboundEvent`, `FeatureTask`, `FeatureOutboundEvent`) used to classify each endpoint. Only `FeatureFunction`, `FeatureWeb`, and `FeatureWorkflow` are emitted into the rendered document; tasks, inbound events, and outbound events are silently filtered out.

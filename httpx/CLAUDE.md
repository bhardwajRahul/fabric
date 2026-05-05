## Design Rationale

### `ValidateHostname` is identity-strict, single-purpose, and not lenient

`httpx.ValidateHostname` is the single source of truth for what a canonical Microbus identity hostname looks like: lowercase letters, digits, dots, and hyphens only (`^[a-z0-9-]+(\.[a-z0-9-]+)*$`), length ≤ 252, no leading or trailing dot, no consecutive dots, no `id-` or `loc-` first segment, not equal to `all`, no `.all` suffix. Callers are responsible for normalization - the validator does not trim or lowercase, and errors on non-canonical input.

The strict rules exist to keep the wire format unambiguous:

- **No underscore.** `_` is reserved as the flat-form encoding of `.` inside NATS subject segments (see `connector/CLAUDE.md`'s "Subject encoding" section). Allowing `_` in identity hostnames would silently collide on round-trip - `my_service.core` and `my.service.core` both flatten to `my_service_core`, and the un-flatten side picks the wrong one.
- **No `id-` or `loc-` first segment.** Those prefixes are reserved for the per-instance and locality slots in the NATS subject layout. A hostname starting with either would collide with the publisher's slot extraction.
- **No `all` or `*.all`.** `all` is the broadcast hostname used by the control plane; `<id>.all` is how broadcast addressing routes to a specific instance.
- **Lowercase only.** Keeps the canonical identity form unambiguous. The framework already lowercases hostnames in subject construction, so accepting mixed case at registration time would just delay the normalization to a less-obvious place.

`ValidateHostname` is intentionally narrow. It does not have variants for "loose" or "route" hostnames, and `httpx` exposes no other hostname helpers - the package owns the *what* of a valid Microbus identity and nothing more. Higher-level packages (notably `sub` for subscription route hostnames) sometimes need to accept a slightly looser shape; they handle that with their own preprocessing on top of this validator. See `sub/CLAUDE.md` for how subscription routes wrap a translation step around the strict check.

The HTTP ingress proxy, by contrast, performs no hostname validation at all - it lowercases the host segment of an incoming URL and lets `Publish` fail naturally when an unrecognized hostname produces no responders.

### Three magic field names drive reflection-based payload routing

`HTTPRequestBody`, `HTTPResponseBody`, and `HTTPStatusCode` are recognized by `ReadInputPayload` / `WriteInputPayload` / `ReadOutputPayload` / `WriteOutputPayload` via `reflect.FieldByName` lookup. They are not types or interfaces - they are well-known *string* names that the reflection layer matches against:

- **`HTTPRequestBody`** on an input struct reroutes body parsing into that field instead of the parent struct, and on the outbound side reroutes only that field into the body (other fields go into the query string).
- **`HTTPResponseBody`** on an output struct reroutes response-body decoding into that field instead of the parent.
- **`HTTPStatusCode`** on an output struct receives the response status code (read side) or sets the response status code (write side) via `Int` semantics, so the field type must be `int`.

The string-name convention exists because Go does not allow struct tags on *function arguments*, and the framework's typed-endpoint signatures are function arguments - there is no syntactic surface to attach a `microbus:"body"` tag to. The magic names are the only available channel for expressing this contract in pure Go signatures. Renaming any of these fields silently disables the magic; there is no compile-time check.

### Input payload precedence: path < body < query

`ReadInputPayload` populates the input struct in this order:

1. Path arguments via `DecodeDeepObject` (so path args set the initial values).
2. Body via `ParseRequestBody` (JSON or form, overwriting path values where they collide).
3. Query string via `DecodeDeepObject` (overwriting both).

Last write wins. So a query-string argument can override a body field, which can override a path arg. This is deliberate but easy to forget when debugging "why did the query value take effect?"

### `BodyReader` is the framework's reusable-buffered body

`BodyReader` wraps a `[]byte` as both an `io.Reader` and `io.Closer`, plus exposes `Bytes()` and `Reset()`. It exists because the framework needs to:

1. **Read the body more than once.** The connector's request handler may parse headers, dispatch, and re-parse the body during defragmentation. `Reset()` rewinds without reallocating.
2. **Hand the underlying bytes to optimization paths.** `Copy` and `NewFragRequest` both type-assert `Body.(*BodyReader)` and operate on the raw byte slice when possible.

`Close()` is a no-op - there's nothing to close, but the type satisfies `io.ReadCloser` for `http.Request.Body` / `http.Response.Body`. The connector substitutes `BodyReader` into incoming requests when it has a buffered body to give them; user handlers can therefore depend on `Reset()` working on inbound requests but should not depend on it on arbitrary `io.ReadCloser` bodies.

### `Copy` transfers byte ownership when possible

When both the source `*http.Response.Body` is a `BodyReader` and the destination `http.ResponseWriter` is a `*ResponseRecorder` (and the recorder is empty), `Copy` constructs the recorder's buffer *directly over the BodyReader's bytes* - `bytes.NewBuffer(br.bytes)` - instead of copying. The comment in code calls this "somewhat risky: bytes are now owned by the buffer." After this transfer, mutating either side affects the other.

The motivation was reducing memory churn on the request hot path - which on a busy ingress proxy or service mesh is the biggest source of allocator pressure. If you call `Copy` and then mutate the source's body bytes downstream, expect surprises.

### `DecodeDeepObject`: bracket and dot notation are interchangeable

`DecodeDeepObject(values, target)` accepts both `a[b][c]=v` and `a.b.c=v` for the same key path. The first step normalizes brackets to dots (`strings.ReplaceAll("[", ".")` and drop `]`), so callers don't need to pick one syntax. This matches the OpenAPI deepObject style on the input side and the human-friendly dot style on the output side.

### Sequential integer keys decode as arrays, not objects

If a decoded sub-tree has keys exactly equal to `"0"`, `"1"`, ..., `"len-1"` (no gaps, contiguous from 0), it is converted to a slice rather than a map. So `x[0]=a&x[1]=b&x[2]=c` becomes `{"x": ["a","b","c"]}`, not `{"x": {"0":"a","1":"b","2":"c"}}`.

A gap or out-of-range key disables the conversion for that sub-tree - `x[1]=b` alone produces a map, since "0" is missing. Nested sub-trees are evaluated independently.

### Two-pass type fallback in `DecodeDeepObject`

`detectValue` infers a type per query value: `null` → nil, `true`/`false` → bool, JSON-number-looking strings → `json.Number`, else string. The decoded tree is JSON-marshalled, then unmarshalled into the user's struct.

If unmarshal fails with `*json.UnmarshalTypeError` (e.g., decoded a number but the target field is a string), the decoder retries with *all* leaf values forced to strings (`leafsToStrings`). This means a target field that's typed as `string` will receive `"42"` even if the URL had `?x=42`. The cost is one wasted marshal/unmarshal round-trip on the mismatch path; the benefit is that user types don't need to match query-string literal types.

`json.Number` is used (rather than `float64`) per the in-code comment: *"to preserve precision through marshal/unmarshal."* Avoids the IEEE-754 round-trip that would clobber large integers.

### `SetPathValues` / `PathValues` round-trip uses Go 1.22 `r.PathValue`

`SetPathValues(r, routePath)` parses the request's actual path against the parameterized route (`/obj/{id}/{}`) and stores values via `r.SetPathValue(name, value)`. `PathValues(r, routePath)` reads them back into `url.Values` for downstream merging into the input struct. The framework relies on Go 1.22's first-class path values for storage rather than maintaining a side map.

Unnamed path arguments - bare `{}` - are auto-named `path1`, `path2`, ... in left-to-right declaration order. Greedy arguments `{name...}` capture the entire path tail.

### `FillPathArguments` and `ResolveURL` unescape `{` and `}`

`url.Parse` percent-encodes braces (`{` → `%7B`, `}` → `%7D`), which is correct for arbitrary URLs but breaks Microbus's parameterized-route format. `ResolveURL` does a post-pass `ReplaceAll` to restore the literal braces. `FillPathArguments` lifts query-string values into matching `{name}` placeholders before publishing - so a client can pass path args as either part of the URL or as query args, and the framework normalizes.

### `ParseURL` rejects backticks

`ParseURL` rejects URLs containing the backtick character (`` ` ``) up front. The reason is that Go raw-string literals are delimited with backticks; allowing them inside URLs would mean a URL embedded in source code via a raw string could break the string boundary. Rejecting them is a small defense that lets Microbus URLs round-trip safely through Go raw strings without escaping concerns.

### `SetRequestBody` content-type sniffing for raw bytes/strings

When the body is `[]byte` or `string` and no `Content-Type` is set, `SetRequestBody` sniffs:

1. If the body starts with `{` and ends with `}` and `json.Unmarshal` to `map[string]any` succeeds → `application/json`.
2. If the body starts with `[` and ends with `]` and `json.Unmarshal` to `[]any` succeeds → `application/json`.
3. Otherwise `http.DetectContentType` (the standard library's MIME sniffer).

This is why a string `"{"foo":1}"` posted via the egress proxy automatically gets the right content type without the caller setting it. `url.Values` and `QArgs` always force `application/x-www-form-urlencoded`. Anything else is JSON-marshalled with `application/json`.

### `FragRequest` short-circuits when body is a `BodyReader` and fits

`NewFragRequest` has two paths: if the body is already a `BodyReader` and its bytes fit within `fragmentSize`, it sets `noFrags=true` and skips the fragment-array allocation entirely - `Fragment(1)` returns the original request unchanged. For other readers it consumes via `io.LimitReader` until EOF. This is the common-case fast path; non-`BodyReader` bodies always pay for the read-through.

### `DefragRequest` tolerates out-of-order fragment arrival

`DefragRequest.Add` stores fragments by index; arrival order doesn't matter. `Integrated()` walks 1..maxIndex and errors if any are missing. The integrated body is built with `io.MultiReader` over the per-fragment readers - no big buffer copy. The result is set onto fragment 1's request (so headers come from the first fragment), with `Content-Length` summed across fragments.

This is what allows the connector's NATS subscriber to call `Add` from its receive callback without sequencing.


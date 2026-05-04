## Design Rationale

### NATS connections are pooled by credential hash, one per executable

Multiple `Conn.Open` calls in the same process share one underlying `*nats.Conn` if their full configuration — URL, user/pw, token, JWT+seed, `ca.pem`, `cert.pem`+`key.pem`, `nats.creds` — hashes to the same key (sha256 of all fields concatenated with `|` separators). Reference counts on `refCountedNATSConn` track how many connectors are using it; the underlying connection closes only when the last reference goes away.

The motivation is production deployments where a Microbus application bundles several services into a single executable. The pooling keeps each shipped executable to **one NATS connection regardless of how many services it hosts**, rather than one per service. The same mechanism also benefits `application.RunInTest`, which fans many connectors out in one process during integration tests.

The hash is an exact-match key — different credentials, even if functionally equivalent, will not share.

### Two transports in one: short-circuit and NATS, registered together

When `shortCircuitEnabled` is true (default; toggle via `MICROBUS_SHORT_CIRCUIT=0`), `Subscribe` / `QueueSubscribe` register the handler on *both* the process-global short-circuit trie and (if connected) NATS. A single `*Subscription` wraps both registrations. This is what makes co-located service-to-service calls bypass NATS entirely while still being reachable from remote peers via NATS.

The short-circuit trie is a **process-global** singleton — `var shortCircuit trie` at package level. All connectors in one process share one trie. This is intentional: short-circuit only works between connectors in the same process, so a process-wide trie is the right granularity.

### Routing rules differ between unicast and multicast

The three send paths — `Request` (unicast), `Response` (unicast), `Publish` (multicast) — make different choices:

- **`Request` and `Response` (unicast)**: Try short-circuit first; if at least one matching handler is found locally, deliver and return. Otherwise fall back to NATS. This is the fast path that gives co-located services a 10× latency improvement over NATS.
- **`Publish` (multicast)**: Use NATS *only* when NATS is connected. Short-circuit is used only when NATS is not connected. The reason is correctness: short-circuit cannot reach remote peers, and a multicast must reach *all* subscribers (local and remote). Sending only locally would silently drop remote subscribers. The rule is universal — there is no opt-in to short-circuit-only multicasts when NATS is up.

### Single-handler short-circuit is zero-copy

When `deliverWithShortCircuit` finds exactly one matching handler, it passes the *live* `*http.Request` (or `*http.Response`) inside the `Msg` directly to the handler — no serialization. The handler can read the live object as-is.

With multiple handlers (e.g., a no-queue subscription with several subscribers), the message is serialized once into `Msg.Data` bytes, and each handler gets a separate `Msg{Data: ...}`. The handler receives serialized bytes in this case because mutating the live request mid-fan-out would corrupt other handlers' views.

The single-handler fast path is what gives the short-circuit its dramatic latency improvement. If you add subscriptions that always fan out (e.g., a non-queue subscriber on a hot path), you lose the zero-copy benefit.

### Trie wildcards mirror NATS, intentionally

The short-circuit's subscription matcher is a custom prefix trie that supports `*` (single segment) and `>` (greedy suffix) wildcards — the same syntax NATS uses. This is what allows short-circuit and NATS to handle the same subject patterns interchangeably. The trie matcher is custom-built rather than reused from a library because it also has to support queue-group round-robin and unsubscribe-with-trim, which most prefix-tree libraries don't.

`Handlers(subject)` walks segments left-to-right BFS-style, expanding into literal/`*`/`>` children at each step. Greedy `>` matches are collected separately and processed last. The traversal queue is reused via `sync.Pool` so high-volume publishers don't churn allocations.

### Queue group semantics: named = round-robin, unnamed = broadcast

Each trie leaf node has a `map[string]*ringList` keyed by queue name. The empty queue name `""` is the broadcast bucket. `appendNodeHandlers` selects:

- **One handler from each named queue** (round-robin via `ringList.Rotate()`).
- **All handlers from the unnamed queue** (every subscriber receives a copy).

So a fan-out subscription with no queue gets every message; named queues load-balance via simple ring rotation. NATS uses the same model — the local trie reproduces it so co-located behavior matches what would happen over the wire.

`Rotate()` is deterministic round-robin (advance head pointer), not random selection. Same as NATS — load balancing is fair across replicas under steady load, and surprises (e.g., locality skew) are predictable.

### Trie self-trims on unsubscribe

When a subscription is removed, `unsub` deletes its handler from its `ringList`. If the ring becomes empty, the queue entry is removed from the leaf node. If the leaf becomes empty (no queues, no children), it's removed from its parent. The trim recurses up the trie until a non-empty node or the root is reached. This keeps the trie minimal under heavy churn — long-lived subscribers don't accumulate stale internal nodes.

### `Latency()` drives the connector's ack timeout

The connector calls `Latency()` after `Open` and uses the result as its `ackTimeout`. The transport returns:

- **5 ms** for short-circuit-only deployments (no NATS).
- **NATS RTT × 2 + 50 ms, rounded up to the nearest 100 ms** for single-server NATS.
- **300 ms default** for multi-server NATS or RTT failure.

The 100 ms minimum is what was observed necessary on localhost under high load — anything below that over-fires on transient hiccups. The `× 2 + 50` shape lets the formula scale up for world-wide deployments where RTT itself is large. A single RTT measurement at startup is a weak basis for predicting steady-state ack latency, so this formula is best-effort and may need adjustment as deployments evolve. Multi-server NATS just gets the conservative 300 ms default because RTT to the connected server doesn't predict cluster-wide latency.

### `WaitForSub` is a 20 ms sleep, observed-required

NATS subscription registration is asynchronous — `Subscribe` returns before the server has acknowledged the subscription. Until the server acks, messages published to the subject won't reach this subscriber. The 20 ms sleep is empirically the minimum that was observed to make NATS reliably recognize a new subscription before traffic flows; without it, race-window-dependent test flakes appeared. There is no synchronous wait primitive in the NATS Go client for this.

It's a no-op when NATS isn't connected (short-circuit subscriptions are synchronous). The connector calls it after activating subs in batches.

### `SetPendingLimits(-1, -1)` is "no arbitrary limit"

NATS subscriptions default to bounded pending-message and pending-byte limits — when exceeded, NATS *drops* messages and the subscription stays alive. The framework calls `SetPendingLimits(-1, -1)` to disable both limits.

The reasoning is not "memory pressure is fine," it's "any cap we pick would be arbitrary." Microbus consumes from the bus quickly enough that hitting a finite limit in normal operation would imply something is already wrong — and at that point silently dropping messages is worse than the symptoms of unbounded queues. If you observe memory growth on a slow handler, the right response is to fix the handler, not to re-enable the cap.

### `mem.Alloc` / `mem.Free` for publish buffers

`Publish` / `Request` / `Response` allocate a 2KB-minimum buffer via `mem.Alloc` to serialize the HTTP message before handing it to NATS, then `mem.Free` it. This is a per-process buffer pool to reduce GC pressure on the publish hot path. The buffer is sized as `1KB + ContentLength`, so most small requests fit in the minimum 2KB block and small-payload publishes don't trigger any allocation at all.

### `Msg` carries one of three forms

`Msg` has three nullable fields — `Data`, `Request`, `Response`. Exactly one is populated:

- **`Request` / `Response`**: live HTTP object, used by the single-handler short-circuit zero-copy path.
- **`Data`**: serialized bytes, used by NATS receive, multi-handler short-circuit fan-out, and the connector's defragmenter.

Handlers that only handle one form will need to call `http.ReadRequest` / `http.ReadResponse` themselves on `Data` if Request/Response is nil. The connector's `onRequest` does this — it lazily parses `Data` when needed and caches the result on the `Msg`.

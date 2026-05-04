## Design Rationale

### Each key lives on exactly one replica by default

`Store` deletes the key at every peer (via `?do=delete` broadcast) and writes only into the local LRU. `Load` first checks local; on miss, it broadcasts `?do=load` and accepts the first peer that has it. The cache is therefore **segmented across replicas** rather than replicated — each key is owned by whichever replica most recently wrote it.

The primary rationale is capacity scaling: total cache size grows linearly with replica count rather than being capped at one replica's memory. The broadcast-on-write is still required to invalidate stale copies that peers may have loaded earlier, even though the value lives on only one replica afterward.

`Replicate(true)` flips this to a write-to-all model: every `Store` broadcasts `?do=store`, every replica keeps a copy. Trades capacity (N× memory) for `Load` latency (always local). Default is non-replicated.

### `ConsistencyCheck` is best-effort detection, not a guarantee

The distributed cache is **not guaranteed consistent**. Concurrent `Store`s on different replicas, network partitions during a broadcast-delete, and ack-timeout races on slow peers can all leave divergent state. The framework does not attempt to prevent these — preventing them would require coordination protocols that the cache deliberately avoids.

Instead, `ConsistencyCheck` (default-on for `Load`) is opportunistic detection: when a local hit happens, the cache sha256s the value, broadcasts `?do=checksum`, and if any peer reports a different hash, both sides delete the key and the load is downgraded to a miss. The next caller will rebuild the value. This is self-healing in the sense that divergence is removed when observed — but it does not prevent divergence and may not detect it (the check itself can race).

Multi-peer `Load` (local miss, multiple peers respond) does the symmetric check: differing peer responses log an inconsistency and treat the load as a miss.

The check costs a multicast round-trip on every local hit. `ConsistencyCheck(false)` skips it. `Replicate(true)` + `ConsistencyCheck(false)` gives the lowest-latency reads at the cost of memory and weaker consistency.

### Single `?do=...` endpoint amortizes the known-responders cache

The `/all` subscription is a multicast (`sub.NoQueue`) reached by every peer. It dispatches via the `do` query parameter: `load`, `store`, `checksum`, `delete`, `clear`, `weight`, `len`. Each `do` value is a private mini-RPC.

The single-endpoint design is a startup-latency optimization. The connector's known-responders cache (see `connector/CLAUDE.md`) starts cold and the first multicast to a given subject incurs the full ack-timeout wait before responses are accepted. With one shared `/all` endpoint, the cache pays this cost *once*. With one endpoint per operation, every operation type would pay it independently — and microservices typically hit the cache during `OnStartup`, so that cost would extend startup time. Sharing one subscription means the responders-cache primes once and benefits all subsequent operations.

The `/rescue` subscription is separate (and unicast, `sub.DefaultQueue`) because it is used only during shutdown offload, not on the request hot path.

### Self-messages are filtered out by `FromID`

Every `/all` handler that mutates state checks `frame.Of(r).FromID() == c.svc.ID()` and either returns 404 or no-ops. The broadcast hits all replicas including the sender, but the sender already has the answer locally — letting it process its own message would either duplicate work or corrupt state. The `FromID` filter is the simplest way to suppress self-receive.

`handleAll` *also* rejects messages from a different hostname (`FromHost != svc.Hostname()`) — only same-microservice peers are allowed in, even though plane isolation already restricts this. Defense in depth.

### Brotli magic-word prefix lets compressed and uncompressed values coexist

When `Compress` is set on a `Store`, the value is prefixed with `0x91 0x19 0x62 0x66` (an arbitrary internal marker — there is no brotli standard for this) and then brotli-compressed. On `Load`, if the value starts with this prefix, the rest is decompressed; otherwise it's treated as uncompressed bytes.

This is what lets the same cache hold both compressed and uncompressed values without out-of-band type tracking. The four-byte cost is paid only on compressed values.

### Subscriptions are owned by the connector, not the cache

`start` registers `/all` and `/rescue` with `sub.Infra()` — the only current use of that flag. `Close` does *not* unsubscribe. The connector owns the subscription lifecycle and tears them down via its infra-deactivation pass during shutdown (see `connector/CLAUDE.md`). Code in this package therefore has no symmetric `stop` for `start`.

This asymmetry is deliberate: the connector knows the precise window in which the dlru's subscriptions must stay alive (through `OnStartup` *and* `OnShutdown`), and that window is enforced by the connector's lifecycle code, not by paired `start`/`stop` calls here.

### `Close` offloads to peers within a fixed 4-second budget

When the microservice shuts down, `Close` is called by the connector. It pings the cluster to count peers, then concurrently (`runtime.NumCPU()` workers) PUTs each local key to a random peer via `/rescue`. After 4 seconds of wall time, remaining keys are dropped.

The 4-second budget is sized to fit within the connector's 8-second pre-cancel drain window — the offload must complete before the connector's lifetime context is cancelled. Two enabling conditions:

1. **Local infra subs are already torn down.** The connector deactivates the dlru's `/all` and `/rescue` subs *before* calling `Close`. So outgoing rescue PUTs route only to peers — the sender's own queue group is gone.
2. **Bounded loss is acceptable.** The cache is opt-in lossy by design (the docs warn "cache only what you can afford to lose"). 4 seconds is long enough to evacuate most caches but short enough to keep total shutdown time bounded.

If zero peers are alive, `offload` returns immediately — there is no fallback to disk.

### Singleflight is per-process; cluster-wide stampedes still possible

`LoadOrCompute` and `GetOrCompute` use `golang.org/x/sync/singleflight` to deduplicate concurrent `maker` invocations *within a single process*. With N replicas, up to N makers may still fire concurrently for a cold key — the framework does not coordinate maker invocations across the cluster.

The `LoadOrCompute` flow uses a double-check idiom:

1. **Fast path.** Try `Load` without entering the singleflight group.
2. **Slow path on miss.** Enter the group, then re-check the cache. Another goroutine may have populated it between the fast-path miss and slot acquisition.
3. **Compute.** Call `maker`, store the result, return.

Maker errors are not cached — the next caller retries. This is intentional: a failed compute should not poison the cache for the rest of the TTL.

### `DeletePrefix` and `DeleteContains` walk all keys

The `?do=delete` handler accepts three mutually exclusive query args: `key` (exact), `prefix` (string prefix), `contains` (substring). `DeletePrefix` and `DeleteContains` broadcast the appropriate flavor and run the same predicate locally. The local LRU's `DeletePredicate` walks all keys — these operations are O(N) on each replica and should not be used as a general delete pattern. Reserve them for cache-invalidation events that genuinely span a key family.

### Per-cache subscription names enable multiple caches on one connector

`subscriptionName` derives a Go-style listen name from the cache's `basePath`: `:888/dcache` → `DcacheAll`, `DcacheRescue`. This keys each cache instance's subscriptions uniquely on the connector, so a microservice could in principle host multiple `dlru.Cache` instances at different paths. The canonical use is one cache per connector (the connector's own `:888/dcache`), but the path-derived naming leaves the door open.

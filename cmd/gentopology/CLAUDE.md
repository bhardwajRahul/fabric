# cmd/gentopology

Renders the application's topology diagram (a Mermaid graph) by walking
the bundle's source. For each service, detects what other services it
depends on, what events it hooks, whether it touches SQL, and whether
it makes HTTP egress calls + which external host(s).

## Why this tool exists

The topology of a Microbus app - what depends on what, what touches a
database, what calls out - is operator-facing documentation. It used to
be regenerated from per-service manifests by a skill that walked
`downstream:`, `db:`, and `cloud:` fields. Those fields were noisy in
the manifest (they shifted on every internal-call refactor) and
duplicative of what's in the source code anyway.

`gentopology` reads source directly. The manifest shrinks to pure
interface-contract (what each service exposes); the topology view
becomes a separate artifact with its own lifecycle.

## Pipeline position

```
source code  --(scan: deps + events + sql + cloud)-->  topology.mmd
```

No manifest dependency for route data; `manifest.yaml`'s `general.{name,
hostname}` is read for display labels only. Everything else is AST-derived.

## Detection signals

Per service:

- **Service deps** - every `*api`-suffixed import (other than own) that
  has at least one `<alias>.NewClient(...)`, `<alias>.NewMulticastClient(...)`,
  or `<alias>.NewMulticastTrigger(...)` call site. The dep target is
  resolved by reading the imported package's `endpoints.go` Hostname
  constant.
- **Event subscriptions** - every `<alias>.NewHook(svc).OnX(...)` chain
  in `intermediate.go`. Filtered to *api packages only. Same hostname
  resolution as deps.
- **SQL** - presence of `import "database/sql"` or
  `import "github.com/microbus-io/sequel"` (or anything under `sequel/`).
- **HTTP egress + external host** - presence of `httpegressapi` import
  triggers a URL-literal scan: every `https?://<host>` in non-test
  source is collected, hostnames matching bundle services are filtered
  out, and the result determines the cloud value:
  - exactly one external host → that hostname
  - zero or multiple → `"various"`

  The URL scan reads raw bytes (not the AST) to catch literals in
  composite-literal initializers, string concat, etc. False positives
  (a URL in a comment) widen the cloud to `"various"`, which is the
  safe default.

## Filter: only services in the bundle

The graph excludes services that aren't in the bundle, even if their
*api packages are imported. A bundled service that imports a non-
bundled `*api` is a wire-dead dependency at runtime; surfacing it
would mislead the operator.

The bundle is resolved from `--bundle main.go`'s `app.Add(...)` calls
(same shape as `cmd/gencreds` uses). Arguments whose leading selector
chain doesn't root in a known import alias (variadic spreads, range-loop
adds, factory expressions, var-bound services) are reported on stderr
with the offending file:line, then skipped. The diagram excludes them;
the operator notices the warning and decides whether to refactor the
add-call into a recognizable shape.

## Edge selection

A *api import that has BOTH client calls and hook calls produces both
a solid `--->` edge and a dotted `-..->` edge. A *api import that has
ONLY hook calls produces only the dotted edge - drawing a dep arrow on
top would be redundant since the event flow already conveys the
dependency.

## Output format

The mermaid follows the convention the framework was already using:

```
graph TB
    classDef core fill:#ed2e92,...
    classDef svc fill:#32a7c1,...
    classDef danger fill:#f15922,...
    classDef ext fill:#e5f4f3,...

    foo.example[Foo<br>foo.example] ---> bar.example[Bar<br>bar.example]
    foo.example --->|danger| trustroot.core[TrustRoot<br>trustroot.core]
    foo.example --- foo.example.db[(SQL)]
    bar.example --- bar.example.cloud@{shape: cloud, label: "api.example.com"}
    src.example -..-> sink.example

    class foo.example,bar.example,... svc
    class some.core,... core
    class trustroot.core,... danger
    class foo.example.db,bar.example.cloud,... ext
```

Services are ordered by reverse-hostname (`hello.example` →
`example.hello`) so domain-suffix groups appear adjacent. The first
appearance of each service emits the full label; subsequent edge
mentions use the bare hostname.

## Trust-root highlighting

A service that exposes any endpoint on port `:666` (the trust-root port — token mint, shell exec, privileged writes; see [`ROOT_TRUST.md`](../../ROOT_TRUST.md)) is rendered with `classDef danger` (orange `#f15922`) instead of `svc`/`core`, and every dep edge pointing at it carries a `|danger|` label. Event edges into a danger service are also labeled.

Detection is manifest-driven: `serviceFromDir` reads `webs`, `functions`, `tasks`, and `workflows` and flags the service if any route's port is `:666`. Outbound and inbound events are excluded — those are conventionally on `:417`, and the trust-root capability is about caller-reachable RPC/web/task/workflow endpoints, not event subscriptions.

The danger class overrides the `.core` suffix rule, so trust-root core services (e.g. `access.token.core`, `bearer.token.core`, `shell.core`) appear orange rather than pink. This is intentional — a reader scanning the diagram for elevated-privilege services should see them at a glance.

## `--check` mode

`--check` exits non-zero (code 2, `errCheckDiff` sentinel) if the
existing `topology.mmd` would change under regen. CI can invoke
gentopology with `--check` to flag unrendered drift.

## Things that would surprise future-me

- **The display name falls back to the lowercase directory base name**
  when `manifest.yaml` lacks `general.name`. This is fine for services
  whose hostname implies the name (`foreman.core` → label "foreman")
  but loses operator intent for services like "HTTPIngress" that need
  case preservation. Operators add `name:` to their manifest to fix.
- **URL-literal scan reads raw bytes, not AST.** It catches more
  patterns (composite literals, concat) but also matches URLs in
  comments. False positives widen cloud to "various", which is safe.
- **The bundle filter changes meaning over time.** A service removed
  from `main/main.go` disappears from the topology even if other
  bundled services still import its *api package. That's intentional -
  the topology shows runtime relationships, not compile-time ones.
- **`go list` requires CWD = main.go's directory** for `--bundle` mode.
  The tool sets it via `cmd.Dir = workDir` so callers don't have to.
- **`go list` failures abort with a real error.** With the `-e` flag set,
  `go list` itself succeeds for missing packages (returned `Dir` is
  empty), so an error from `goListDir` represents a genuine toolchain
  failure (broken module, missing `go`, malformed `go.mod`) rather than
  a missing dependency. Earlier versions of the tool swallowed these as
  silent skips, which masked a class of operator-side environment bugs.
- **Hook-only deps produce only the dotted edge.** If a future use
  case wants both edges, the renderer needs a flag - today the dotted
  edge is treated as superseding the solid one.

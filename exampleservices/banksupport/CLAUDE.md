# banksupport.example

**CRITICAL**: This directory is a Microbus microservice. Before performing any task, check for pertinent skills in `.claude/skills/` and its subdirectories. Follow the workflow of the most relevant skill.

## Agent Instructions

This microservice reads actor claims and mints tokens. See `.claude/rules/auth.txt` for the conventions.

## Design Rationale

### Purpose

BankSupport is the suite's authentication + structured-output showcase. A signed-in bank customer asks
natural-language questions ("What's my balance?", "How much did I spend on groceries last month?") and an LLM
agent answers by calling the customer's own `Balance` and `Transactions` as tools, returning a structured verdict
(advice, a card-block flag, and a 0-10 risk score). It also subsumes the standalone "structured output" example:
`Support` is exactly typed structured LLM output from a functional endpoint, inside a real scenario.

### Single microservice with an in-memory demo store

`banksupport` is a single host: it owns the login surface, the `/demo` console, the LLM agent, and a small
in-memory store of demo accounts and their transaction histories (`populate.go`). The store is built once in
`OnStartup` and is read-only afterward, so it needs no lock. It is fixture data, not real persistence; the
example's lessons are authentication, actor-scoped LLM tools, durable workflows, and structured output, and an
in-memory store keeps the focus there.

### Confused-deputy protection

`Balance` and `Transactions` take **no** customer-id argument. Each reads the username from `frame.Of(ctx)` and
scopes its lookup to that account. This is the whole point of exposing a data-backed endpoint as an LLM tool:
because the identity comes from the verified claim and never from tool input, the model (or any caller) can only
ever read the signed-in customer's own data. `requiredClaims: "roles.customer"` additionally guarantees
there *is* an authenticated customer, and makes the tools disappear from the OpenAPI tool list for anyone who
is not one.

### The agent is a durable workflow, not a synchronous Chat

`Support` is a **workflow**, not a synchronous `llmapi.Chat` call, and this is deliberate. A multi-turn
tool-calling agent conversation (the model reasons, calls `Balance`/`Transactions`, reasons again) routinely
outlasts a single request's time budget: `llmapi.Chat` runs the whole loop under one shrinking deadline, so a
mid-loop provider call gets whatever budget is left and 408s. `Support`'s one task, `RunSupport`, instead runs
`llm.core`'s `ChatLoop` as a subgraph, so every turn is its own durable foreman step with a fresh per-step budget
- the framework's prescribed way to use an LLM inside a workflow. (This is why `banksupport` depends on
`foreman.core` and not directly on `llm.core`: the `ChatLoop` subgraph is foreman-dispatched.)

The `Demo` POST is the trigger: it `foreman.Create`s the `Support` workflow and returns immediately with the flow
key (well within the ingress budget), rather than blocking on the flow. The demo owns the `foremanapi` dependency
because it is the triggering UI - the same self-contained-service shortcut the `creditflow` demo uses; a workflow
*provider* would not import `foremanapi`.

### DemoStatus: a long-poll over the foreman

The browser learns the result by polling `DemoStatus`, which is a **long-poll**, not a fixed-interval poll. It
delegates to the foreman's `Poll` (via `foremanapi.PollAndParse`): `Poll` waits up to the request's time budget
for the flow to stop, but unlike `Await` it does not treat a timeout as an error - a still-running flow comes back
as a non-error `running` outcome, so the page can re-fetch *immediately* with no client-side delay. A completed
flow's verdict is delivered the instant it finishes; an in-flight one holds the connection efficiently instead of
hammering. The headroom that returns the running outcome *before* the ingress cuts the request lives inside the
foreman's `Poll` handler, not here. `DemoStatus` itself only maps the outcome: on a stopped, completed flow it
returns the parsed structured verdict, otherwise a `running`, `failed`, or `cancelled` status.

The launch-then-poll split is needed because the agent's tool-calling loop routinely outlasts a single request's
budget. A workflow whose tasks finish within one request could instead `Await` synchronously in the web handler
(as the `creditflow` demo does), but an LLM flow cannot.

### Login and the 401 redirect

`Login` mirrors `login.example`: it validates a hardcoded demo customer, mints a bearer token via
`bearertokenapi.Mint`, sets it as the `Authorization` cookie, and redirects to `/demo`. The demo customers are
`alice` (a healthy surplus) and `bob` (spends beyond income, trends into overdraft to exercise the risk/block
path). The app wires a `middleware.ErrorPageRedirect` scoped to the `/banksupport.example/` path prefix so an
unauthenticated hit on a gated page bounces to `/login`.

### Structured output by prompt-and-parse

The LLM APIs have no native response-schema option, so structured output is achieved by instructing the model to
end with a strict JSON object and parsing it (`parseVerdict`, tolerant of code fences and stray prose, clamps risk
to 0-10 and falls back to treating the whole reply as advice). `RunSupport` parses the `ChatLoop` result this way.
This is the portable way to get typed output over the bus today.

### Seeded, deterministic demo data

Seeding lives in `populate.go`, isolated from the request handlers. `populateDemoData` runs once from `OnStartup`
and builds the in-memory accounts map; because the map is read-only afterward, no lock is needed. Each account
gets several months of synthesized history in a single pass, and its resulting balance is stored. Fixed
obligations (salary, rent, utilities) are constant, while discretionary streams (groceries, dining, transport) are
jittered +/-20% so the ledger reads like a real statement rather than a metronome.

The jitter is seeded deterministically from a hash of the username (`seededRand`), and that determinism is
load-bearing, not merely reproducibility: every replica of `banksupport` builds an identical store, so a
customer's unicast requests - which round-robin across replicas - always see the same balance and history. True
randomness would let replicas disagree, and the same customer would get different answers on successive calls.
Determinism also keeps bob reliably overdrawn - his fixed monthly surplus is dwarfed by discretionary spend, and
+/-20% jitter cannot flip that - which is what exercises the agent's risk/block path.

### Needs a real provider

The agent uses `llmapi.ProviderAny`/`ModelDefault` and needs a real LLM provider key to do tool-calling and
produce the structured verdict; the simulated `chatbox.example` provider does not tool-call. The data and login
flows work without a key.

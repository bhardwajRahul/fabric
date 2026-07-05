# weather.example

**CRITICAL**: This directory is a Microbus microservice. Before performing any task, check for pertinent skills in `.claude/skills/` and its subdirectories. Follow the workflow of the most relevant skill.

## Agent Instructions

This microservice implements agentic workflows. See `.claude/rules/workflows.txt` for the conventions.

## Design Rationale

### Purpose

Weather is the suite's canonical answer to "how do I build an agent?" The agent is the `AskAgent`
**workflow**: it exposes two of its own endpoints,
`LatLng` and `Forecast`, as LLM tools and lets the model chain them, geocoding a location and then fetching
its conditions, to compose a natural-language answer.

### The agent is a workflow, not a function

Agents in Microbus are workflows. This example leads with that default deliberately, because it is the first
thing a reader meets when learning to build an agent, and the first example sets the mental model. The
`AskAgent` workflow is the headline teaching surface; the microservice also ships a plain synchronous `Ask`
function (calling `llmapi.Chat`) purely as the tour's clickable endpoint (see "Ask runs the agent
synchronously, without the foreman"). The two share helpers and stay in lockstep, but the workflow is the
form the reader should learn from.

`AskAgent` is a single-node graph whose one task, `Answer`, runs `llm.core`'s `ChatLoop` as a subgraph via
`llmapi.NewSubgraph(flow).ChatLoop(...)`. The node count is one, but the depth is real: `ChatLoop` is a
multi-step workflow (call the model, execute each tool, loop), so this is genuine subgraph composition, not
a graph wrapped around a one-shot call. As the agent grows (more tools, a validation-retry loop, a
human-in-the-loop approval), nodes are added to this same graph; the function form would have to be
rewritten. `creditflow` and the flight-booking example carry the larger graph, subgraph, and Interrupt/Resume
stories from here.

### Workflow vs. synchronous call

The distinction this example is meant to teach:

- **Synchronous `Chat`** (as in `chatbox`) runs the whole tool-calling loop in one Go call. It keeps no
  persisted state and must finish within a single time budget. Right for a quick request/response agent.
- **`ChatLoop` as a workflow** (here) is durable: each step persists its state and gets its own time budget,
  so the agent can span far more work than one request could, survive a worker restart mid-run, and be
  observed step-by-step in agentstudio. Right for anything you want durable, resumable, or multi-step.

### Single host, three endpoints

The example deliberately puts geocoding, forecasting, and the agent on one host. In a production system the
geocoding and weather wrappers would each be their own microservice so their third-party credentials and
rate limits are isolated. They are combined here to keep the example readable as one unit and cheap to run
in the tour.

### Ask runs the agent synchronously, without the foreman

`Ask(q) (answer)` is a plain functional endpoint that runs the same tool-calling loop as the `AskAgent`
workflow, but synchronously: it calls `llm.core`'s `Chat` (the in-process loop) with the same system prompt
and the same `LatLng`/`Forecast` tools, and returns the final assistant message. `Answer` (the workflow task)
and `Ask` (the function) share the `weatherAgentPrompt` and `finalAnswer` helpers, so the two forms of the
agent stay in lockstep. `Ask` exists to give the guided tour one browser-clickable URL (`/ask?q=...`).

Crucially, `Ask` does **not** go through the foreman, and this microservice does not import `foremanapi` at
all. A microservice that *provides* a workflow must not depend on the execution engine to run it - the foreman
is to workflows what the ingress proxy is to inbound HTTP, and a provider no more calls the foreman to run its
own graph than it calls the ingress proxy to serve its own endpoint. Launching `AskAgent` durably is the job
of the code that owns the *triggering event* (a UI/API request, a cron ticker, an inbound event); that code
takes the `foremanapi.Run` dependency, usually in a different microservice, though a service that genuinely
owns the trigger (its own UI or ticker) may launch it as a self-contained shortcut. Weather has no such
trigger, so it stays a clean provider with zero `foremanapi` imports. This is the general rule captured in
`.claude/rules/workflows.txt`.

That `Ask` *can* run this agent synchronously at all is itself the tell: this demo agent is short enough that
it does not strictly need to be a workflow. The example still models it as one (see "The agent is a workflow,
not a function") because that is the shape you want the moment the agent grows a validation-retry loop, an
approval pause, or more tools - work that outlives a single request budget and so can no longer be a
synchronous call. Keep the durable form as the teaching surface; treat `Ask` as the short synchronous twin
that the tour clicks.

### Mock data, not a real API

`LatLng` and `Forecast` return deterministic mock data rather than calling a real geocoding/weather service.
A handful of well-known cities map to their true coordinates; every other input is hashed to a stable
pseudo-coordinate, and forecasts are derived from the coordinate (warmer near the equator, plus a
hash-driven wobble). This keeps the example self-contained: the only external dependency is an LLM provider
key, not a third-party weather account. The trade-off is that the numbers are fictional; a variant that
calls a real API through the HTTP egress proxy would swap the mock bodies for `httpegressapi` calls without
changing the agent shape.

### Provider portability

`Answer` passes `llmapi.ProviderAny` and `llmapi.ModelDefault` to `ChatLoop`, so the example runs against
whichever single provider key the user has configured (Claude, Gemini, or ChatGPT) rather than hardcoding
one. This is why it belongs in the guided tour behind a single "needs a real LLM provider key" gate: the
simulated `chatbox.example` provider does not do real tool-calling, so this example needs a real provider.

# weather.example

**CRITICAL**: This directory is a Microbus microservice. Before performing any task, check for pertinent skills in `.claude/skills/` and its subdirectories. Follow the workflow of the most relevant skill.

## Agent Instructions

This microservice implements agentic workflows. See `.claude/rules/workflows.txt` for the conventions.

## Design Rationale

### Purpose

Weather is the Microbus port of the Pydantic AI weather-agent example, and the suite's canonical answer to
"how do I build an agent?" The agent is the `AskAgent` **workflow**: it exposes two of its own endpoints,
`LatLng` and `Forecast`, as LLM tools and lets the model chain them, geocoding a location and then fetching
its conditions, to compose a natural-language answer.

### The agent is a workflow, not a function

Agents in Microbus are workflows. This example models that default deliberately rather than shipping the
simpler synchronous form, because it is the first thing a reader meets when learning to build an agent, and
the first example sets the mental model. The synchronous shape (calling `llmapi.Chat` from a plain handler)
is already demonstrated by `chatbox`, so nothing is lost by making this one purely the workflow pattern.

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

# flightbooking.example

**CRITICAL**: This directory is a Microbus microservice. Before performing any task, check for pertinent skills in `.claude/skills/` and its subdirectories. Follow the workflow of the most relevant skill.

## Agent Instructions

This microservice implements agentic workflows. See `.claude/rules/workflows.txt` for the conventions.

## Design Rationale

### Purpose

FlightBooking is the suite's showcase for the parts of the framework that a synchronous LLM call cannot
express: a durable graph, subgraph composition, and a
**real human-in-the-loop pause** via `flow.Interrupt` / `foreman.Resume`. Where `weather.example` is a
single-node agent and `creditflow.example` is human-in-the-loop *by branching*, this example parks the flow on
an actual Interrupt and resumes it with an external decision.

### The two ways to compose, taught deliberately

The example teaches the graph-composition vs. subgraph distinction on purpose:

- **Graph composition** - `SearchFlights`, `ProposeFlight`, `AwaitDecision`, `ChooseSeat`, `ConfirmBooking`, and
  `NoFlights` are nodes in the one `BookFlight` graph. They share its full state vocabulary (candidates,
  flightIndex, currentFlight, seat, ...) and hand off through transitions.
- **Subgraph** - `ChooseSeat` invokes the separate `ChooseSeatAgent` workflow as an isolated child via the
  generated `flightbookingapi.NewSubgraph(flow).ChooseSeatAgent(...)` client. Only the explicit seat preference
  and available-seat list cross in, and only the chosen seat crosses back. `ChooseSeatAgent` in turn composes
  `llm.core`'s `ChatLoop` as a further subgraph, so the seat choice is a genuine two-level nested subgraph.

### The Interrupt/Resume loop

`AwaitDecision` is the heart of the example. On first dispatch it calls `flow.Interrupt` with the proposed flight
as the payload and returns, parking the flow in `interrupted` status. The `Demo` page reads the payload,
presents the flight, and calls `foreman.Resume(flowKey, {accepted})`. On re-entry the task reads the decision:
accept takes the normal transition to `ChooseSeat`; keep-searching advances `flightIndex` and drives
`flow.Goto("ProposeFlight")` to loop back and propose the next candidate. `ProposeFlight` uses a `Switch` to end
the search at `NoFlights` once the candidate list is exhausted (no flights on the route, or every candidate
declined).

### Why the Demo owns the foreman dependency

A workflow's provider must not import `foremanapi` to run its own graph (the foreman is infrastructure in front
of the workflow, not a downstream dependency). This microservice is the documented exception: it *owns the
triggering surface* - the `Demo` web page is the UI that starts and resumes the flow - so the `foremanapi`
`Create` / `Await` / `Resume` calls live in the `Demo` handler, which is triggering code, not workflow-provider
code. That is the same self-contained-service shortcut `creditflow`'s demo uses. In a real system the booking UI
would be its own microservice and this one would expose only the graph and tasks.

### Why a Demo web UI rather than plain functions

Interrupt/Resume is inherently stateful and multi-round: search, propose a candidate, await a human decision,
then propose the next or book. A single stateless functional URL cannot round-trip that, because the flow parks
and must be resumed by `flowKey`. The `Demo` page carries the `flowKey` in a hidden form field across the
Accept / Keep-searching buttons so the traveler never copy-pastes it, which is why the tour stop is a web UI and
not a pair of clickable functional endpoints.

### Seat selection degrades without a provider key

`ChooseSeat` calls the LLM-backed `ChooseSeatAgent`. If no real provider is configured the subgraph errors;
`ChooseSeat` catches it, logs a warning, and falls back to the first available seat so the human-in-the-loop
booking still completes end-to-end. The LLM only refines the seat choice, so the headline (Interrupt/Resume,
subgraph, goto loop) is fully demonstrable with no key, and a configured provider adds the natural-language seat
match on top. This is the deliberate difference from `weather.example`, whose entire output is the LLM answer and
so cannot degrade.

### Mock data, not a real API

`SearchFlights` returns deterministic mock flights for a handful of routes rather than calling a real GDS. This
keeps the example self-contained; a production variant would wrap a booking API behind the HTTP egress proxy
without changing the workflow shape.

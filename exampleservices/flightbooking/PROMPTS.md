## Create the flight-booking example agent

Implement `flightbooking.example`, a flight-booking example, as an example microservice under
`exampleservices/`. It is the suite's showcase for durable agentic workflows: a graph, subgraph
composition, and a real human-in-the-loop pause via `flow.Interrupt` / `foreman.Resume`.

Model the `BookFlight` workflow as a graph that searches a route (deterministic mock flight data, no real API),
proposes candidate flights one at a time, and parks on a human accept/keep-searching decision. Accepting a
flight runs a child `ChooseSeatAgent` workflow as an isolated subgraph to pick a seat from the flight's
available seats matching a natural-language preference (via `llm.core`'s `ChatLoop`), then confirms the booking.
Keep-searching advances to the next candidate via a goto loop; an exhausted candidate list ends with a
not-booked message.

Expose the human-in-the-loop interaction as a `/demo` web page (Interrupt/Resume is stateful and multi-round, so
a single functional URL cannot round-trip it): the page starts the flow, presents each proposed flight, and its
Accept / Keep-searching buttons resume the parked flow, carrying the flow key in a hidden field. The demo owns
the `foremanapi` trigger because it is the triggering surface, matching the `creditflow` demo's shortcut.

Seat selection must degrade gracefully to the first available seat when no LLM provider is configured, so the
human-in-the-loop booking completes end-to-end without a key; a configured provider adds the natural-language
seat match.

Then integrate the example into the agent-guided tour (`take-tour` skill) as an agentic-workflow stop showing
Interrupt/Resume, gated like the other LLM stops behind a real-provider-key note but usable without one.

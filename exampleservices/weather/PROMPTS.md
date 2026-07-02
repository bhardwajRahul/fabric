## Weather Agent example

Port the Pydantic AI weather-agent example to Microbus as a single self-contained example microservice
under `exampleservices/`. The agent answers natural-language weather questions through sequential
tool-calling: one tool geocodes a location to latitude/longitude, a second fetches that location's
forecast, and the model chains them to compose an answer.

This example is the suite's canonical answer to "how do I build an agent?" Agents in Microbus are
workflows, so the agent is built as a workflow, not a synchronous function: the simpler synchronous
`llmapi.Chat` shape is already demonstrated by `chatbox`, and a first agent example should model the
default. Keep it to a single host (`weather.example`) with:

- Two functional endpoints, `LatLng` (geocode a place name) and `Forecast` (conditions for coordinates),
  returning deterministic mock data so the example needs no third-party weather account. In a production
  system these wrappers would each be their own microservice so their credentials and rate limits are
  isolated; they are combined here to keep the example to one host.
- A workflow, `AskAgent`, whose single task `Answer` runs `llm.core`'s `ChatLoop` as a subgraph via
  `llmapi.NewSubgraph(flow).ChatLoop(...)`, exposing `LatLng` and `Forecast` as the LLM's tools. The one
  node is real subgraph composition: `ChatLoop` is itself multi-step, so the workflow is durable and each
  step gets its own time budget.

`Answer` passes `llmapi.ProviderAny` and `llmapi.ModelDefault`, so the example runs against whatever single
provider key the user has configured. Only a real LLM provider key is required to run it end-to-end; the
tests mock `ChatLoop` and need no key.

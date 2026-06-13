## Initial scaffold

Create a new microservice `agentstudio` in `devservices/` with hostname `agentstudio.dev`.

Add the following web endpoints:
- At `/`, show a list of flows (sourced from `foreman.core`) in a matrix table with columns for
  status, current task, workflow name, error, and an action to drill in. Pagination and sorting
  use the bespa Table widget. Clicking a row opens the flow detail screen.
- At `/flow/{flowKey}`, render a detail page. The top shows global properties (flow key, thread
  key, workflow URL, status, step count, created/updated times, error/cancel reason). Below,
  two tabs: "DAG" (the Mermaid diagram of the history) and "Log" (the step history as a table).

Use the [bespa](https://github.com/microbus-io/bespa) library to render the pages, with a
`replace` directive in `go.mod` pointing at the sibling checkout.

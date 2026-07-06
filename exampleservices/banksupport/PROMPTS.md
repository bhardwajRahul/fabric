## Create the bank-support agent example

Build a bank customer-support example, a single microservice `banksupport.example` under `exampleservices/`, that
showcases JWT authentication, actor-claims authorization, actor-scoped LLM tools, and structured LLM output. It
owns the login surface, the `/demo` support console, and the LLM agent, and holds a small in-memory store of demo
accounts and their transaction histories (fixture data, not real persistence).

The customer is a **user/actor identity**: login mints a bearer token with `sub=<username>` and
`roles:["customer"]`, and `banksupport` derives the username from the verified actor claim and scopes every lookup
itself.

Expose `Balance` and `Transactions` as LLM tools that derive the customer from the actor claim (never an argument)
and read only that customer's data from the in-memory store, giving confused-deputy protection. `Support` runs the
tool-calling loop and returns a structured verdict `{advice, blockCard, risk}` (structured output via
prompt-for-JSON + parse, since `llmapi.Chat` has no native response schema). All customer endpoints are gated with
`requiredClaims: "roles.customer"`; login is public and redirects to `/demo` on success; the app adds a
401-to-`/login` redirect middleware scoped to the `/banksupport.example/` prefix.

Seed demo data (customers `alice`, healthy, and `bob`, overdrawn) once in `OnStartup`, building the in-memory
accounts map: synthesize several months of history per account in a single pass, then store the resulting balance.
Fixed obligations (salary, rent, utilities) are constant while discretionary streams (groceries, dining,
transport) are jittered +/-20%, seeded deterministically from the username so histories are varied but
reproducible, every replica builds an identical store, and `bob` reliably overdraws.

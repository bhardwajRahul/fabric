# Coding Agents

`Microbus` was designed from the ground up to make coding agents effective at creating microservices. Its architecture, conventions and tooling are optimized so that agents can work with small, well-defined units of code rather than sprawling monolithic codebases. The benefits include:

- **Tighter build-test-debug cycles** - microservices are small enough to build and test quickly, giving agents rapid feedback on their changes
- **More accurate code** - focused context and clear conventions reduce the likelihood of hallucinations and errors
- **Smaller contexts that fit in memory** - each microservice is self-contained, so agents don't need to load an entire codebase to be productive
- **Parallelism** - independent microservices can be developed by multiple agents working in parallel

<img src="./coding-agents-1.drawio.svg">
<p></p>

### What Agents Produce

For each microservice, coding agents produce:
- The implementation code of the microservice's [features](../blocks/features.md)
- A [client stub](../blocks/client-stubs.md) that is used by upstream microservices to call the microservice in a type-safe manner
- [Integration tests](../blocks/integration-testing.md) that thoroughly test it along with its downstream dependencies
- An [OpenAPI](../blocks/openapi.md) endpoint that describes its API
- Documentation

All code follows a [uniform code structure](../blocks/uniform-code.md). A familiar structure helps agents get oriented quickly. Often a quick glance at `manifest.yaml` saves the agent from reading a thousand lines of code.

### How Agents Are Guided

Coding agents don't operate on instinct alone. `Microbus` provides a layered system of instructions that train the agent to work correctly.

[`AGENTS.md`](../blocks/agents-md.md) files provide context at two levels. A global `AGENTS.md` at the root of the project includes instructions applicable to the project as a whole, while a local `AGENTS.md` in each microservice's directory keeps context specific to that microservice. The local file is maintained by the agent itself as it works.

`.claude/rules/microbus.md` contains the conventions and patterns for working on a `Microbus` solution. This file is updated with each release of `Microbus` and should not be edited by hand.

`PROMPTS.md` files in each microservice's directory keep an auditable trail of the prompts that shaped the microservice. This allows a future agent to reproduce the microservice from scratch.

Skills are predefined workflows in `.claude/skills/` that guide the agent step by step through complex multi-step tasks. When a developer's prompt matches a skill, the agent follows that skill's workflow rather than improvising. This makes the agent's behavior predictable and its output consistent.

### Incremental Development

Features are added to a microservice one at a time. Each prompt results in a focused, incremental change: new code is added without impacting existing code, tests are written or updated, and the manifest is kept in sync. This incremental approach keeps changes small and reviewable.

### Build-Test-Debug Cycle

After making changes, agents build the code and run the [integration tests](../blocks/integration-testing.md) to verify correctness. If a test fails, the agent reads the error, adjusts the code and tries again. Because microservices are small and tests run fast, this feedback loop is tight - typically a few seconds per iteration.

### Prompting

Coding agents do best when asked to perform a single focused task such as the creation of a new microservice or the addition of a [feature](../blocks/features.md) to an existing microservice.

#### Add a Microservice

> HEY CLAUDE...
>
> Add a new microservice "fincalc" with the hostname "fincalc.example". This service will perform financial calculations such as mortgage payments. Place it in the @examples directory.

#### Add a Configuration Property

> HEY CLAUDE...
>
> Add a configuration property for the API key we'll need to call the Federal Reserve web service to obtain the current interest rates. Set the key "RAYxeLwtuiFazz0RCGnE" in @config.local.yaml for development purposes.

#### Add a Functional Endpoint

> HEY CLAUDE...
>
> Add a functional endpoint that calls the Federal Reserve web service and returns the current 30-year and 15-year mortgage interest rates from the Federal Reserve web service. Use caching to reduce the number of calls to the web service.

> HEY CLAUDE...
>
> Add a functional endpoint that accepts a loan amount and returns the monthly payments for the 15-year and 30-year mortgages.

#### Add an Outbound Event

> HEY CLAUDE...
>
> Add an outbound event "OnInterestRatesChange" that will be triggered when interest rates change.

#### Add an Inbound Event Sink

> HEY CLAUDE...
>
> Add an inbound event sink to handle the "OnMarketClose" and "OnMarketOpen" events. Keep track of the market status.

#### Add a Web Endpoint

> HEY CLAUDE...
>
> Add a web endpoint that presents the user with a form to enter a loan amount, and on submittal, shows the user the monthly payments for a 15-year and 30-year mortgage.

#### Add a Metric

> HEY CLAUDE...
>
> Add a histogram metric that will keep track of loan amounts entered by the user in the web form.

#### Add a Ticker

> HEY CLAUDE...
>
> Add a ticker that runs once an hour to obtain the current interest rates and fires the "OnInterestRatesChange" event if they changed.

#### Remove a Feature

> HEY CLAUDE...
>
> Remove the "OnInterestRatesChange" outbound event from the fincalc microservice.

> HEY CLAUDE...
>
> Remove the MaxRetries config property.

#### Externalize and Translate Text

> HEY CLAUDE...
>
> Externalize the user-facing strings in the fincalc microservice and add Spanish translations.

#### Upgrade a Microservice

> HEY CLAUDE...
>
> Upgrade the fincalc microservice from v1 to v2.

#### Upgrade Microbus

> HEY CLAUDE...
>
> Get the latest version of Microbus.

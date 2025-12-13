# Coding Agents

In `Microbus`, coding agents such as Claude work hand in hand with the [code generator](../blocks/codegen.md). The coding agent triggers calls to the code generator based on commands by the developer. In response, the code generator produces deterministic boilerplate code sprinkled with hints to help guide the coding agent. The coding agent then fills in the gaps with the appropriate implementation of the business logic.

<img src="./coding-agents-1.drawio.svg">
<p></p>

This approach reduces the size of the context window that the coding agent needs to complete the job, yielding:
- Faster iteration
- More accurate code
- More efficient token usage

### Prompting

Coding agents do best when asked to perform a single focused task such as the creation of a new microservice or the addition of a feature to an existing microservice, to which there is a defined skill.

After the coding agent finishes a task, be sure to check that it didn't skip any work items such as writing tests.

#### Add a Microservice

```
Add a new microservice "fincalc" with the hostname "fincalc.example". This service will perform financial calculations such as mortgage payments. Place it in the @examples directory.
```

"Microservice" is the key term for this feature.

#### Add a Configuration Property

```
Add a configuration property for the API key we'll need to call the Federal Reserve web service to obtain the current interest rates. Set the key "RAYxeLwtuiFazz0RCGnE" in @main/config.yaml for development purposes.
```

"Configuration property" is the key term for this feature.

#### Add a Functional Endpoint

```
Add a functional endpoint that calls the Federal Reserve web service and returns the current 30-year and 15-year mortgage interest rates from the Federal Reserve web service. Use caching to reduce the number of calls to the web service.
```

```
Add a functional endpoint that accepts a loan amount and returns the monthly payments for the 15-year and 30-year mortgages. 
```

"Functional endpoint" is the key term for this feature.

#### Add an Outbound Event

```
Add an outbound event "OnInterestRatesChange" that will be triggered when interest rates change.
```

"Outbound event" is the key term for this feature.

#### Add an Inbound Event Sink

```
Add an inbound event sink to handle the "OnMarketClose" and "OnMarketOpen" events. Keep track of the market status.
```

"Inbound event sink" is the key term for this feature.

#### Add a Web Endpoint

```
Add a web endpoint that presents the user with a form to enter a loan amount, and on submittal, shows the user the monthly payments for a 15-year and 30-year mortgage.
```

"Web endpoint" or "web handler" are the key terms for this feature.

#### Add a Metric

```
Add a histogram metric that will keep track of loan amounts entered by the user in the web form.
```

"Metric" is the key term for this feature. There are three metric types: "counter", "gauge" and "histogram".

#### Add a Ticker

```
Add a ticker that runs once an hour to obtain the current interest rates and fires the "OnInterestRatesChange" event if they changed.
```

"Ticker" or "recurring operation" are the key terms for this feature.

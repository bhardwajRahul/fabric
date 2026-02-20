---
name: Chart Topology Diagram
description: Regenerates the topology diagram of the application's microservices. Use when microservices are added or removed from main/main.go, or when downstream dependencies change.
---

## Workflow

Copy this checklist and track your progress:

```
Chart the microservice topology:
- [ ] Step 1: Analyze project
- [ ] Step 2: Generate Mermaid diagram
```

#### Step 1: Analyze Project

Read `main/main.go` to identify the included microservices by their import paths.

Read the `manifest.yaml` of each of the included microservice to get its `name`, `hostname`, `downstream` dependencies, `db` and `cloud` properties.

#### Step 2: Generate Mermaid Diagram

Regenerate `main/topology.mmd`, a Mermaid diagram of all included microservices and the dependencies among them.

**Every** microservice added to the app in `main/main.go` must appear in the diagram — including core services, not just those that appear as downstream dependencies.

To build the diagram:
- Generate a `graph TB` diagram
- Use the `hostname` of the microservice as the node key, and `Name<br>hostname` as the node label
- Draw an edge `--->` (three dashes) from each upstream service to its downstream dependencies
- Draw a dotted edge `-..->` from each event source microservice to the event sink microservice
- Sort nodes by the reverse of the hostname (e.g. `hello.example` becomes `example.hello`) so that nodes with the same domain suffix are grouped together
- Include microservices that have no edges as standalone nodes
- If a microservice has a `db` property in its manifest, draw a `---` edge (two dashes, short) to a cylinder node `[(value)]` where `value` is the value of the `db` property. Name the cylinder node using the hostname of the microservice with a `.db` suffix
- If a microservice has a `cloud` property in its manifest, draw a `---` edge (two dashes, short) to a cloud node `@{shape: cloud, label: "value"}` where `value` is the value of the `cloud` property. Name the cloud node using the hostname of the microservice with a `.cloud` suffix
- Define the styling classes `core`, `svc` and `ext` as shown in the example below
- Assign classes using `class` statements at the end of the diagram (do NOT use inline `:::` syntax, as it is incompatible with some node shapes like cloud)
- Apply class `svc` to non-core microservice nodes
- Apply class `core` to core microservice nodes (hostnames ending in `.core`)
- Apply class `ext` to `db` cylinder nodes and `cloud` nodes

Example:

```mermaid
graph TB
    classDef core fill:#ed2e92,color:#f4f2ef,stroke-width:0px
    classDef svc fill:#32a7c1,color:#f4f2ef,stroke-width:0px
    classDef ext fill:#e5f4f3,color:#434343,stroke:#434343

    book.example[Book<br>book.example]
    browser.example[Browser<br>browser.example] ---> http.egress.core[HTTP Egress<br>http.egress.core]
    directory.example[Directory<br>directory.example] --- directory.example.db[(SQL)]
    eventsource.example[EventSource<br>eventsource.example] -..-> eventsink.example[EventSink<br>eventsink.example]
    fetcher.example[Fetcher<br>fetcher.example] --- fetcher.example.cloud@{shape: cloud, label: "Stripe"}
    hello.example[Hello<br>hello.example] ---> calculator.example[Calculator<br>calculator.example]
    configurator.core[Configurator<br>configurator.core]
    http.ingress.core[HTTP Ingress<br>http.ingress.core]

    class hello.example,calculator.example,eventsource.example,eventsink.example,browser.example,directory.example,fetcher.example,book.example svc
    class http.egress.core,configurator.core,http.ingress.core core
    class directory.example.db,fetcher.example.cloud ext
```

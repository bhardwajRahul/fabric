---
name: housekeeping
description: Run after completing any change to a microservice. Vets compilation, updates manifest.yaml, documentation, version, and topology diagram. Skip if the skill you just followed already includes housekeeping as a final step.
---

**CRITICAL**: Read and analyze this microservice before starting. Do NOT explore or analyze other microservices. The instructions in this skill are self-contained to this microservice.

## Workflow

Copy this checklist and track your progress:

```
Post-change housekeeping:
- [ ] Step 1: Vet compilation
- [ ] Step 2: Reconcile the manifest
- [ ] Step 3: Document the microservice
- [ ] Step 4: Versioning
- [ ] Step 5: Log the prompts
- [ ] Step 6: Visualize workflows
- [ ] Step 7: Chart the topology
```

#### Step 1: Vet Compilation

Run `go vet ./main/...` and fix any compilation errors before proceeding.

#### Step 2: Reconcile the Manifest

Update `manifest.yaml` to reflect the changes. Ensure that all sections accurately describe the current state of the microservice's features, including `general`, `functions`, `webs`, `configs`, `tickers`, `metrics`, `outboundEvents`, `inboundEvents`, `tasks`, `workflows`, and `downstream`. Set `modifiedAt` under `general` to the current time in RFC 3339 format.

#### Step 3: Document the Microservice

Skip this step if instructed to be "quick" or to skip documentation, or if the change introduces no new design rationale or tradeoffs worth capturing.

Update the microservice's local `CLAUDE.md` file to reflect the changes. If the file does not exist, create it with the hostname as an H1 heading (from `manifest.yaml`). Capture purpose, design rationale, tradeoffs, and the context needed for someone to safely evolve this microservice in the future. Focus on the reasons behind decisions rather than describing what the code does.

#### Step 4: Versioning

Increment the `Version` const in `intermediate.go`.

#### Step 5: Log the Prompts

Skip this step if the change is too minor to affect how you'd describe the microservice to a new agent.

Update `PROMPTS.md` to reflect the current capabilities of the microservice — what you would need to prompt to reproduce it from scratch today. Rewrite or extend the existing content as needed. The goal is a concise, up-to-date description a future agent could use to recreate the microservice, not a history of every change.

#### Step 6: Visualize Workflows

Skip this step if the microservice's `manifest.yaml` does not have a `workflows` section, or if instructed to be "quick" or to skip documentation.

For each workflow defined in the `workflows` section of `manifest.yaml`, generate a Mermaid flowchart and save it to a separate `.mmd` file named after the workflow in ALLCAPS, e.g. `MYWORKFLOW.mmd`.

To generate the Mermaid output, read the graph builder function in `service.go`, reproduce its structure, and call `graph.Mermaid()`. After generating the output, strip the `hostname:428/` prefix from node labels where `hostname` matches this microservice's hostname and the port is `:428`. Do not strip hostnames of other microservices or labels using a different port.

After generating the diagram, apply the following `classDef` styles and assign classes to each node:

```
classDef task fill:#32a7c1,color:#f4f2ef,stroke:#434343
classDef sub fill:#ed2e92,color:#f4f2ef,stroke:#434343
classDef term fill:#e5f4f3,color:#434343,stroke:#434343
```

- `:::task` - regular task nodes
- `:::sub` - nodes registered as subgraphs via `AddSubgraph`
- `:::term` - title, start `(( ))`, and end `(( ))` nodes

Add a title node at the top of the graph using the workflow's route name in kebab-case: `_title{{"my-workflow"}}:::term --> _start`. The title connects to the start node.

#### Step 7: Chart the Topology

This step is required if the `downstream` section of `manifest.yaml` or `main/main.go` were changed. Otherwise, skip it.

Follow the `chart-topology` skill.

---
name: housekeeping
description: Run after completing any change to a microservice. Vets compilation, updates manifest.yaml, documentation, version, and topology diagram. Skip if the skill you just followed already includes housekeeping as a final step.
---

**CRITICAL**: Read and analyze this microservice before starting. Do NOT explore or analyze other microservices - the instructions in this skill are self-contained to this microservice. NATS ACLs are derived from source at deploy time by `cmd/gencreds` (not at housekeeping time), so a code change here doesn't require regenerating anything in other services.

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

Run `go run github.com/microbus-io/fabric/cmd/genmanifest --path .` from the microservice's directory to regenerate `manifest.yaml` from the code. The tool extracts:

- Service identity, description, configs, metrics, tickers from `intermediate.go` and `service.go`.
- Endpoint declarations from `intermediate.go`'s `Subscribe(...)` blocks and `*api/endpoints.go`'s `Def` literals.
- The `inboundEvents` section, populated by detecting `Hook.OnEvent(...)` calls. Only the source package is recorded; the resolved hostname/route/method are derived from source at deploy time by `cmd/gencreds`.

The manifest captures only what this microservice exposes. Outbound dependencies (which other microservices it calls, which databases it reads, which external hosts it dials) are not stored in the manifest at all; they're rederived from source at deploy time.

The tool preserves operator-curated fields (`general.name`, `general.frameworkVersion`) and updates `modifiedAt` to the current UTC time only when other content actually changed. Review the diff.

NATS ACL files are NOT generated at housekeeping time - they're derived from source by `cmd/gencreds` at deploy. Do not invoke `cmd/gencreds` from this skill; it consumes operator-sensitive material (the account NKey) and runs in the CD pipeline.

#### Step 3: Document the Microservice

Skip this step if instructed to be "quick" or to skip documentation, or if the change introduces no new design rationale or tradeoffs worth capturing.

Update the microservice's local `CLAUDE.md` file to reflect the changes. If the file does not exist, create it with the hostname as an H1 heading (from `manifest.yaml`). Capture purpose, design rationale, tradeoffs, and the context needed for someone to safely evolve this microservice in the future. Focus on the reasons behind decisions rather than describing what the code does.

#### Step 4: Versioning

Increment the `Version` const in `intermediate.go`.

#### Step 5: Log the Prompts

Skip this step if the change is too minor to affect how you'd describe the microservice to a new agent.

Update `PROMPTS.md` to reflect the current capabilities of the microservice - what you would need to prompt to reproduce it from scratch today. Rewrite or extend the existing content as needed. The goal is a concise, up-to-date description a future agent could use to recreate the microservice, not a history of every change.

#### Step 6: Visualize Workflows

Skip this step if the microservice's `manifest.yaml` does not have a `workflows` section, or if instructed to be "quick" or to skip documentation.

For each workflow defined in the `workflows` section of `manifest.yaml`, generate a Mermaid flowchart and save it to a separate `.mmd` file named after the workflow in ALLCAPS, e.g. `MYWORKFLOW.mmd`.

Call `graph.Mermaid()` and write its output verbatim to the `.mmd` file. The function emits a fully-styled diagram including the title node, classDef block, per-node class annotations, forEach (`st-rect`) and fan-in (`trap-t`) shapes, and `"fan-in"` labels on edges into `SetFanIn` nodes. Do not post-process the output.

Use a small throwaway program to invoke it (run from the microservice directory; substitute the workflow function name):

```go
// /tmp/genmmd.go
package main

import (
	"fmt"

	"github.com/microbus-io/fabric/<your/microservice/path>"
)

func main() {
	svc := <microservicePackage>.NewService()
	g, err := svc.<WorkflowFn>(nil)
	if err != nil {
		panic(err)
	}
	fmt.Print(g.Mermaid())
}
```

Then `go run /tmp/genmmd.go > <WORKFLOW>.mmd`.

#### Step 7: Chart the Topology

Run `go run github.com/microbus-io/fabric/cmd/gentopology --bundle main/main.go` from the project root to regenerate `main/topology.mmd`. The tool walks each bundled service's source for downstream typed-client calls, event hooks, SQL imports, and HTTP egress + external host detection. The resulting Mermaid diagram is committed alongside `main/main.go` and serves as the human-readable view of the system shape. Always safe to run; the file diff is the audit trail.

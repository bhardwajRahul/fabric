---
name: Housekeeping After a Change
description: Post-change housekeeping steps to perform after modifying a microservice. Includes updating the manifest, documentation, versioning, and prompt tracking. Use after completing a change to a microservice, unless the relevant skill already includes these steps.
---

**CRITICAL**: Read and analyze this microservice before starting. Do NOT explore or analyze other microservices. The instructions in this skill are self-contained to this microservice.

## Workflow

Copy this checklist and track your progress:

```
Post-change housekeeping:
- [ ] Step 1: Update manifest
- [ ] Step 2: Document the microservice
- [ ] Step 3: Versioning
- [ ] Step 4: Update prompts
- [ ] Step 5: Update topology diagram
```

#### Step 1: Update Manifest

Update `manifest.yaml` to reflect the changes. Ensure that all sections accurately describe the current state of the microservice's features, including `general`, `functions`, `webs`, `configs`, `tickers`, `metrics`, `outboundEvents`, `inboundEvents`, and `downstream`.

#### Step 2: Document the Microservice

Skip this step if instructed to be "quick" or to skip documentation, or if the change introduces no new design rationale or tradeoffs worth capturing.

Update the microservice's local `AGENTS.md` file to reflect the changes. Capture purpose, design rationale, tradeoffs, and the context needed for someone to safely evolve this microservice in the future. Focus on the reasons behind decisions rather than describing what the code does.

#### Step 3: Versioning

Increment the `Version` const in `intermediate.go`.

#### Step 4: Prompt History

Update `PROMPTS.md` in the microservice's directory with the prompt that motivated this change. Rephrase the language to include context that was not made explicit in the original prompt. The intent is to maintain an auditable trail of the prompts, and to allow a future agent to reproduce the functionality of the microservice from these prompts.

Save each prompt under a `## Title`.

```md
## Prompt title

Prompt comes here...
```

#### Step 5: Update Topology Diagram

This step is required if the `downstream` section of `manifest.yaml` or `main/main.go` were changed. Otherwise, skip it.

Follow the `microbus/chart-topology` skill.

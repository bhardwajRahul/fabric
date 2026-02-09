# Automatic Documentation

`Microbus` keeps documentation in sync with code by making documentation updates a required step in every coding agent skill workflow. When an agent adds, modifies or removes a feature from a microservice, the skill's checklist includes steps to update the relevant documentation files. This means documentation is never an afterthought — it's part of the definition of done.

Each microservice maintains three documentation files:

### `manifest.yaml`

The `manifest.yaml` is a structured inventory of every feature a microservice offers: endpoints, configuration properties, tickers, metrics, events and downstream dependencies. It is the fastest way to understand what a microservice does without reading its code. Agents are instructed to read all manifests when starting work on a project to build a mental map of the system.

### `PROMPTS.md`

The `PROMPTS.md` is an audit trail of every user prompt that shaped the microservice. Each entry is rephrased to include context that may not have been explicit in the original request. The intent is twofold: to provide an auditable history of decisions, and to allow a future agent to reproduce the microservice's functionality from these prompts alone.

### `AGENTS.md`

The `AGENTS.md` is a living record of design choices for the microservice. Every skill workflow that modifies a microservice includes a step to update `AGENTS.md` with the purpose, context, and design rationale behind the change. The focus is on the reasons behind decisions rather than describing what the code does — design choices, tradeoffs, and the context needed for someone to safely evolve the microservice in the future. It also serves as the entry point for coding agents working in the directory, directing them to the framework rules and reminding them to keep `PROMPTS.md` and `manifest.yaml` up to date. A companion `CLAUDE.md` file ensures that Claude reads `AGENTS.md` immediately upon entering the directory.

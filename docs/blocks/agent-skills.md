# Agent Skills

Skills are predefined workflows in `.claude/skills/` that guide a [coding agent](../blocks/coding-agents.md) step by step through complex multi-step tasks. When a developer's prompt matches a skill, the agent follows that skill's workflow rather than improvising. This makes the agent's behavior predictable and its output consistent across microservices and developers.

Without skills, a coding agent must infer the right approach from general patterns, which leads to inconsistencies: different naming conventions, missing test coverage, forgotten manifest updates, or structural drift between microservices. Skills eliminate that variance by encoding `Microbus`'s conventions into an explicit recipe the agent follows every time.

### Anatomy of a Skill

Each skill lives in its own directory under `.claude/skills/` and contains a `SKILL.md` file. The file has three parts:

Frontmatter declares the skill's name and when to use it:

```yaml
---
name: Adding a Functional Endpoint
description: Creates or modify a functional endpoint of a microservice. Use when
  explicitly asked by the user to create or modify a functional or RPC endpoint
  of a microservice.
---
```

Workflow checklist gives the agent a copy-paste tracking list so it can mark off steps as it goes:

```
Creating or modifying a functional endpoint:
- [ ] Step 1: Read local AGENTS.md file
- [ ] Step 2: Determine signature
- [ ] Step 3: Extend the ToDo interface
...
```

Detailed steps expand each item in the checklist with precise instructions, constraints and code templates the agent copies verbatim. Templates include all necessary imports, struct definitions, function signatures and test patterns.

### How Skills Guide the Agent

Skills enforce several principles that keep agent output reliable:

**Self-contained templates** - Most skills explicitly state that they are self-contained and that an agent should not explore or analyze existing microservices before starting its work. This prevents the agent from looking at other microservices and mimicking patterns that may be outdated or non-standard.

**Marker comments** - Skills instruct the agent to include `MARKER` comments throughout the generated code. These comments act as waypoints for future edits. When a feature needs to be modified or removed, the agent locates the relevant code by scanning for its marker rather than guessing.

```go
if example { // MARKER: FeatureName
	// ...
}
```

**Comprehensive coverage** - A single skill touches every file that needs updating: implementation, API client stubs, mocks, tests, OpenAPI descriptors, the manifest and documentation. Nothing is left for the developer to wire up manually.

**Incremental development** - Each skill adds or modifies a single feature without disrupting existing code. Tests are written or updated in the same pass, and the manifest is kept in sync.

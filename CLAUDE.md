This repository is the **Microbus framework** itself - the foundation that downstream application projects import and build on. You are working on framework internals, not on an application built with the framework.

## Two kinds of code in this repo

1. **Framework packages** - `connector/`, `service/`, `application/`, `transport/`, `frame/`, `httpx/`, `pub/`, `sub/`, `cfg/`, `env/`, `lru/`, `dlru/`, `mem/`, `openapi/`, `trc/`, `utils/`, `setup/`, `workflow/`. These are library code consumed by every Microbus application. Changes here affect all downstream projects.

2. **Microservices built with the framework** - `coreservices/` and `examples/` contain microservices that follow the same conventions as any downstream application. When working in these directories, follow the patterns and skills described in `.claude/rules/microbus.md`.

## Working on framework packages

- **Public API surface matters.** Exported types, functions, and interfaces in framework packages are consumed by downstream projects. Avoid breaking changes to exported signatures.
- **`connector/`** is the backbone - it implements the messaging bus, subscriptions, lifecycle, and configuration machinery that every microservice relies on.
- **`service/`** builds on `connector/` to provide the higher-level `Service` base type with convenience methods (`LogInfo`, `DistribCache`, `Now`, etc.).
- **`application/`** handles microservice orchestration, startup/shutdown sequencing, and the test harness (`RunInTest`).
- **Tests** for framework packages are standard Go unit tests (`go test ./connector/...`), not the microservice integration test pattern used in `coreservices/` and `examples/`.

## Working on coreservices/ and examples/

These directories contain microservices built using the framework. Treat them the same way you would treat microservices in a downstream application project:

- Follow all conventions in `.claude/rules/microbus.md`
- Use the skills in `.claude/skills/` for scaffolding and adding features
- Respect `MARKER` and `HINT` comments
- Update `manifest.yaml` when changing microservice code

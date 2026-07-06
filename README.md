<a href="https://www.microbus.io"><img src="./microbus-logo.svg" height="100" alt="Microbus.io logo"></a>

[![License Apache 2](https://img.shields.io/badge/License-Apache2-blue.svg)](https://www.apache.org/licenses/LICENSE-2.0)
[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![Reference](https://pkg.go.dev/badge/github.com/microbus-io/fabric)](https://pkg.go.dev/github.com/microbus-io/fabric)
[![Test](https://github.com/microbus-io/fabric/actions/workflows/test.yaml/badge.svg?branch=main&event=push)](https://github.com/microbus-io/fabric/actions/workflows/test.yaml)
[![Discord](https://img.shields.io/badge/Discord-Join-5865F2?logo=discord&logoColor=white)](https://discord.gg/FAJHnGkNqJ)

**Microbus is the only fabric where every agentic workflow runs on a true microservice substrate.** That single design choice gives your workflows security, scale, observability and prompt-driven authoring that bolt-on workflow engines cannot match.

## Documentation

Full documentation lives at **[docs.microbus.io](https://docs.microbus.io)**, including:

- [Get Started](https://docs.microbus.io/get-started/) — bootstrap a project, take the agent-guided tour, build your first workflow or microservice.
- [Agentic Workflows](https://docs.microbus.io/agentic-workflows/) — multi-step processes with branching, fan-out, human-in-the-loop interrupts, and durable state.
- [Agentic RAD](https://docs.microbus.io/agentic-rad/) — how coding agents drive Microbus development end to end.
- [Microservice Substrate](https://docs.microbus.io/microservice-substrate/) — the production-grade fabric the workflows run on.
- [Security in Depth](https://docs.microbus.io/security-in-depth/) — the layered security model.
- [Package Reference](https://docs.microbus.io/package-reference/) — every package's role and exported API.
- [Release Notes](https://docs.microbus.io/about/release-notes/) — per-release changes.

## Quick Start

You'll need a coding agent (e.g. Claude Code), [Go](https://go.dev/) 1.26+, [NATS](https://nats.io), and a SQL database for [agentic workflow](https://docs.microbus.io/agentic-workflows/) state. Then:

```sh
mkdir -p myproject
cd myproject
```

Ask your coding agent to bootstrap Microbus:

> curl the workflow at microbus.io/bootstrap and follow it

The agent walks through the bootstrap, sets up `.claude/` rules and skills, scaffolds the project, and verifies the build. See [Get Started](https://docs.microbus.io/get-started/) for the full walkthrough.

## Community

| | |
|---|---|
| Website | [www.microbus.io](https://www.microbus.io) |
| Docs | [docs.microbus.io](https://docs.microbus.io) |
| GitHub | [github.com/microbus-io](https://github.com/microbus-io) |
| Discord | [discord.gg/FAJHnGkNqJ](https://discord.gg/FAJHnGkNqJ) |
| LinkedIn | [linkedin.com/company/microbus-io](https://www.linkedin.com/company/microbus-io) |
| Reddit | [r/microbus](https://reddit.com/r/microbus) |
| YouTube | [@microbus-io](https://www.youtube.com/@microbus-io) |
| Email | info@microbus.io |

## License

Apache 2.0. See [LICENSE](./LICENSE) and [ATTRIBUTION.md](./ATTRIBUTION.md) for third-party OSS licensing.

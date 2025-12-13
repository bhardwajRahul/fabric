# `AGENTS.md`

An `AGENTS.md` file is the de-facto standard for providing coding agents with context about a project or a part thereof. `Microbus` utilizes this concept to train [coding agents](../blocks/coding-agents.md) how to work correctly on a `Microbus` solution in general, and on each individual microservice.

A global `AGENTS.md` file at the root of the project includes context that is applicable to the project as a whole. The first instruction in that file points the coding agent to `.claude/rules` where `microbus.md` is also located.

The `microbus.md` file includes instructions for working on a `Microbus` solution in general. This file may be updated with each release of `Microbus` and should not be edited by hand. Use the [code generator](../blocks/codegen.md) to refresh this file in your project with each new release of `Microbus`.

The `.claude/skills` directory is a collection of workflows that guide the coding agent when working on complex multi-step tasks. These skills are referenced in `microbus.md`.

An `AGENTS.md` file placed in the directory of each microservice keeps context that is applicable to that single microservice alone. This local `AGENTS.md` is maintained by the coding agent itself as it works on the microservice, but it can also be edited by hand.

Since Claude may not recognize `AGENTS.md`, next to each `AGENTS.md` file you'll find a simple `CLAUDE.md` file that tells Claude to read `AGENTS.md`.

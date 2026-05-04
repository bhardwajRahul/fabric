# `CLAUDE.md`

A `CLAUDE.md` file is the de-facto standard for providing coding agents with context about a project or a part thereof. Microbus utilizes this concept to train [coding agents](../blocks/coding-agents.md) how to work correctly on a Microbus solution in general, and on each individual microservice.

A global `CLAUDE.md` file at the root of the project includes context that is applicable to the project as a whole. The first instruction in that file points the coding agent to `.claude/rules` where `microbus.md` is also located.

The `.claude/rules/microbus.md` file is instructions for working on a Microbus solution in general.

The `.claude/rules/sequel.md` file extends the general instructions with SQL-specific instructions.

The `.claude/skills` directory is a collection of workflows that guide the coding agent when working on complex multi-step tasks. These skills are referenced in `microbus.md`.

A `CLAUDE.md` file placed in the directory of each microservice keeps context that is applicable to that single microservice alone. This local `CLAUDE.md` is maintained by the coding agent itself as it works on the microservice, but it can also be edited by hand.

Similarly, a `PROMPTS.md` file in located in the directory of each microservice to keep track of the prompts used to generate that microservice.

The aforementioned files are often updated with each release of Microbus and should not be edited by hand. Let your coding agent [upgrade Microbus](../blocks/coding-agents.md#upgrade-microbus) for you.

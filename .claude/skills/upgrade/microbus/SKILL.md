---
name: Upgrading the Project to the Latest Version of Microbus
description: Upgrades an existing project that is based on the Microbus framework to the latest version of Microbus. Use when explicitly asked by the user to upgrade a project to the latest version of the Microbus framework.
---

**CRITICAL**: Do NOT explore or analyze the project unless explicitly instructed to do so. The instructions in this skill are self-contained.

## Workflow

Copy this checklist and track your progress:

```
Upgrade the project to the latest Microbus framework:
- [ ] Step 1: Determine the original version
- [ ] Step 2: Get the latest Microbus package
- [ ] Step 3: Determine the latest version
- [ ] Step 4: Download latest agent rules and skills
- [ ] Step 5: Upgrade
```

#### Step 1: Determine the Original Version

Look in `go.mod` for the `github.com/microbus-io/fabric` dependency. If the dependency is not found, this is not a Microbus project. Do not proceed to the next step, exit this workflow.

Identify the version next to the `github.com/microbus-io/fabric` dependency. This is the **original version**.

#### Step 2: Get the Latest Microbus Package

Get the latest version of the Microbus package.

```shell
go get github.com/microbus-io/fabric
```

#### Step 3: Determine the Latest Version

Look in `go.mod` and identify the current version of the `github.com/microbus-io/fabric` dependency. This is the **latest version**.

If the original version is the same as the latest version, no upgrade is necessary. Do not proceed to the next step, exit this workflow.

#### Step 4: Download Latest Agent Rules and Skills

Download the latest coding agent rules and skills from Github.

```shell
git clone --depth 1 https://github.com/microbus-io/fabric temp-clone
rm -rf .claude/rules/microbus.md
rm -rf .claude/rules/sequel.md
rm -rf .claude/skills/microbus
rm -rf .claude/skills/sequel
rm -rf .claude/skills/upgrade
cp -r temp-clone/.claude .
rm -rf temp-clone
```

**CRITICAL**: After copying, reread `.claude/rules/microbus.md` and all skills referenced by this workflow. The downloaded versions may differ from what was previously loaded into context.

#### Step 5: Upgrade

Scan `.claude/skills/upgrade/` and identify all skill files named `vX.Y.Z`. The name of the skill file is the skill's version. Sort the identified skills by version from earliest to latest. Follow each skill whose version is later than the **original version**, in order.

For example, if upgrading from original version `v1.21.0`, follow skill `v1.22.0` first, then skill `v1.23.0`, and so on. Skip any upgrade skill whose version is equal to or earlier than the original version.

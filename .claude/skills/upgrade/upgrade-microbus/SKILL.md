---
name: upgrade-microbus
description: TRIGGER when user asks to upgrade the project to the latest version of Microbus, or to update the framework. Orchestrates version-specific upgrade skills in sequence.
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

Scan `.claude/skills/upgrade/` and identify all skill directories named `upgrade-vX-Y-Z`. The hyphens in the directory name correspond to dots in the semver - e.g. `upgrade-v1-27-0` is the skill for version `v1.27.0`. Sort the identified skills by version from earliest to latest. Follow each skill whose version is later than the **original version**, in order.

For example, if upgrading from original version `v1.21.0`, follow skill `upgrade-v1-22-0` first, then `upgrade-v1-23-0`, and so on. Skip any upgrade skill whose version is equal to or earlier than the original version.

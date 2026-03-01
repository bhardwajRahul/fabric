---
name: Upgrading the Project to the Latest Version of Microbus
description: Upgrades an existing Microbus project to use the latest version of the Microbus framework. Use when explicitly asked by the user to upgrade a project to the latest version of the Microbus framework.
---

**CRITICAL**: Do NOT explore or analyze the project unless explicitly instructed to do so. The instructions in this skill are self-contained.

## Workflow

Copy this checklist and track your progress:

```
Upgrade the project to the latest Microbus framework:
- [ ] Step 1: Determine if Microbus project
- [ ] Step 2: Download agent rules and skills
- [ ] Step 3: Upgrade the project
```

#### Step 1: Determine if Microbus Project

Look in `go.mod` for the `github.com/microbus-io/fabric` dependency. If the dependency is not found, this is not a `Microbus` project. Do not proceed to the next step, exit this workflow.

#### Step 2: Download Agent Rules and Skills

Download the latest coding agent rules and skills from Github.

```shell
git clone --depth 1 https://github.com/microbus-io/fabric temp-clone
rm -rf .claude/rules/microbus.md
rm -rf .claude/rules/sequel.md
rm -rf .claude/skills/microbus
rm -rf .claude/skills/sequel
cp -r temp-clone/.claude .
rm -rf temp-clone
```

**CRITICAL**: After copying, reread `.claude/rules/microbus.md` and all skill files referenced by this workflow. The downloaded versions may differ from what was previously loaded into context.

#### Step 3: Upgrade the Project

Follow the skill in `microbus/upgrade-microbus` to upgrade the `Microbus` framework, skipping the step to download the latest agent rules and skills.

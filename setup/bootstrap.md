---
name: Bootstrapping a Microbus Project
description: Bootstraps a project to use the Microbus framework.
---

**CRITICAL**: Do NOT explore or analyze the project unless explicitly instructed to do so. The instructions in this skill are self-contained.

## Workflow

Copy this checklist and track your progress:

```
Bootstrap a project to use Microbus:
- [ ] Step 1: Confirm directory
- [ ] Step 2: Check if already bootstrapped
- [ ] Step 3: Init the module
- [ ] Step 4: Get the latest version of Microbus
- [ ] Step 5: Download agent rules and skills
- [ ] Step 6: Init the project
- [ ] Step 7: Tidy up
```

#### Step 1: Confirm Directory

Confirm with the user in what directory they would like to bootstrap Microbus. Suggest the current working directory by default. Use the chosen directory as the project directory for future steps.

#### Step 2: Check if Already Bootstrapped

If `go.mod` exists in the project directory and includes a reference to `github.com/microbus-io/fabric`, the project is already bootstrapped for Microbus and no action is required. Exit this workflow.

#### Step 3: Init the Module

If `go.mod` already exists, skip this step.

Ask the user for the name of the module to set for this project. Then use it as the input to `go mod init`.

```shell
go mod init github.com/mycompany/myproject
```

#### Step 4: Get the Latest Microbus Package

Get the latest version of the Microbus package.

```shell
go get github.com/microbus-io/fabric
```

#### Step 5: Download Agent Rules and Skills

Copy the coding agent rules and skills from the pinned Microbus version already in the Go module cache (from Step 4). This is version-matched to `go.mod` and needs no network access. The module cache is read-only, so make the copies writable.

```shell
src="$(go list -m -f '{{.Dir}}' github.com/microbus-io/fabric)"
mkdir -p .claude
cp -R "$src/.claude/rules" "$src/.claude/skills" .claude/
chmod -R u+w .claude/rules .claude/skills
```

#### Step 6: Init the Project

Follow the `init-project` skill to scaffold the project structure, verify the build, and offer the tour.

#### Step 7: Tidy up

Tidy up the module if needed.

```shell
go mod tidy
```

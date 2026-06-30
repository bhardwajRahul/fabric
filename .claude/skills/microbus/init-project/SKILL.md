---
name: init-project
description: TRIGGER when user asks to initialize, set up, or bootstrap a Microbus project. Scaffolds main/main.go, config files, agent files, .gitignore, VS Code launch config, Claude Code permissions, and the token-signing key.
---

**CRITICAL**: Do NOT explore or analyze the project unless explicitly instructed to do so. The instructions in this skill are self-contained.

## Workflow

Copy this checklist and track your progress:

```
Initialize a project to use Microbus:
- [ ] Step 1: Check if a Microbus project
- [ ] Step 2: Download agent rules and skills
- [ ] Step 3: Scaffold the project
- [ ] Step 4: Verify the build
- [ ] Step 5: Offer the tour
```

#### Step 1: Check if a Microbus Project

If `go.mod` does not exist in the project directory, this is not a Go project. Exit this workflow.

If `go.mod` does not include a reference to `github.com/microbus-io/fabric`, this is not a Microbus project. Exit this workflow.

Note whether `main/main.go` already exists. If it does not, this is a **new install** - remember this for Step 5.

#### Step 2: Download Agent Rules and Skills

Copy the agent rules and skills from the pinned Microbus version already in the Go module cache. This is version-matched to `go.mod` and needs no network access. The module cache is read-only, so make the copies writable.

```shell
src="$(go list -m -f '{{.Dir}}' github.com/microbus-io/fabric)"
mkdir -p .claude
cp -R "$src/.claude/rules" "$src/.claude/skills" .claude/
chmod -R u+w .claude/rules .claude/skills
```

#### Step 3: Scaffold the Project

Run `geninit` to scaffold the project. It is idempotent: it creates `main/main.go`, `main/env.yaml`, `CLAUDE.md`, `config.yaml`, `config.local.yaml`, `env.yaml`, `env.local.yaml`, `.gitignore`, `.vscode/launch.json`, and `.claude/settings.json`, and generates a fresh Ed25519 token-signing key for `bearer.token.core` in `config.local.yaml`. Existing files are left intact.

```shell
go run github.com/microbus-io/fabric/cmd/geninit
```

`.claude/settings.json` auto-approves the Bash commands invoked by the `housekeeping` and `regenerate-boilerplate` skills, so an agent following them is not prompted for each invocation. The file is project-shared and should be checked into git.

#### Step 4: Verify the Build

Confirm the scaffolded project compiles.

```shell
go vet ./main/...
```

#### Step 5: Offer the Tour

If this is a **new install** (from Step 1), tell the user the project is ready and offer to take the agent-guided tour, which runs the bundled example microservices. If they accept, follow the `take-tour` skill. If this is not a new install, skip this step.

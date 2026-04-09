---
name: init-project
description: TRIGGER when user asks to initialize, set up, or bootstrap a Microbus project. Creates main/main.go, config files, agent files, .gitignore, VS Code launch config, and authentication scaffolding.
---

**CRITICAL**: Do NOT explore or analyze the project unless explicitly instructed to do so. The instructions in this skill are self-contained.

## Workflow

Copy this checklist and track your progress:

```
Initialize a project to use Microbus:
- [ ] Step 1: Check if a Microbus project
- [ ] Step 2: Prepare main package
- [ ] Step 3: Prepare agent files
- [ ] Step 4: Prepare config files
- [ ] Step 5: Prepare env files
- [ ] Step 6: Prepare git ignore
- [ ] Step 7: Prepare VS Code launch
- [ ] Step 8: Prepare authentication
```

#### Step 1: Check if a Microbus Project

If `go.mod` does not exist in the project directory, this is not a Go project. Exit this workflow.

If `go.mod` does not include a reference to `github.com/microbus-io/fabric`, this is not a Microbus project. Exit this workflow.

#### Step 2: Prepare Main Package

Create the `main` directory in the root of the project if one does not exist.

```shell
mkdir -p main
```

Create `main/main.go` the content of the template `main.go` located in the directory of this skill..
If the file already exists, do not update it.

Create `main/env.yaml` with the following verbatim.
If the file already exists, prepend the content to the existing file unless already there.

```yaml
MICROBUS_DEPLOYMENT: LOCAL
```

#### Step 3: Prepare Agent Files

Create `CLAUDE.md` at the root of the project with the following content.
If the file already exists, prepend the content to the existing file unless it is already there.

```md
**CRITICAL**: Read `AGENTS.md` immediately.
```

Create `AGENTS.md` at the root of the project with the following content.
If the file already exists, prepend the content to the existing file unless it is already there.

```md
**CRITICAL**: This project uses the Microbus framework. Read all `.md` files in `.claude/rules/` before starting any task.
```

#### Step 4: Prepare Config Files

Create `config.yaml` at the root of the project with the following content verbatim.
If the file already exists, do not update it.

```yaml
all:
  Example: value

myservice.hostname:
  Example: value
```

Create `config.local.yaml` at the root of the project with the following content verbatim.
If the file already exists, do not update it.

```yaml
all:
  ExampleSecret: secret value

myservice.hostname:
  ExampleSecret: secret value
```

#### Step 5: Prepare Env Files

Create `env.yaml` and `env.local.yaml` at the root of the project the content of the template `env.yaml` located in the directory of this skill. If the files already exist, do not overwrite them.

#### Step 6: Prepare Git Ignore

Create `.gitignore` at the root of the project with the following content. If the file already exists, append the content to the existing file unless already there.

```gitignore
# Microbus
*.local.*
/main/main
/main/__debug_bin*
.DS_Store
```

#### Step 7: Prepare VS Code Launch

Create `.vscode/launch.json` relative to the root of the project with the following content. If the file already exists, add the `Main` configuration to the existing file instead unless already there.

```json
{
	// Use IntelliSense to learn about possible attributes.
	// Hover to view descriptions of existing attributes.
	// For more information, visit: https://go.microsoft.com/fwlink/?linkid=830387
	"version": "0.2.0",
	"configurations": [
		{
			"name": "Main",
			"type": "go",
			"request": "launch",
			"mode": "auto",
			"program": "${workspaceFolder}/main",
			"cwd": "${workspaceFolder}/main"
		},
	]
}
```

#### Step 8: Prepare Authentication

**IMPORTANT**: Read `.claude/rules/auth.txt` for authentication conventions before proceeding with this step.

Create the `act` directory in the root of the project if one does not exist.

```shell
mkdir -p act
```

Create `act/actor.go` with the content of the template `actor.go` located in the directory of this skill. If the file already exists, do not overwrite it.

Generate an Ed25519 key and set it in `config.local.yaml` for the `PrivateKey` config of the `bearer.token.core` microservice.

```shell
openssl genpkey -algorithm Ed25519 -out private.pem
```

```yaml
bearer.token.core:
  PrivateKey: MC4CAQAwBQYDK2VwBCIEILioh4C097ydAtppNWBMxO1hkewbzzmbGs1z7n9+OHnp
```

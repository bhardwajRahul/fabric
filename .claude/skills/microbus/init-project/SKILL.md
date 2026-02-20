---
name: Initialize a Microbus Project
description: Sets up a project with the latest the Microbus framework. Use when explicitly asked by the user to initialize a project to use the Microbus framework, or to upgrade an existing Microbus project to the latest version of the framework.
---

**CRITICAL**: Do NOT explore or analyze other microservices unless explicitly instructed to do so. The instructions in this skill are self-contained to this project.

## Workflow

Copy this checklist and track your progress:

```
Initialize a project to use Microbus:
- [ ] Step 1: Download latest agent rules and skills
- [ ] Step 2: Determine old version of Microbus
- [ ] Step 3: Get the latest version of Microbus
- [ ] Step 4: Prepare main
- [ ] Step 5: Prepare agent files
- [ ] Step 6: Prepare config files
- [ ] Step 7: Prepare env files
- [ ] Step 8: Prepare .gitignore
- [ ] Step 9: Prepare launch.json
- [ ] Step 10: Upgrade
- [ ] Step 11: Tidy up
```

#### Step 1: Download Latest Agent Rules and Skills

Download the latest coding agent rules and skills from Github.

```shell
git clone --depth 1 https://github.com/microbus-io/fabric temp-clone
rm -rf .claude/rules/microbus.md
rm -rf .claude/rules/skills/microbus
cp -r temp-clone/.claude .
rm -rf temp-clone
```

Read all markdown files in `./claude/rules/` to familiarize with the `Microbus` framework.

#### Step 2: Determine Old Version of Microbus

Look in `go.mod` and identify the current version of the `github.com/microbus-io/fabric` dependency. This is the "old framework version". If the dependency is not found, skip this step.

#### Step 3: Get the Latest Version of Microbus

Get the latest version of `Microbus` and tidy up.

```shell
go get github.com/microbus-io/fabric
```

Tidy up `go.mod`. If it fails, continue to the next step. An upgrade may be necessary first.

```shell
go mod tidy
```

#### Step 4: Prepare `main`

Create the `main` directory in the root of the project if one does not exist.

```shell
mkdir -p main
```

Create `main/main.go` with the following verbatim.
If the file already exists, do not update it.

```go
package main

import (
	"fmt"
	"os"

	"github.com/microbus-io/fabric/application"
	"github.com/microbus-io/fabric/coreservices/configurator"
	"github.com/microbus-io/fabric/coreservices/httpegress"
	"github.com/microbus-io/fabric/coreservices/httpingress"
	"github.com/microbus-io/fabric/coreservices/openapiportal"
	"github.com/microbus-io/fabric/coreservices/tokenissuer"
)

/*
main runs the application.
*/
func main() {
	app := application.New()
	app.Add(
		// Configurator should start first
		configurator.NewService(),
	)
	app.Add(
		// Core microservices
		httpegress.NewService(),
		openapiportal.NewService(),
		tokenissuer.NewService(),
		// metrics.NewService(),
	)
	app.Add(
		// HINT: Add solution microservices here
	)
	app.Add(
		// When everything is ready, begin to accept external requests
		httpingress.NewService(),
		// smtpingress.NewService(),
	)
	err := app.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%+v", err)
		os.Exit(19)
	}
}
```

Create `main/env.yaml` with the following verbatim.
If the file already exists, prepend the content to the existing file unless already there.

```yaml
MICROBUS_DEPLOYMENT: LOCAL
```

#### Step 5: Prepare Agent Files

Create `CLAUDE.md` at the root of the project with the following content.
If the file already exists, prepend the content to the existing file unless already there.

```md
**CRITICAL**: Read `AGENTS.md` immediately.
```

Create `AGENTS.md` at the root of the project with the following content.
If the file already exists, prepend the content to the existing file unless already there.

```md
**CRITICAL SETUP INSTRUCTIONS**: At the start of EVERY conversation, BEFORE responding to the user's first message, you MUST:
- The directory `.claude/rules/` contains essential project-specific instructions and guidelines for this codebase. Read all `.md` files in `.claude/rules/` immediately
- This project follows a microservices architecture based on the Microbus framework. Follow all instructions and guidelines in `.claude/rules/microbus.md`
```

#### Step 6: Prepare Config Files

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

#### Step 7: Prepare Env Files

Create `env.yaml` and `env.local.yaml` at the root of the project with the following content verbatim.
If the files already exist, do not update them.

```yaml
# NATS connection settings
# MICROBUS_NATS: nats://127.0.0.1:4222
# MICROBUS_NATS_USER:
# MICROBUS_NATS_PASSWORD:
# MICROBUS_NATS_TOKEN:
# MICROBUS_NATS_USER_JWT:
# MICROBUS_NATS_NKEY_SEED:

# The deployment impacts certain aspects of the framework such as the log format and verbosity
#   PROD - production deployments
#   LAB - fully-functional non-production deployments such as dev integration, testing, staging, etc.
#   LOCAL - developing locally
#   TESTING - unit and integration testing
# MICROBUS_DEPLOYMENT: LOCAL

# The plane of communication isolates communication among a group of microservices
# MICROBUS_PLANE: microbus

# The geographic locality of the application
# MICROBUS_LOCALITY: west.us

# Enable logging of debug-level messages
# MICROBUS_LOG_DEBUG: 1

# OpenTelemetry
# https://opentelemetry.io/docs/specs/otel/protocol/exporter/
# https://opentelemetry.io/docs/specs/otel/configuration/sdk-environment-variables/
# OTEL_EXPORTER_OTLP_PROTOCOL: grpc
# OTEL_EXPORTER_OTLP_ENDPOINT: http://127.0.0.1:4317
# OTEL_EXPORTER_OTLP_TRACES_ENDPOINT:
# OTEL_EXPORTER_OTLP_METRICS_ENDPOINT:

# OTEL_METRIC_EXPORT_INTERVAL: 60000

# Enable metric collection to enable Prometheus polling
# MICROBUS_PROMETHEUS_EXPORTER: 1
```

#### Step 8: Prepare `.gitignore`

Create `.gitignore` at the root of the project with the following content.
If the file already exists, append the content to the existing file unless already there.

```gitignore
# Microbus
*.local.*
/main/main
/main/__debug_bin*
.DS_Store
```

#### Step 9: Prepare `launch.json`

Create `.vscode/launch.json` relative to the root of the project with the following content.
If the file already exists, add the `Main` configuration to the existing file instead unless already there.

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

#### Step 10: Upgrade

Skip this step if the old framework version could not be determined. Otherwise, perform the following sequentially in order:

- If the old framework version is earlier than `1.22.0`, follow the `microbus/upgrade-project-v1.22.0` skill. 
- If the old framework version is earlier than `1.23.0`, follow the `microbus/upgrade-project-v1.23.0` skill. 

#### Step 11: Tidy Up

Tidy up `go.mod`.

```shell
go mod tidy
```

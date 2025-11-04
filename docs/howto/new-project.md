# Bootstrapping a New Project

Follow these steps to create a new project based on the `Microbus` framework.

#### Step 1: Init the Go Project

Make a directory to hold your projects files.

```shell
mkdir mysolution
```

Init the Go project with the name of the package of your project, for example `github.com/mycompany/mysolution`.

```shell
cd mysolution
go mod init github.com/mycompany/mysolution
```

#### Step 2: Code Generate the Project Structure

Add `Microbus`'s code generator to `go.mod` using:

```shell
go get github.com/microbus-io/fabric/codegen
```

Create `doc.go` in the root of the project next to `go.mod`.

```go
package root

//go:generate go run github.com/microbus-io/fabric/codegen
```

Use the code generator to create the project structure.

```shell
go generate
```

```
mysolution/
├── .claude/                     # Claude Code setup
│   └── skills/				     # Claude Code skills
├── .vscode/
│   └── launch.json				 # VSCode launch file
├── main/
│   ├── config.yaml              # Configuration file
│   ├── env.yaml                 # Environment settings
│   └── main.go                  # Main application
├── AGENTS-MICROBUS.md           # Instructions to coding agents for Microbus
├── AGENTS.md                    # Instructions to coding agents for this project
└── CLAUDE.md                    # Refer Claude to AGENTS.md
```

Fetch the dependencies.

```shell
go mod tidy
```

#### Step 3: Create Microservices

[Create a microservice](../howto/create-microservice.md), rinse and repeat.

# Project Bootstrapping

The code generator facilitates the creation of a new `Microbus` project.

Initialize the project:

```shell
cd mysolution
go mod init github.com/mycompany/mysolution
go get github.com/microbus-io/fabric/codegen
```

Create `doc.go` in the root of the project next to `go.mod`:

```go
package root

//go:generate go run github.com/microbus-io/fabric/codegen
```

Use `go generate` to create the initial project structure:

```shell
go generate
```

Your project structure will now look like this:

```
mysolution/
├── .claude/                     # Claude setup
│   └── skills/
├── .vscode/
│   └── launch.json				 # VSCode launch file
├── main/
│   ├── config.yaml              # Configuration file
│   ├── env.yaml                 # Environment settings
│   └── main.go                  # Main application
├── AGENTS-MICROBUS.md           # Instructions to coding agents for Microbus
├── AGENTS.md                    # Instructions to coding agents for this project
├── CLAUDE.md                    # Refer Claude to AGENTS.md
├── doc.go
├── go.mod
└── go.sum
```

Refresh the dependencies:

```shell
go mod tidy
```

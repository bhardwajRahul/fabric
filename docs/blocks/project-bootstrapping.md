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
├── .claude/                    # Claude setup
│   ├── rules/
│   └── skills/
├── .vscode/
│   └── launch.json             # VSCode launch file
├── main/
│   ├── config.yaml             # Configuration file
│   ├── env.yaml                # Environment settings
│   └── main.go                 # Main application
├── .gitignore                  # git ignore
├── AGENTS.md                   # Instructions for coding agents
├── CLAUDE.md                   # Instructions for Claude
├── config.yaml                 # Configuration properties
├── config.local.yaml           # git ignored configuration properties
├── doc.go
├── env.yaml                    # Environment variables
├── env.local.yaml              # git ignored environment variables
├── go.mod
└── go.sum
```

Refresh the dependencies:

```shell
go mod tidy
```

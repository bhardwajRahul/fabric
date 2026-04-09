# Package `coreservices/shell`

The `shell.core` microservice runs shell commands on the host where it is deployed. It is intended for agentic workflows and admin automation that need to drive local tooling (Git, build systems, file utilities, etc.) from the bus.

The single `Execute` functional endpoint runs a command and returns the exit code, captured standard output, and captured standard error:

```go
exitCode, stdout, stderr, err := shellapi.NewClient(svc).Execute(
	ctx,
	"git status --porcelain", // cmd
	"/path/to/repo",          // workDir
	"",                       // stdin
	nil,                      // envars
)
```

`Execute` listens on internal Microbus [port](../tech/ports.md) `:444`, so it is not reachable from outside the bus through the [HTTP ingress proxy](../structure/coreservices-httpingress.md). Cross-platform behavior matches the host: on macOS and Linux the command runs through the user's shell; on Windows it runs through `cmd.exe`.

Because shell execution is inherently sensitive, treat any deployment of `shell.core` as a privileged surface. Restrict access through `RequiredClaims` on the caller's path or by gating the service behind authenticated, authorized callers only.

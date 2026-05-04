## Purpose

Shell is a core microservice that enables running shell commands on a host machine. It is intended for internal use by other microservices (e.g. agentic workflows that need to execute system commands) and is not exposed externally.

## Design Decisions

- **Single `Execute` endpoint** rather than multiple variants of different complexity. Callers pass zero values for optional arguments (`workDir`, `stdin`, `envars`), and `omitzero` keeps the JSON payload clean.
- **Port `:444`** (internal-only) to prevent external access through the ingress proxy. This service should never be exposed to untrusted callers.
- **Cross-platform**: uses `sh -c` on Unix and `cmd /C` on Windows, selected at runtime via `runtime.GOOS`.
- **`workDir` defaults to the process's current directory** when empty, rather than leaving it unset (which would also default to cwd, but this makes the behavior explicit).
- **Commented out in `main/main.go`** by default. Must be explicitly uncommented to include in a deployment.

## Testing

Tests are split into `TestShell_ExecuteUnix` and `TestShell_ExecuteWindows`, each using native shell commands for their platform. The non-matching test is skipped via `t.Skip`.


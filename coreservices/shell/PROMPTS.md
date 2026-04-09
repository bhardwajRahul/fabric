## Create shell core microservice

Create a core microservice named "shell" with hostname "shell.core" to enable running shell commands on a host. Add a functional endpoint `Execute(cmd string, envars map[string]string) (exitCode int, stdOut string, stdErr string)` that executes a shell command with optional environment variables and returns the exit code, standard output, and standard error.

## Add workDir and stdin arguments

Add `workDir` and `stdin` input arguments to `Execute`, in the order `cmd, workDir, stdin, envars`. Rename the return arguments from `stdOut`/`stdErr` to `stdout`/`stderr` (all lowercase). Keep a single `Execute` function rather than splitting into multiple versions of varying complexity.

## Route and workDir defaults

Change the route to `:444/execute` so the endpoint is internal-only and not accessible from outside the bus. Default `workDir` to the current working directory when empty. Comment out the service in `main/main.go` (it's in the core services section but disabled by default).

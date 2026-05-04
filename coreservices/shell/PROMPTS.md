## Shell Core Service

Create a core microservice at hostname `shell.core` that enables running shell commands on the host machine from within the bus. Comment out the service in `main/main.go` by default (opt-in only).

Expose one endpoint:

- `Execute(cmd string, workDir string, stdin string, envars map[string]string) (exitCode int, stdout string, stderr string)` on `POST :444/execute`

Port `:444` is internal-only and inaccessible from outside the bus via the ingress proxy.

### Execute Logic

1. Return `400 Bad Request` if `cmd` is empty.
2. Select the shell: `cmd /C` on Windows, `sh -c` on all other platforms (detected via `runtime.GOOS`).
3. Call `configureProcessGroup(c)` (platform-specific, in `process_unix.go` / `process_windows.go`) to configure the process group for clean termination.
4. Default `workDir` to `os.Getwd()` when empty.
5. If `stdin` is non-empty, set `c.Stdin = strings.NewReader(stdin)`.
6. If `envars` is non-empty, set `c.Env = os.Environ()` and append `key=value` pairs, inheriting the process environment.
7. Capture stdout and stderr into separate `cappedBuffer` instances (see below).
8. Run via `c.Run()`. Distinguish exit-code errors (`exec.ExitError`) from execution errors (return the latter as a real error). If the context was cancelled, log a warning.
9. Log the command (truncated to 1024 chars), working directory, exit code, and byte counts regardless of success or failure.

### cappedBuffer

Implement a bounded `io.Writer` that retains the first `headCap` bytes and the last `tailCap` bytes written to it (each half of `MaxOutputBytes`). Bytes in the middle are discarded but counted. `String()` renders `head + "\n... [truncated N bytes] ...\n" + tail` when truncation occurred; otherwise returns the full content. Uses a circular ring buffer for the tail portion.

### Config

- `MaxOutputBytes` — maximum bytes retained from each of stdout and stderr, default `262144` (256 KiB), minimum `1024`.

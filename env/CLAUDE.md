## Design Rationale

### Dotenv-style: yaml writes through to the real OS env

The package walks ancestor directories from `os.Getwd()`, merges `env.yaml` and `env.local.yaml` files in priority order (subdirectory over ancestor; `.local` over non-local), and calls `os.Setenv` for the resulting key/value pairs at package init time. This matches the convention of dotenv (Node), python-dotenv, godotenv, direnv, Foreman, Rails dotenv, and `docker compose .env`: the file's role is to populate the *real* OS environment, not to maintain a parallel sandbox.

The earlier model held loaded values in an in-memory shadow store and only `env.Get` consulted them; `os.Getenv`-direct callers (third-party libraries like the OTel SDK, gRPC config readers, AWS SDK env-resolution paths) never saw yaml values. That forced framework code to write per-variable pass-through plumbing for any yaml-tunable knob a third-party library exposed. Switching to write-through eliminated that class of plumbing.

### YAML format is kept (not switched to `.env`)

Format is independent of semantics. The package adopts dotenv *semantics* (write to OS env) but keeps the YAML *format* for visual consistency with `config.yaml` — both project-local config files use the same parser, the same `.local.yaml` override convention, and the same operator mental model. Switching to flat `KEY=VALUE` text would have matched the rest of the dotenv ecosystem more literally but at the cost of fragmenting the project's config-file conventions. The first-line comment block in env.yaml communicates intent ("environment variables") well enough.

The yaml schema is deliberately flat — top-level keys map to env var names. There is no nested section (`os:`, `internal:`, etc.). Every entry mirrors to OS env. Anything that's not a flat string scalar is ignored.

### Real OS env wins over yaml (dotenv convention)

`Load` calls `os.Setenv(k, v)` *only when `k` is not already present in the OS env*. This matches the default behavior of every major dotenv implementation (godotenv.Load, python-dotenv default, Node dotenv default — none override an existing OS env entry). Operators who set values via systemd / k8s / docker / shell get those values; yaml is a fallback for unset keys. This is a behavioral change from the legacy "file overrides OS" semantics that the previous in-memory model accidentally implemented.

If you genuinely need yaml to override OS, set the OS env var before binary launch to the value you want, or remove it from the environment entirely so yaml fills it. There is intentionally no `Overload` mode — the asymmetry simplifies reasoning ("OS always wins over the file") and aligns with what new contributors expect.

### `Push`/`Pop` mutates OS env with save-and-restore

`Push(key, value)` captures the current OS env state for `key` (`os.LookupEnv` returns both value and presence flag), pushes that state onto an in-memory stack keyed by name, then `os.Setenv`s the new value. `Pop` reverses the process: pops the stack, then either `os.Setenv`s the saved value back, or `os.Unsetenv`s if the prior state was unset.

This is the central mechanism that makes Microbus tests able to control third-party SDK behavior. Because Push writes through to the real OS env, libraries that read env vars directly (OTel SDK, gRPC, etc.) see the test's overrides. The previous in-memory-only Push silently failed for any library that bypassed `env.Get`.

### Push/Pop is goroutine-safe but the global env is not

The mutex serializes the push/pop bookkeeping, but `os.Setenv` itself manipulates a process-global state shared with everything else in the binary. Tests that mutate env via Push/Pop must not run with `t.Parallel()`. The package documentation reflects this; the lack of cross-test isolation is inherent to the OS env model and not something the package can paper over.

A forgotten `Pop` leaks into other tests in the same process. The convention in test code is `defer env.Pop(key)` immediately after each `env.Push(key, value)`. The same discipline existed under the in-memory model; switching to OS-backed didn't change the rule, only widened the impact of a missed Pop (it now leaks to anything reading `os.Getenv`, not just `env.Get` callers).

### `Get` and `Lookup` are thin wrappers

Post-migration, `env.Get` is `os.Getenv` and `env.Lookup` is `os.LookupEnv`. They're kept rather than removed for source compatibility with downstream Microbus projects that have already adopted them. New code can use either form interchangeably; existing code keeps working without churn.

### `init()`-time loading is intentional

The `Load` call at `init()` is the standard dotenv pattern — values are present in the OS env before any other code runs. This matters because:

- **Construction-time env reads** in third-party SDKs (e.g. `otlptracegrpc.New(ctx)`) only see env vars that exist when they're called. With load-on-init, by the time any constructor runs, yaml has already populated.
- **`os.Environ()` snapshots** taken by anything (subprocess `exec.Command` env inheritance, telemetry resource attribute capture, etc.) include yaml values automatically.

A user who needs yaml reload after process start can call `Load` again. In normal use the init-time invocation is sufficient.

### Subprocess inheritance is a feature *and* a footgun

Because yaml values land in the real OS env, anything that does `exec.Command(...)` inherits them by default (Go's `exec` populates `cmd.Env` from `os.Environ()` when `cmd.Env` is nil). This is the correct behavior for most cases — a child process that needs OTel tuning, AWS credentials, etc. should see the same env as its parent.

Where this is a footgun: a service that exec's *user-supplied* commands (notably `coreservices/shell`) shouldn't blindly leak the parent's env to arbitrary child processes. Such services must explicitly populate `cmd.Env` with a filtered or empty allowlist. This is a service-level concern that the framework cannot solve generically, and the pollution-via-yaml issue is no different in kind from pollution-via-real-env-var. The fix lives at the spawning site, not in the env package.

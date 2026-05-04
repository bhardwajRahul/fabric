# Package `env`

Package `env` loads environment variables from `env.yaml` and `env.local.yaml` files into the real OS environment at package init time, following dotenv conventions.

`Get(key string) string` and `Lookup(key string) (string, bool)` retrieve a variable's value. They are thin wrappers over `os.Getenv` and `os.LookupEnv`. Keys are case-sensitive.

### Dotenv-style write-through

At package init, `Load` walks ancestor directories from the current working directory up to the filesystem root, merges all `env.yaml` and `env.local.yaml` entries (subdirectory overrides ancestor; `.local` overrides non-local), and calls `os.Setenv` for each entry that is not already present in the OS environment. Real OS env vars set before the binary launches always win - this matches the convention of dotenv (Node), python-dotenv, godotenv, direnv, and `docker compose .env`.

Because yaml values land in the real OS environment, code that reads env vars directly (third-party SDKs like the OTel exporter, gRPC, AWS SDK, etc.) sees them transparently. There is no in-memory shadow store and no per-variable pass-through plumbing required to make a yaml-tunable knob visible to a third-party library.

`Load` is called automatically at `init()`. It can be called again to reload yaml changes, but in normal use the init-time invocation is sufficient.

### `Push` and `Pop` for tests

`Push(key, value string)` and `Pop(key string)` mutate the OS environment with save-and-restore. `Push` captures the current OS env state for the key (`os.LookupEnv` returns both value and presence), pushes that state onto an in-memory stack, then `os.Setenv`s the new value. `Pop` reverses it: pops the stack, then either restores the saved value or `os.Unsetenv`s if the prior state was unset.

This is the central mechanism that lets tests control third-party SDK behavior. Because `Push` writes through to the real OS env, libraries that bypass `env.Get` still see the override.

```go
env.Push("OTEL_EXPORTER_OTLP_TIMEOUT", "100ms")
defer env.Pop("OTEL_EXPORTER_OTLP_TIMEOUT")
// ... test code that expects a short OTel timeout
```

`Push` is goroutine-safe but the global OS environment it mutates is not - tests using `Push` must not run with `t.Parallel`. A forgotten `Pop` leaks into other tests in the same process, so the convention is `defer env.Pop(key)` immediately after each `env.Push(key, value)`.

### Subprocess inheritance

Anything that does `exec.Command(...)` inherits yaml-loaded values by default, because Go's `exec` populates `cmd.Env` from `os.Environ()` when `cmd.Env` is nil. This is correct for most cases (a child process that needs OTel tuning, AWS credentials, etc. should see the same env as its parent). Services that exec user-supplied commands - notably [`shell.core`](../structure/coreservices-shell.md) - must explicitly populate `cmd.Env` with a filtered allowlist rather than blindly leaking the parent's env.

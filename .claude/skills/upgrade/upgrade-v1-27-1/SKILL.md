---
name: upgrade-v1-27-1
user-invocable: false
description: Called by upgrade-microbus. Upgrades the project to v1.27.1. Splits each microservice's `*api/client.go` into `client.go` + `endpoints.go`, moving the `Hostname` constant, the `Def` type and `URL()` method, the `var (...)` block of `Def{...}` literals, and every `*In`/`*Out` struct into `endpoints.go`.
---

## Workflow

Copy this checklist and track your progress:

```
Upgrade a Microbus project to v1.27.1:
- [ ] Step 1: Find all microservices to upgrade
- [ ] Step 2: Split client.go into client.go + endpoints.go
- [ ] Step 3: Update manifests
- [ ] Step 4: Vet
```

#### Step 1: Find All Microservices to Upgrade

Find all microservice directories in the project that contain a `*api/client.go` file. Exclude files under `.claude/skills/` - those templates are maintained separately. Skip any microservice whose `*api/` directory already contains an `endpoints.go` file.

#### Step 2: Split `client.go` Into `client.go` + `endpoints.go`

Run the bundled `split_client.go` script against each `*api/client.go` identified in Step 1. The script lives next to this `SKILL.md` at `.claude/skills/upgrade/upgrade-v1-27-1/split_client.go` and edits `client.go` in place + writes a new `endpoints.go` next to it. It uses only the Go standard library, so it runs without a `go.mod`.

```shell
go run .claude/skills/upgrade/upgrade-v1-27-1/split_client.go -- path/to/myservice/myserviceapi/client.go
```

The `--` is required: without it, `go run` interprets the trailing `client.go` argument as another source file to compile.

Run the script once per microservice. It is idempotent against the layout produced by the `add-*` skills; running it twice on the same file will fail on the second run because the `Hostname` comment will already be gone.

What the script does:

- Locates the `// Hostname is the default hostname of the microservice.` comment in `client.go` and treats everything from there through the next `var (...)` block (the one with `// HINT: Insert endpoint definitions here`) as the "endpoints header".
- Collects every `*In` / `*Out` struct identified by the godoc pattern `// XxxIn are ...` / `// XxxOut are ...` followed by `type XxxIn struct` / `type XxxOut struct`.
- Writes `endpoints.go` with the package declaration, an `import` block (always `httpx`; adds `time` if any moved field uses `time.Time` / `time.Duration`; adds `workflow` if any moved field uses `workflow.Flow`), the endpoints header, and the In/Out structs in source order.
- Rewrites `client.go` with those ranges removed and runs of 3+ blank lines collapsed to 2.

If a microservice has In/Out structs that reference packages other than `time` or `workflow` (e.g. a third-party type), the script will not detect those imports and `endpoints.go` will fail to compile. Fix by adding the missing import to the top of `endpoints.go` manually, or by widening the script's import detection in the `# 5. Determine extra imports` section.

After running the script, the result for each microservice should be:

- `endpoints.go` contains: imports, `Hostname`, `Def`, `URL()`, the `var (...)` block of `Def{...}` literals (with the `// HINT: Insert endpoint definitions here` comment), and all `*In`/`*Out` structs.
- `client.go` contains: imports + import-guard block, `multicastResponse`, `Client`, `MulticastClient`, `MulticastTrigger`, `Hook`, `Executor`, `WorkflowRunner`, `marshalRequest`/`marshalPublish`/`marshalFunction`/`marshalTask`/`marshalWorkflow`, all `*Response` wrappers, and all per-feature client methods.

Do not modify `client.go`'s import block or the `var (_ context.Context; _ json.Encoder; ...)` guard block - the script leaves them alone, and `httpx` stays imported because the marshal helpers still use it.

#### Step 3: Update Manifests

Update the `frameworkVersion` in all `manifest.yaml` files in the project to `1.27.1`. Update each manifest's `modifiedAt` to the current UTC timestamp in RFC 3339 format.

#### Step 4: Vet

Run `go vet ./main/...` on the project. Fix any compilation errors caused by missed imports or duplicate declarations before finishing.

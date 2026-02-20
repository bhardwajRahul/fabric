---
name: Upgrade a Project to V1.22.0
description: Upgrades all microservices to v1.22.0.
---

## Workflow

Copy this checklist and track your progress:

```
Upgrade a Microbus project to v1.22.0:
- [ ] Step 1: Breaking changes (before microservices)
- [ ] Step 2: Identify microservices to upgrade
- [ ] Step 3: Upgrade in parallel
- [ ] Step 4: Breaking changes (after microservices)
```

#### Step 1: Breaking Changes (Before Microservices)

The package `github.com/microbus-io/fabric/rand` was deprecated.

- Replace calls to `rand.AlphaNum64(n)` with `utils.RandomIdentifier(n)` located in the `github.com/microbus-io/fabric/utils` package
- Similarly, replace calls to `rand.AlphaNum32(n)` with `strings.ToLower(utils.RandomIdentifier(n))`
- Replace calls to `rand.IntN` with the equivalent function in `math/rand/v2`

Some distributed caching methods were deprecated.

- Replace calls to `svc.DistribCache().LoadJSON` or `svc.DistribCache().LoadCompressedJSON` with `svc.DistribCache().Get`
- Replace calls to `svc.DistribCache().StoreJSON` or `svc.DistribCache().StoreCompressedJSON` with `svc.DistribCache().Set`

The JWT dependency was upgraded from `github.com/golang-jwt/jwt/v4` to `github.com/golang-jwt/jwt/v5`. When parsing a JWT, set the time function option to `svc.Now(ctx)` if validation is required.

```go
jwt.WithTimeFunc(func() time.Time {
	return svc.Now(ctx)
}),
```

The `Startup` and `Shutdown` methods of the `Connector` now require `ctx context.Context` argument. If a context is not available in the parent function, use `t.Context()` if in tests, or `context.Background()` otherwise.

The `Startup`, `Shutdown` and `AddAndStartup` methods of the `Application` now require `ctx context.Context` argument. If a context is not available in the parent function, use `t.Context()` if in tests, or `context.Background()` otherwise.

#### Step 2: Identify Microservices to Upgrade

Scan the project for all directories containing a `service.yaml` file. Each of these directories is a microservice that needs upgrading to v1.22.0.

#### Step 3: Upgrade in Parallel

Use the skill `.claude/skills/microbus/upgrade-microservice-v1.22.0` to upgrade each of the identified microservices. You may invoke sub agents to perform the upgrade in parallel.

#### Step 4: Breaking Changes (After Microservices)

The `initializer` passed to the `Init` method of the microservice now must return an `err error`.

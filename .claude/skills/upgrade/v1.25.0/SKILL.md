---
name: Upgrading a Project to v1.25.0
description: Upgrades the project to v1.25.0.
---

## Workflow

Copy this checklist and track your progress:

```
Upgrade a Microbus project to v1.25.0:
- [ ] Step 1: Rename bearer token config keys
- [ ] Step 2: Update manifests
```

#### Step 1: Rename Bearer Token Config Keys

In all `config.yaml` and `config.local.yaml` files in the project, rename the following configuration properties under `bearer.token.core`:

- `PrivateKeyPEM` to `PrivateKey`
- `AltPrivateKeyPEM` to `AltPrivateKey`

#### Step 2: Update Manifests

Update the `frameworkVersion` in all manifest files to `1.25.0`.

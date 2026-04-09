---
name: upgrade-v1-23-0
user-invocable: false
description: Called by upgrade-microbus. Upgrades the project and all microservices to v1.23.0.
---

## Workflow

Copy this checklist and track your progress:

```
Upgrade a Microbus project to v1.23.0:
- [ ] Step 1: Identify microservices to upgrade
- [ ] Step 2: Upgrade in parallel
- [ ] Step 3: Breaking changes
```

#### Step 1: Identify Microservices to Upgrade

Scan the project for all directories containing a `manifest.yaml` file.
If the `frameworkVersion` in `manifest.yaml` is earlier than 1.23.0, this directory identifies a microservice that requires upgrade.

#### Step 2: Upgrade in Parallel

Read and follow ALL steps of the skill `upgrade/v1.23.0-microservice` to upgrade each of the identified microservices separately. You may invoke subagents to perform the upgrade in parallel.

#### Step 3: Breaking Changes

Vet the project and correct issues.

In particular:
- Replace `myserviceapi.URLOfMyEndpoint` with `myserviceapi.MyEndpoint.URL()`

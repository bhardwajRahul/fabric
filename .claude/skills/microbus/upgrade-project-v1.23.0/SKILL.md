---
name: Upgrade a Project to V1.23.0
description: Upgrades all microservices to v1.23.0.
---

## Workflow

Copy this checklist and track your progress:

```
Upgrade a Microbus project to v1.23.0:
- [ ] Step 1: Breaking changes (before microservices)
- [ ] Step 2: Identify microservices to upgrade
- [ ] Step 3: Upgrade in parallel
- [ ] Step 4: Breaking changes (after microservices)
```

#### Step 1: Breaking Changes (Before Microservices)

This is a no op.

#### Step 2: Identify Microservices to Upgrade

Scan the project for all directories containing a `manifest.yaml` file.
If the `frameworkVersion` in `manifest.yaml` is earlier than 1.23.0, this directory identifies a microservice that requires upgrade.

#### Step 3: Upgrade in Parallel

Use the skill `microbus/upgrade-microservice-v1.23.0` to upgrade each of the identified microservices. You may invoke sub agents to perform the upgrade in parallel.

#### Step 4: Breaking Changes (After Microservices)

Vet the project and correct issues.

In particular:
- Replace `myserviceapi.URLOfMyEndpoint` with `myserviceapi.MyEndpoint.URL()`

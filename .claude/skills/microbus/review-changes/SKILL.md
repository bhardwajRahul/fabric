---
name: review-changes
user-invocable: true
description: Reviews the microservices touched by a set of changes - by default the whole current feature branch versus its merge-base with main, plus any uncommitted work. Runs the review-microservice skill on each changed microservice and the review-architecture skill scoped to those microservices and their graph neighbors, then consolidates one report. Use before merging a branch or before committing working-tree changes.
---

This skill composes the two review skills over a diff. It does not re-implement their checks - it resolves *what*
changed, then drives `review-microservice` per changed microservice and `review-architecture` scoped to the changed
set. The separation of concerns between the two is deliberate: per-microservice internals come from
`review-microservice`, cross-cutting concerns from `review-architecture`, and this skill must not duplicate either.

## Workflow

Copy this checklist and track your progress:

```
Change review:
- [ ] Step 1: Resolve the diff scope
- [ ] Step 2: Map changed files to microservices
- [ ] Step 3: Review each changed microservice
- [ ] Step 4: Architectural review scoped to the changed set
- [ ] Step 5: Consolidate the report
```

#### Step 1: Resolve the Diff Scope

Determine which files changed. The default depends on the current branch; an explicit argument overrides it.

First identify the current branch and the repository's default branch:
```shell
branch=$(git rev-parse --abbrev-ref HEAD)
git status --porcelain                          # staged, unstaged, and untracked (always part of the scope)
```
Use the repository's actual default branch if it is not `main` (check `git symbolic-ref refs/remotes/origin/HEAD`
or `git remote show origin`); substitute it for `main` below.

- **On a feature branch (not the default branch)** - the default. Review the whole branch plus uncommitted work,
  the pre-merge review:
  ```shell
  base=$(git merge-base main HEAD)
  git diff --name-only "$base" HEAD             # committed on this branch
  ```
  Union that with the working-tree list.
- **On the default branch (`main`)** - there is no feature branch to diff, so default to the changes since the last
  tagged release, plus uncommitted work:
  ```shell
  tag=$(git describe --tags --abbrev=0 2>/dev/null)
  git diff --name-only "$tag" HEAD              # only if a tag was found
  ```
  If `git describe` finds no tag (empty output / non-zero exit), fall back to reviewing the **uncommitted work
  only**.
- **`uncommitted` argument** - regardless of branch, narrow to just the working tree (`git status --porcelain`,
  including untracked files) and skip any committed-diff.

If the resolved file list is empty, report that there is nothing to review and stop.

State the mode (branch-vs-default, since-last-release, or uncommitted-only), the base commit or tag, and the
resolved file list to the user before proceeding.

#### Step 2: Map Changed Files to Microservices

For each changed file, walk up the directory tree to the nearest ancestor containing a `manifest.yaml` - that
directory is the owning microservice. Collect the distinct set of changed microservices.

Also note, separately from that set:

- Changes to `main/main.go`, `config.yaml`, `env.yaml`, or `main/topology.mmd` - these are application-composition
  changes that feed the architectural review in Step 4 even when no microservice directory changed.
- Changes to framework packages (`connector/`, `service/`, `application/`, and the other library packages listed in
  the repository's root `CLAUDE.md`). These are out of scope for this skill, which reviews microservices. List them
  so the user knows they were seen and skipped, and suggest a plain code review for them.

Present the changed-microservice set and the out-of-scope list. If the set is empty but composition files changed,
proceed to Step 4 only.

#### Step 3: Review Each Changed Microservice

For each microservice in the changed set, run the `review-microservice` skill's full workflow on that directory,
one microservice at a time, in this context. Do **not** launch subagents to review microservices in parallel unless
the user explicitly asks to run the reviews concurrently or asks for a faster review; the default is sequential and
in-context. Review the **whole** microservice, not only its changed files - a
change frequently breaks or leaves stale something elsewhere in the same directory (a new endpoint that should have
updated a shared helper, a renamed field a sibling handler still reads, a test that no longer covers the new path).

When ranking findings, put the ones that touch changed lines first; whole-service findings that predate this branch
come after, labelled as pre-existing so the reader can tell regressions from latent issues.

Keep each microservice's findings under its own heading for the consolidated report.

#### Step 4: Architectural Review Scoped to the Changed Set

Run the `review-architecture` skill, but scope its findings. Build the full system map (Step 1 of that skill is
cheap - it reads manifests), then focus the cross-cutting checks on:

- the **changed microservices**, and
- their **immediate graph neighbors** - the microservices that call them or that they call (from the `downstream`
  sections and `*api` imports). A change breaks coupling at the edges, so a neighbor one hop away is in scope; the
  rest of the system is context, not audited.

Prioritize the architectural checks most sensitive to a diff: newly introduced dependency cycles, new or removed
downstream edges and whether the manifests still match the code, new events and their pairing, boundary shifts (an
endpoint or table that moved between microservices), `main/main.go` startup-group ordering for added microservices,
and cross-service workflow wiring when a workflow or task changed. Report system-wide findings that the changed set
introduced or made worse; do not re-report unrelated latent architecture issues elsewhere in the system.

#### Step 5: Consolidate the Report

Merge the per-microservice reports from Step 3 and the scoped architectural report from Step 4 into one document.
De-duplicate: if the same underlying issue surfaced in both a per-service and the architectural pass, keep the more
specific statement once and drop the other. Order everything by severity (Critical, then Warning, then Info), and
within a severity put change-introduced findings ahead of pre-existing ones.

```markdown
# Change Review

## Summary
Scope (branch-vs-default, since-last-release, or uncommitted-only), the base commit or tag, the changed
microservices, and out-of-scope changes noted. Overall assessment and the count of findings by severity.

## Per-Microservice Findings

### {hostname}
(the review-microservice report for this microservice, change-introduced findings first)

(repeat for each changed microservice)

## Architectural Findings
(the review-architecture report, scoped to the changed set and its neighbors)

## Conclusion
Prioritized action items, change-introduced issues first.
```

Present the consolidated report to the user.

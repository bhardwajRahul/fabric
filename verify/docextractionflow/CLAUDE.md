# docextractionflow.verify

## Agent Instructions

This microservice implements agentic workflows. See `.claude/rules/workflows.txt` for the conventions.

## Purpose

Verification fixture for the heaviest pipeline shape: **doubly-nested forEach with two levels of
explicit fan-in**, plus simulated latency and a retried failure rate. It models a document
extraction pipeline:

```
ScanPDF -forEach(pageImages as page)->
  IdentifyChunks -forEach(chunks as chunk)->
    TranscribeChunk -[fan-in]-> JoinPageTranscriptions -[fan-in]-> JoinDocTranscriptions -> END
```

- `ScanPDF` (forEach source over pages): ~100ms delay, returns 5-22 pages of random image
  bytes (~50-150 KB each) and `pageCount`.
- `IdentifyChunks` (inner forEach source over chunks): 2-5 chunk rectangles per page (always
  >=1 so the inner forEach never empties).
- `TranscribeChunk`: 50-150ms simulated OCR latency; 5% failure rate retried via
  `flow.Retry(100, 500ms, 1.0, 500ms)` (constant delay, no backoff); on success contributes
  one sentence (8-20 words) as the single-element `transcriptions` delta.
- `JoinPageTranscriptions` (`SetFanIn`, inner): joins a page's chunk transcriptions and
  contributes the page text as the single-element `pageTexts` delta.
- `JoinDocTranscriptions` (`SetFanIn`, outer): joins all page texts into `docTranscription`,
  one page per line.

## Patterns exercised

- Doubly-nested dynamic fan-out (`forEach` within `forEach`)
- Two stacked explicit fan-ins (`SetFanIn` on the inner and outer join)
- Explicit `SetReducer("transcriptions", ReducerAppend)` and `SetReducer("pageTexts", ReducerAppend)`
  at the two fan-in levels, with distinct field names per level so reducers do not cross levels
- Deterministic fan-in ordering via `fan_out_ordinal` (page order, chunk order) despite the
  random per-chunk latency that scrambles completion order
- Task-initiated bounded retry (constant delay, no backoff) under a real failure rate
- A non-trivial `Rectangle` complex type flowing through state

## Deterministic assertions

Random page/chunk counts and transcription text make exact-value assertions impossible, so the
test asserts structural invariants:

- final status is `completed`
- `docTranscription` is non-empty
- it has between 5 and 22 lines, i.e. one line per page over the source range (no page dropped or duplicated by the
  nested fan-in)
- every line is non-empty (every page produced >=1 chunk transcription)

The `pageCount` output is deliberately **not** asserted. The outer `forEach` uses alias `page`, so the foreman injects
and later strips a `<as>Count` bookkeeping field named `pageCount` (the cohort size) - the same name as `ScanPDF`'s
`pageCount` output. The fan-in strip removes it by name, so `pageCount` is cleared by the time the flow completes. The
line count carries the page-count invariant instead. (This is the forEach `<as>Index`/`<as>Count` naming collision; an
author who needs a surviving page count must name the output something other than `<alias>Count`.)

## Runtime note

The simulated latencies are deliberately small (ScanPDF ~100ms, TranscribeChunk
50-150ms/chunk). They exist only to scramble task-completion order so the deterministic
`fan_out_ordinal` merge is actually exercised - the property under test needs *variance* in
completion order, not *seconds* of it. Keeping them small decouples wall time from worker
count and per-core budget: the whole flow finishes in a few seconds at 4 workers even on a
contended or `GOMAXPROCS=2` runner, so the timeout/time-budget flake class is gone at the
root rather than papered over with worker tuning.

Historical note: an earlier revision used 1s + 2-5s sleeps with 64 foreman workers. That
burst of long, concurrent task dispatches to the single replica oversubscribed a CPU-starved
runner badly enough that a `transcribe-chunk` call exceeded its per-task time budget (HTTP
408) or ack window (404); dropping to 4 workers then made a single run exceed the harness
`-timeout`. Shrinking the sleeps removed the underlying cause. The test is also **not**
`t.Parallel()` so it doesn't add to cross-package CPU contention, and the per-chunk sleeps
use `svc.Sleep(ctx, d)` so a cancelled task aborts immediately instead of finishing its
sleep and burning more CPU after cancellation.

The synchronous `foremanapi.Client.Run`/`Await` blocks on a single request whose context
bounds `Await`. The test builds the foreman client with `pub.Timeout(5 * time.Minute)` so
that request is not the limiting factor - cheap insurance, not a foreman bug.
`TranscribeChunk`'s retry uses a constant 500ms / 100-attempt policy so neither retry
exhaustion nor backoff inflation contributes to latency.

This fixture was used to prove the nested `forEach`×`forEach` + double `SetFanIn` machinery is
correct: instrumented tracing showed every inner cohort and the outer cohort firing exactly
once. There is no fan-in deadlock; the only failure mode was the request-timeout above.

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

- `ScanPDF` (forEach source over pages): ~1s delay, returns 5-22 pages of random image bytes
  (~50-150 KB each) and `pageCount`.
- `IdentifyChunks` (inner forEach source over chunks): 2-5 chunk rectangles per page (always
  >=1 so the inner forEach never empties).
- `TranscribeChunk`: 2-5s simulated OCR latency; 5% failure rate retried via
  `flow.Retry(10, 1s, 2.0, 5s)`; on success contributes one sentence (8-20 words) as the
  single-element `listTranscriptions` delta.
- `JoinPageTranscriptions` (`SetFanIn`, inner): joins a page's chunk transcriptions and
  contributes the page text as the single-element `listPageTexts` delta.
- `JoinDocTranscriptions` (`SetFanIn`, outer): joins all page texts into `docTranscription`,
  one page per line.

## Patterns exercised

- Doubly-nested dynamic fan-out (`forEach` within `forEach`)
- Two stacked explicit fan-ins (`SetFanIn` on the inner and outer join)
- `list*` append reducer at both fan-in levels, with distinct field names per level
  (`listTranscriptions` inner, `listPageTexts` outer) so reducers do not cross levels
- Deterministic fan-in ordering via `fan_out_ordinal` (page order, chunk order) despite the
  random per-chunk latency that scrambles completion order
- Task-initiated bounded retry with exponential backoff under a real failure rate
- A non-trivial `Rectangle` complex type flowing through state

## Deterministic assertions

Random page/chunk counts and transcription text make exact-value assertions impossible, so the
test asserts structural invariants:

- final status is `completed`
- `pageCount` in `[5, 22]`
- `docTranscription` is non-empty
- it has exactly `pageCount` lines (no page dropped or duplicated by the nested fan-in)
- every line is non-empty (every page produced >=1 chunk transcription)

## Runtime note

This is intentionally a heavy fixture (real 1s + 2-5s sleeps, large image bytes carried in
state through every nested step). The test pins the foreman to the maximum 64 workers to
parallelize the chunk transcriptions; expect a multi-second wall time.

The synchronous `foremanapi.Client.Run`/`Await` blocks on a single request whose context
bounds `Await`; the default request timeout is far shorter than this workflow. The test
therefore builds the foreman client with `pub.Timeout(5 * time.Minute)` - **omitting that is
the difference between green and an intermittent HTTP 408 on slow runs**, not a foreman bug.
`TranscribeChunk`'s retry uses a constant 500ms / 100-attempt policy so neither retry
exhaustion nor backoff inflation contributes to latency.

This fixture was used to prove the nested `forEach`×`forEach` + double `SetFanIn` machinery is
correct: instrumented tracing showed every inner cohort and the outer cohort firing exactly
once. There is no fan-in deadlock; the only failure mode was the request-timeout above.

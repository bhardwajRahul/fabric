# calculator.example

## Design Rationale

This microservice exists to demonstrate Microbus functional endpoint patterns, custom struct types in API signatures, and just-in-time observable metrics. It has no persistence and no downstream dependencies.

The `Arithmetic` handler normalizes `" "` back to `"+"` before the switch because `+` is URL-encoded as a space in query strings — callers using raw URL construction will hit this silently. The normalization keeps the echoed `opEcho` consistent with the actual operator used.

Division by zero is deliberately left to panic rather than being guarded with an explicit check. The platform's panic recovery converts it to a 500 error, and this behavior is called out in the code as an example of how the framework handles unexpected runtime errors. Intentional user-input validation failures should instead use an explicit `errors.New` with `http.StatusBadRequest`.

The four `atomic.Int64` fields on `Service` (`sumAdd`, `sumSubtract`, `sumMultiply`, `sumDivide`) accumulate running totals for the `SumOperations` observable gauge. They are updated inline in `Arithmetic` rather than in the JIT observer `OnObserveSumOperations`, because the observer is called on a metrics-scrape interval and must return the current snapshot without performing any computation itself. `atomic.Int64` is used instead of a mutex-guarded field because the accumulation is a single add with no read-modify-write dependency.

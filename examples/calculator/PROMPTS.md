## Calculator Microservice

Create an example microservice at hostname `calculator.example` that performs simple integer arithmetic and demonstrates functional endpoints, custom types, and observable metrics.

## Endpoints

Three functional endpoints on port `:443`:

- `Arithmetic` on `GET :443/arithmetic` — accepts `x int`, `op string`, `y int`; returns `xEcho int`, `opEcho string`, `yEcho int`, `result int`. Performs `+`, `-`, `*`, `/` on two integers. Returns an error for unknown operators. Note: `+` may arrive URL-encoded as a space character `" "` and must be normalized back to `"+"`. After each operation, increments the `UsedOperators` counter metric and accumulates the result into one of four `atomic.Int64` fields (one per operator) for the `SumOperations` observable gauge.
- `Square` on `GET :443/square` — accepts `x int`, returns `xEcho int`, `result int`. Computes `x * x`. Increments `UsedOperators` with operator label `"^2"`.
- `Distance` on `ANY :443/distance` — accepts `p1 Point`, `p2 Point`, returns `d float64`. Computes Euclidean distance using `math.Sqrt`. Demonstrates use of a custom struct type `Point` (defined in the `calculatorapi` package with fields `X float64` and `Y float64`).

## Metrics

Two metrics:

- `UsedOperators` — counter, `otelName: used_operators`, signature `UsedOperators(num int, op string)`. Incremented on every arithmetic or square operation.
- `SumOperations` — observable gauge, `otelName: sum_operations`, signature `SumOperations(sum int, op string)`. Measured just-in-time via `OnObserveSumOperations`, which calls `svc.RecordSumOperations` once per operator (`+`, `-`, `*`, `/`) using the corresponding `atomic.Int64` accumulator.

## State

Four `atomic.Int64` fields on the `Service` struct — `sumAdd`, `sumSubtract`, `sumMultiply`, `sumDivide` — accumulate the running sum of results for each operator. These are read in `OnObserveSumOperations` for the observable gauge. No initialization or reset is needed.

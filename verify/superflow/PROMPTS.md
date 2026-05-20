## Initial design

Create a unified workflow fixture, `superflow.verify`, whose Super graph exercises every workflow transition
primitive (sequential, dynamic fan-out via forEach, fan-in, OnError with sibling-cancel, conditional via
AddTransitionWhen, subgraph call, withGoto) in a single topology. Tasks are uniform: each one increments a
per-name atomic visit counter on the service, then reads a behaviors map from flow state to optionally
sleep, return an error with a chosen HTTP status, interrupt, goto, or retry. Tests assert on the counters
and run the same graph under 1 shard and 4 shards to exercise sharding orthogonal to shape.

The fixture is additive: existing per-behavior fixtures (basicflow, fanoutflow, conditionalflow, etc.)
remain as readable single-purpose documentation. superflow covers cross-product behaviors that no single
existing fixture can express alone. 404 ack-timeout, priority, fairness, sharding-distribution, and
backpressure remain in their dedicated fixtures because their tests rely on side-channel recording that is
intrinsic to each.

# Tickers

Tickers are background jobs that need to run irrespective of an external trigger. Tickers have a fixed schedule, with consecutive iterations separated by a fixed interval. If execution of a prior iteration takes longer than the duration of the interval and has not yet completed by the time the next iteration is due, the new iteration is skipped.

Tickers run independently for each replica of the microservice, so a once an hour ticker will run 5 times an hour if 5 replicas are running concurrently. To avoid coordinated spikes of multiple tickers all running at the same time, iterations are aligned to the startup time of the microservice, not to clock time. 

Tickers are defined using the `Connector`s `StartTicker` method which accepts a name, an interval and a callback function (handler). If the microservice is running, the ticker activates immediately. Otherwise, it will activate when the microservice starts. `StopTicker` can be used to stop a ticker identified by its name.

The common case is to let the [coding agent](../blocks/coding-agents.md) define tickers.

Tickers are disabled in the `TESTING` [deployment environment](../tech/deployments.md) in order to avoid the unpredictability of their running schedule.

If a job is running at the time that the microservice shuts down, the `ctx` argument of the handler gets canceled and the job is given a [grace period to end cleanly](../blocks/graceful-shutdown.md).

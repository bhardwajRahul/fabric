# Metrics

Observability is crucial when operating a SaaS system because it's not possible to debug it live. Alongside [structured logging](../blocks/logging.md) and [distributed tracing](../blocks/distrib-tracing.md), metrics are one of the pillars of observability.

`Microbus` supports both push and pull models:
- To push metrics to an [OpenTelemetry](https://opentelemetry.io) collector, set the `OTEL_EXPORTER_OTLP_METRICS_ENDPOINT` or `OTEL_EXPORTER_OTLP_ENDPOINT` [environment variable](../tech/envars.md#opentelemetry) appropriately
- Alternatively, configure [Prometheus](https://prometheus.io) to scrape metrics from the [metrics](../structure/coreservices-metrics.md) core microservice

<img src="metrics-1.png" width="1158">

### Standard Metrics

By default, all microservices produce a standard set of metrics:

* The microservice's uptime
* Histogram of the execution time of callbacks such as `OnStartup`, `OnShutdown`, tickers, etc.
* Histogram of the processing time of incoming requests
* Histogram of the size of the response to incoming requests
* Count of outgoing requests, along with the error and status code
* Histogram of time to receive an acknowledgement from a downstream microservice
* Count of log messages recorded
* Count of distributed cache operations, including hit and miss stats
* Memory usage of the distributed cache

### Custom Metrics

Custom metrics may be defined using the `Connector`'s `DescribeCounter`, `DescribeGauge` or `DescribeHistogram`. Metrics are incremented or observed using `AddCounter`, `RecordGauge` or `RecordHistogram`, depending on their type.

[Code generation](../blocks/codegen.md) can be used to assist in the definition of metrics.

```yaml
# Metrics
#
# signature - Go-style method signature (numeric measure and labels for arguments, no return value)
#   MyMetric(measure float64) - int, float or duration measure argument
#   MyMetric(measure int, label1 string, label2 int, label3 bool) - labels of any type
#   MyMetricSeconds(dur time.Duration) - time unit name as suffix
#   MyMetricMegaBytes(mb float64) - byte size unit name as suffix
# description - Documentation
# kind - The kind of the metric, "counter" (default), "gauge" or "histogram"
# buckets - Bucket boundaries for histograms [x,y,z,...]
# alias - The name of the OpenTelemetry metric (defaults to module_package_function_name)
# callback - Whether or not to observe the metric just in time (defaults to false)
metrics:
  - signature: Likes(num int, postId string)
    description: Likes counts the number of likes for a given post.
    kind: counter
```

### Usage in Code

`AddCounterLikes` is created by the code generator based on the `service.yaml` example above.

```go
func (svc *Intermediate) AddCounterLikes(ctx context.Context, num int, postId string) error {
	// ...
}
```

It can then be used to count the number of likes in the relevant endpoint.

```go
func (svc *Service) Like(ctx context.Context, postId string) error {
	// ...

	err := svc.AddCounterLikes(ctx, 1, postId)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}
```

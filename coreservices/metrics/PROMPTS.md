## Metrics Core Service

Create a core microservice at hostname `metrics.core` that aggregates Prometheus metrics from all microservices and exposes them for collection by a scraper.

Expose one endpoint:

- `Collect` on `GET /collect` — serves a Prometheus text-format metrics response aggregated from all running microservices.

### Collect Logic

1. Require authentication: if `SecretKey` is configured (non-empty), require the request to provide a matching `secretKey`, `secretkey`, or `secret_key` query parameter. Return `404` on mismatch. Skip the check entirely in `LOCAL` and `TESTING` deployments. If `SecretKey` is empty outside those deployments, return an error (scraper must configure a key in production).

2. Accept an optional `service` query parameter (defaults to `"all"`) to filter which microservice(s) to scrape. Validate the value with `httpx.ValidateHostname`.

3. Use gzip compression on the response when the client sends `Accept-Encoding: gzip`, except in `LOCAL` deployment (to avoid special characters in NATS debug mode).

4. Respect the `X-Prometheus-Scrape-Timeout-Seconds` header to impose a context deadline matching the scraper's timeout.

5. Discover live microservices by multicasting `controlapi.NewMulticastClient(svc).ForHost(host).PingServices(ctx)` and iterate the results. For each hostname returned, launch a goroutine (stagger requests by 1ms per service to avoid simultaneous fan-in) that fetches `https://<hostname>:888/metrics` directly via `svc.Publish`. Copy each response body (decompressing gzip if needed) into the shared output writer under a `sync.Mutex`. Log warnings for fetch errors or non-200 status codes without failing the whole collection (e.g. status 501 means Prometheus exporter is disabled on that instance).

6. Wait for all goroutines with `sync.WaitGroup` then flush the gzip writer if used.

Config:

- `SecretKey` — secret string required as a query parameter for collection. Required in non-local/test deployments. Marked `secret: true`.

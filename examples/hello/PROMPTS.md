## Hello Microservice

Create an example microservice at hostname `hello.example` that demonstrates a broad range of Microbus features in a single service: configuration, tickers, downstream calls, localization, static file serving, and multicast pinging.

## Configuration

Two config properties:

- `Greeting` (string, default `Hello`) — the word used to greet in the `Hello` endpoint
- `Repeat` (int, default `1`, validation `int [0,100]`) — how many times to repeat the greeting line

Both are read at request time via `svc.Config(...)` and `strconv.Atoi`.

## Endpoints

Expose seven web handler endpoints (all method `ANY` unless noted):

- `Hello` on `/hello` — reads the `?name=` query param (defaults to `World`), prepends the `Greeting` config value, repeats the line `Repeat` times, and writes it as `text/plain`.
- `Echo` on `/echo` — serializes the incoming HTTP request in wire format using `r.Write(w)`. Clears the `User-Agent` header if absent to avoid Go's default value appearing in the output.
- `Ping` on `/ping` — multicasts `GET https://all:888/ping` (the management port) with `pub.Multicast()` and collects responses from all running microservices, printing `<id>.<hostname>` per line as `text/plain`. Uses `frame.Of(res).FromHost()` and `frame.Of(res).FromID()` to extract metadata from response frames.
- `Calculator` on `/calculator` — renders a simple HTML form for arithmetic. On form submission, calls `calculatorapi.NewClient(svc).Arithmetic(ctx, x, op, y)` and displays the result. Demonstrates downstream service-to-service calls.
- `BusPNG` on `GET /bus.png` — serves an embedded PNG file from the `resources/` directory via `svc.ServeResFile("bus.png", w, r)`.
- `Localization` on `/localization` — calls `svc.LoadResString(ctx, "Hello")` to look up the locale-appropriate translation of "Hello" from `resources/text.yaml` (keyed by `Hello`, with translations in `en`, `fr`, `es`, `it`, `de`, `pt`, `da`, `nl`, `pl`, `no`, `tr`, `sv`), then writes it to the response. The framework picks the best match for the request's `Accept-Language` header.
- `Root` on `//root` — mapped to the ingress root `/`. Returns a minimal HTML page `<html><body><h1>Microbus</h1></body></html>`.

## Ticker

- `TickTock` — fires every `10s`, logs `"Ticktock"` at `Info` level.

## Downstream Dependency

Imports `calculatorapi` for the `Calculator` endpoint. The `calculator.example` service must be present in the app.

## Resources

Place a `bus.png` image and a `text.yaml` localization file in `resources/`. The YAML structure maps a string key (`Hello`) to a map of locale codes to translated strings.

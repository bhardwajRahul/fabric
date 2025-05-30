# `service.yaml`

`service.yaml` defines the various characteristics of the microservice in a declarative fashion. The code generator then picks up these definitions to generate boilerplate and skeleton code, leaving it up to the developer to fill in the gaps and implement the business logic. Code generation is additive and idempotent. Declarations can be added over time and applied incrementally.

`service.yaml` include several sections that define the characteristics of the microservice: general, configs, functions, events, sinks, webs, tickers.

<img src="./service-yaml-1.drawio.svg">
<p></p>

## General

The `general` section of `service.yaml` defines the `host` name of the microservice and a human-friendly `description`. The hostname is required. It is how the microservice will be addressed by other microservices. A hierarchical naming scheme for hostnames such as `myservice.mydomain.mysolution` can help avoid conflicts. These names along with port numbers are the basis for enforcing [access control](../blocks/unicast.md#notes-on-subscription-subjects) in low-trust environments.

```yaml
# General
#
# host - The hostname of the microservice
# description - A human-friendly description of the microservice
# integrationTests - Whether or not to generate integration tests (defaults to true)
# openApi - Whether or not to generate an OpenAPI document at openapi.json (defaults to true)
general:
  host:
  description:
```

## Configs

The `configs` section is used to define the [configuration](../blocks/configuration.md) properties of the microservices. Config properties get their values in runtime from the [configurator](../structure/coreservices-configurator.md) core microservice. 

```yaml
# Config properties
#
# signature - Func() (val Type)
# description - Documentation
# default - A default value (defaults to empty)
# validation - A validation pattern
#   str ^[a-zA-Z0-9]+$
#   bool
#   int [0,60]
#   float [0.0,1.0)
#   dur (0s,24h]
#   set Red|Green|Blue
#   url
#   email
#   json
# callback - "true" to handle the change event (defaults to "false")
# secret - "true" to indicate a secret (defaults to "false")
configs:
  # - signature:
  #   description:
  #   default:
  #   validation:
```

The `signature` is required. It defines the name and type of the property. The name must start with an uppercase letter. Types are limited to `string`, `bool`, `int`, `float` or `Duration`.

`validation` is enforced before accepting a new value for the config property. A validation comprises of a type and an optional regexp (for strings) or range (for numeric types).

If `callback` is set to `true`, a callback function will be generated and called when the value changes in runtime.

```go
// OnChangedFoo is triggered when the value of the Foo config property changes.
func (svc *Service) OnChangedFoo(ctx context.Context) (err error) {
    return nil
}
```

## Functions

`functions` define a web endpoint that is made to appear like a function (RPC). Input arguments are pulled from either the JSON body of the request or from the query arguments. Output arguments are written as JSON to the body of the response.

```yaml
# Functions
#
# signature - Go-style method signature
#   MyFunc(s string, f float64, i int, b bool) (t time.Time, d time.Duration)
#   MyFunc(val Complex, ptr *Complex)
#   MyFunc(m1 map[string]int, m2 map[string]*Complex) (a1 []int, a2 []*Complex)
#   MyFunc(httpRequestBody *Complex, queryArg int, pathArg string) (httpResponseBody []string, httpStatusCode int)
# description - Documentation
# method - "GET", "POST", etc. or "ANY" (default)
# path - The URL path of the subscription, relative to the hostname of the microservice
#   (empty) - The function name in kebab-case (default)
#   /path - Default port :443
#   /directory/{filename+} - Greedy path argument
#   /article/{aid}/comment/{cid} - Path arguments
#   :443/path
#   :443/... - Ellipsis denotes the function name in kebab-case
#   :443 - Root path of the microservice
#   :0/path - Any port
#   //example.com:443/path
#   https://example.com:443/path
#   //root - Root path of the web server
# queue - The subscription queue
#   default - Load balanced (default)
#   none - Pervasive
# actor - Authorization requirements as a boolean expression over actor properties
# openApi - Whether or not to include this endpoint in the OpenAPI document (defaults to true)
functions:
  # - signature:
  #   description:
```

The `signature` defines the function name (which must start with an uppercase letter) and the input and output arguments. For any unknown type, the code generator automatically defines an empty struct in a file of the same name in the API package of the microservice. 

`Microbus` takes care of marhsaling and unmarshaling of arguments and return values behind the scenes. Input arguments are unmarshaled from either the HTTP request path (if [path arguments](../tech/path-arguments.md) of the same name are defined), HTTP query arguments of the same name, or the JSON or URL form encoded body of the HTTP request. Output arguments are marshaled as JSON to the HTTP response body. Specially named [HTTP magic arguments](../tech/http-arguments.md) allow finer control over the marshaling and unmarshaling of arguments.

A typical generated functional request handler will look similar to the following:

```go
/*
FuncHandler is an example of a functional handler.
*/
func (svc *Service) FuncHandler(ctx context.Context, id string) (ok bool, err error) {
    return ok, nil
}
```

`method` can be used to restrict the function to accept only certain HTTP methods. By default, the function is agnostic to the HTTP method.

Along with the hostname of the microservice, the `path` defines the URL to this endpoint. It defaults to the function name in `kebab-case`. It may include [path arguments](../tech/path-arguments.md).

`queue` defines whether a request is routed to one of the replicas of the microservice (load-balanced) or to all (pervasive).

`actor` stipulates authorization requirements as a boolean expression over the properties of the actor associated with the request. If no actor is associated with the request, it is rejected with a `401 Unauthorized` error. If an actor is associated but it does not satisfy the requirements, the request is rejected with a `403 Forbidden` error.

The `actor` boolean expression supports the following syntax:
- Boolean operators `&&`, `||` and `!`
- Comparison operators `==` and `!=`
- Order operators `>`, `>=`, `<` and `<=`
- Regexp operators `=~` and `!~`. The regular expression must be quoted and on the right, e.g. `prop=~"regexp"`
- Grouping operators `(` and `)`
- String quotation marks `"` or `'`
- Dot notation `.` for traversing nested objects. For this purpose, arrays are construed as `map[string]bool`, enabling expressions such as `array.value` to match the property `"array": ["value", "another_value"]`

`openApi` controls whether or not to expose the function in the `/openapi.json` endpoint.

## Event Sources

`events` are very similar to functions except they are outgoing rather than incoming function calls. An event is fired without knowing in advance who is (or will be) subscribed to handle it. [Events](../blocks/events.md) are useful to push notifications of events that occur in the microservice that may interest upstream microservices. For example, `OnUserDeleted(id string)` could be an event fired by a user management microservice.

```yaml
# Event sources
#
# signature - Go-style method signature
#   OnMyEvent(s string, f float64, i int, b bool) (t time.Time, d time.Duration)
#   OnMyEvent(val Complex, ptr *Complex)
#   OnMyEvent(m1 map[string]int, m2 map[string]*Complex) (a1 []int, a2 []*Complex)
#   OnMyEvent(httpRequestBody *Complex, queryArg int, pathArg string) (httpResponseBody []string, httpStatusCode int)
# description - Documentation
# method - "GET", "POST", etc. (defaults to "POST")
# path - The URL path of the subscription, relative to the hostname of the microservice
#   (empty) - The function name in kebab-case (default)
#   /path - Default port :417
#   /directory/{filename+} - Greedy path argument
#   /article/{aid}/comment/{cid} - Path arguments
#   :417/path
#   :417/... - Ellipsis denotes the function name in kebab-case
#   :417 - Root path of the microservice
#   //example.com:417/path
#   https://example.com:417/path
#   //root - Root path of the web server
events:
  # - signature:
  #   description:
```

The `signature` defines the event name (which must start with the word `On` followed by an uppercase letter) and the input and output arguments. In `Microbus`, events are bi-directional and event sinks may return values back to the event source.

## Event Sinks

`sinks` are the flip side of event sources. A sink subscribes to consume events that are generated by other microservices.

```yaml
# Event sinks
#
# signature - Go-style method signature
#   OnMyEvent(s string, f float64, i int, b bool) (t time.Time, d time.Duration)
#   OnMyEvent(val Complex, ptr *Complex)
#   OnMyEvent(m1 map[string]int, m2 map[string]*Complex) (a1 []int, a2 []*Complex)
#   OnMyEvent(httpRequestBody *Complex, queryArg int, pathArg string) (httpResponseBody []string, httpStatusCode int)
# description - Documentation
# event - The name of the event at the source (defaults to the function name)
# source - The package path of the microservice that is the source of the event
# forHost - For an event source with an overridden hostname
# queue - The subscription queue
#   default - Load balanced (default)
#   none - Pervasive
# actor - Authorization requirements as a boolean expression over actor properties
sinks:
  # - signature:
  #   description:
  #   source: package/path/of/another/microservice
```

The `signature` of an event sink must match that of the event source. The one exception to this rule is the option to use an alternative name for the handler function while providing the original event name in the `event` field. This allows an event sink to resolve conflicts if different event sources use the same name for their events. That becomes necessary because handler function names must be unique in the scope of the sink microservice.

```yaml
sinks:
  - signature: OnDeleteUser(userID string)
    event: OnDelete
    source: package/path/to/user/store
  - signature: OnDeleteGroup(groupID string)
    event: OnDelete
    source: package/path/to/group/store
```

The `source` must point to the microservice that is the source of the event. This is the fully-qualified package path.

The optional field `forHost` adjusts the subscription to listen to microservices whose hostname was changed with `SetHostname`.

## Web Handlers

The `webs` section defines raw web handlers which allow the microservice to handle incoming web requests with full access to the `r *http.Request` and full control over the `w http.ResponseWriter`.

```yaml
# Web handlers
#
# signature - Go-style method signature (no arguments or return values)
#   MyHandler()
# description - Documentation
# method - "GET", "POST", etc. or "ANY" (default)
# path - The URL path of the subscription, relative to the hostname of the microservice
#   (empty) - The function name in kebab-case (default)
#   /path - Default port :443
#   /directory/{filename+} - Greedy path argument
#   /article/{aid}/comment/{cid} - Path arguments
#   :443/path
#   :443/... - Ellipsis denotes the function name in kebab-case
#   :443 - Root path of the microservice
#   :0/path - Any port
#   //example.com:443/path
#   https://example.com:443/path
#   //root - Root path of the web server
# queue - The subscription queue
#   default - Load balanced (default)
#   none - Pervasive
# actor - Authorization requirements as a boolean expression over actor properties
# openApi - Whether or not to include this endpoint in the OpenAPI document (defaults to true)
webs:
  # - signature:
  #   description:
```

The `signature` must not include any arguments. The handler receives the `*http.Request` from where it is expected to extract the input, and the `http.ResponseWriter` where it can write the response.

The code generated web handler will look similar to this:

```go
/*
WebHandler is an example of a web handler.
*/
func (svc *Service) WebHandler(w http.ResponseWriter, r *http.Request) (err error) {
    return nil
}
```

## Tickers

Tickers are means to trigger a job on a periodic basis. The `signature` and `interval` fields are required.

```yaml
# Tickers
#
# signature - Go-style method signature (no arguments or return values)
#   MyTicker()
# description - Documentation
# interval - Duration between iterations (e.g. 15m)
tickers:
  # - signature:
  #   description:
  #   interval:
```

The code generated ticker handler will look similar to this:

```go
/*
TickerHandler is an example of a ticker handler.
*/
func (svc *Service) TickerHandler(ctx context.Context) (err error) {
    return nil
}
```

`ctx` will be canceled when the microservice shuts down.

## Metrics

The `metrics` section is used to define custom metrics, whether operational in nature (e.g. performance) or having a business purpose (e.g. usage tracking).

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
  # - signature:
  #   description:
  #   kind:
```

It is recommended to follow [best practices](https://prometheus.io/docs/practices/naming/) when naming metrics.

Metrics support three [instrument types](https://opentelemetry.io/docs/specs/otel/metrics/data-model/): counter, gauge and histogram.

The OpenTelemetry name of the metric is derived from the Go module's name, the Go package name and the function signature. An alias is automatically generated but may be overridden if necessary.

A callback can be created to observe the value of the metric just before it is pushed to the OpenTelemetry collector.

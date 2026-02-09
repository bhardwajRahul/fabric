# Path Arguments

Path arguments are request arguments that are extracted from a URL's path rather than its query string.

[Warning!](#warning) Path arguments interfere with [multicast](../blocks/multicast.md) as well as [locality-aware routing](../blocks/locality-aware-routing.md).

### Fixed Path

In the typical case, endpoints of a microservice have fixed URLs at which they are reachable.

An endpoint such as `Add(x int, y int) (sum int)` is typically reachable at the internal `Microbus` URL of `https://calculator.example/add` or the external URL of `https://localhost:8080/calculator.example/add` assuming that the ingress proxy is listening at `localhost:8080`. The arguments `x` and `y` of the function are unmarshaled from the request query argument or from the body of the request.

```http
GET /add?x=5&y=5 HTTP/1.1
Host: calculator.example
```

```http
POST /add?x=5&y=5 HTTP/1.1
Host: calculator.example

{"x":5,"y":5}
```

### Variable Path

A fixed path is consistent with the [RPC over JSON](../tech/rpc-vs-rest.md) style of API but is insufficient for implementing a [RESTful](../tech/rpc-vs-rest.md) style of API where it is common to expect input arguments in the path of the request. This is where path arguments come into play.

A variable path allows the `Load(id int) (article *Article)` endpoint to be reachable at the internal `Microbus` wildcard URL of `https://article/{id}`. The function's `id` argument is unmarshaled from the path argument of the same name.

```http
GET /1 HTTP/1.1
Host: article
```

### Greediness

A typical path argument only captures the data in one segment of the path, i.e. between two `/`s in the path or after the last `/`. The values of these arguments therefore cannot contain a `/`. However, multiple such path arguments may be defined in the path, for example `/article/{articleID}/comment/{commentID}`.

A greedy path argument captures the remainder of the path and can span multiple parts of the path and include `/`s. A greedy path argument must be the last element in the path specification. Greedy path arguments are denoted using a `...` in their definition, for example `//file/{category}/{filePath...}`.

### Unnamed Path Arguments

Path arguments that are left unnamed are automatically given the names `path1`, `path2` etc. in order of their appearance. In `/foo/{}/bar/{}/greedy/{...}` for example, the three unnamed path arguments are named `path1`, `path2` and `path3`. It is recommended to name path arguments and avoid this ambiguity.

### Conflicts

Path arguments are a form of wildcard subscription and might overlap if not crafted carefully. For example, if subscriptions are created at both `/{suffix...}` and `/hello/{name}`, requests to `/hello/world` will alternate between the two handlers and will result in unpredictable behavior.

### Web Handlers

Path arguments are also allowed for web handlers, in which case their value can be obtained using `r.PathValue`. The next example assumes the route `/avatar/{uid}/{size}/{name...}`.

```go
func (svc *Service) AvatarImage(w http.ResponseWriter, r *http.Request) (err error) {
    // Use r.PathValue to obtain the value of a path argument
    uid := r.PathValue("uid")
    size, _ := strconv.Atoi(r.PathValue("size"))
    name := r.PathValue("name")

    return serveImage(uid, size)
}
```

### Warning

Path arguments are not recommended for multicast endpoints, events or sinks. Using path arguments in these cases results in significantly slower response times because they interfere with an optimization that relies on a fixed URL pattern.

In addition, the variable nature of the path also interferes with locality-aware routing and relevant requests will route randomly instead.

Given these constraints, it is advised to use path arguments only for external-facing endpoints and use fixed paths for internal service-to-service communications.
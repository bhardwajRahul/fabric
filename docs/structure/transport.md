# Package `transport`

The `transport` package implements the communication substrate that microservices use to exchange messages. It abstracts the details of the underlying messaging system behind a single `Conn` type that supports both NATS-based distributed messaging and an in-process [short-circuit](../tech/short-circuit.md) path for co-located microservices.

A `Conn` is opened with `Open` and closed with `Close`. Messages are sent with `Publish` (one-to-many), `Request` (one-to-one) and `Response` (reply to a request). Interest in a subject is expressed with `Subscribe` (all subscribers receive every message) or `QueueSubscribe` (messages are [load-balanced](../blocks/lb.md) across members of the same queue group).

Messages are represented by the `Msg` type, which carries the raw bytes alongside the parsed `*http.Request` or `*http.Response`. The `MsgHandler` function type processes incoming messages.

Subjects support wildcards: `*` matches a single segment and `>` matches the remainder of the subject. A prefix tree is used internally for efficient subject routing over the short-circuit transport.

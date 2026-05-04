## SMTP Ingress Core Service

Create a core microservice at hostname `smtp.ingress.core` that listens for incoming SMTP email and fires an `OnIncomingEmail` outbound event for each received message.

The service embeds a `go-guerrilla` SMTP daemon (`github.com/phires/go-guerrilla`) and a mutex to serialize restarts. The daemon is started in `OnStartup` and shut down in `OnShutdown`. All five config callbacks restart the daemon via a synchronized `restartDaemon` helper that calls `stopDaemon` then `startDaemon` while holding the mutex.

The daemon's Logrus output is diverted to the Microbus logger via a `logHook` (implementing `logrus.Hook`) that bridges all six logrus levels to the appropriate `svc.Log*` calls, mapping logrus fields to `slog` name=value pairs.

Register a custom backend processor named `"MessageProcessor"` that intercepts each `TaskSaveMail` event. In the processor, create an OpenTelemetry root span using `svc.StartSpan(svc.Lifetime(), ":"+port, trc.Server())`. Parse the raw email with `letters.ParseEmail` (`github.com/mnako/letters`), log the message ID and date, then multicast `OnIncomingEmail` to all subscribers. Only attach request attributes to the span in `LOCAL` deployment or on error. Force-trace on error.

The `Email` type in `smtpingressapi` is a type alias for `letters.Email` (the parsed email struct from the `mnako/letters` library).

TLS is enabled automatically if both `smtpingress-<port>-cert.pem` and `smtpingress-<port>-key.pem` exist in the current working directory. The `AllowedHosts` is `["."]` (accept mail for any domain). The backend pipeline is `"HeadersParser|Header|MessageProcessor"`.

Config properties (all with `callback: true`, all restart the daemon on change):

- `Port` — TCP port to listen on, default `25`, range `[1, 65535]`
- `Enabled` — whether the server starts, default `true`
- `MaxSize` — maximum message size in megabytes, default `10`, range `[0, 1024]`
- `MaxClients` — maximum concurrent client connections, default `128`, range `[1, 1024]`
- `Workers` — number of backend save workers, default `8`, range `[1, 1024]`

Outbound event:

- `OnIncomingEmail` on `:417/on-incoming-email` — fires with a `*letters.Email` payload when a message is received.

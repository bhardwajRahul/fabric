# Embedded Resources

The `Connector` construct provides a read-only file system (`FS`) from which microservices can read static resources during runtime. The convenience methods `ReadResFile`, `ReadResTextFile`, `ServeResFile`, `WriteResTemplate` and `LoadResString` provide access to the file system.

The `Connector`'s file system is initialized to the current working directory. Two microservices sharing the same app will therefore share the same working directory and their file systems will overlap. The `FS` can be set using the `Connector`'s `SetResFS` before the microservice is started.

In the common case a microservice is created using a [coding agent](../blocks/coding-agents.md) that takes care to initialize the file system to an [embedded `FS`](https://pkg.go.dev/embed) pointing to the `resources` directory in the [source directory of the microservice](../blocks/uniform-code.md). Any source files placed in that directory are automatically made available via the `FS`.

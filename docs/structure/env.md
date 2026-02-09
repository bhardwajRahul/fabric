# Package `env`

Package `env` manages the loading of [environment variables](../tech/envars.md). A lookup checks three sources in order: an in-memory stack, an `env.yaml` file in the current working directory or an ancestor directory, and the OS environment.

`Get(key string) string` and `Lookup(key string) (string, bool)` retrieve a variable's value. Keys are case-sensitive.

`Push(key, value string)` and `Pop(key string)` manipulate the in-memory stack, which is useful for overriding variables during [integration testing](../blocks/integration-testing.md).

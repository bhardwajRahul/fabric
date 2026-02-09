# Package `utils`

Package `utils` includes various independent utilities.

`RandomIdentifier(length int) string` generates a random alphanumeric string of the given length, useful for producing unique IDs such as message IDs and plane names.

`SyncMap` is a thin wrapper over a subset of the operations of the standard `sync.Map`. It introduces generics to make these more type-safe.

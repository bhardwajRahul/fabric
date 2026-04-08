# SQL Databases

Microbus microservices sometimes need to persist data in a SQL database. Rather than coupling directly to a single database engine, Microbus relies on the [Sequel](https://github.com/microbus-io/sequel) library to provide a unified API across MySQL, PostgreSQL, Microsoft SQL Server and SQLite. This allows teams to choose the database that best fits their deployment while keeping the application code database-agnostic.

Sequel is a lightweight Go library that wraps `sql.DB` with cross-driver conveniences. It rewrites queries at runtime - converting parameter placeholders to the driver-native format and expanding virtual functions like `NOW_UTC()` and `LIMIT_OFFSET()` into the correct dialect. It also manages connection pool sizing, schema migrations, and provides helpers for mapping between Go values and SQL NULLs. Sequel targets the specific needs of Microbus but can be used independently of the framework.

The most common use of Sequel is inside [CRUD microservices](sql-crud.md) - microservices whose primary job is to persist and retrieve domain objects. A dedicated coding agent skill scaffolds these microservices with database-agnostic code that works against all four supported databases without modification.

Sequel is also used by core microservices that require persistence. The Foreman core service - Microbus's built-in workflow orchestrator - persists workflow state and task queues to a SQL database via Sequel, including support for database sharding.

Because Sequel supports SQLite, tests can run against an ephemeral in-memory database by default. No external database server is needed - `go test` exercises the full lifecycle, including migrations and transactions, entirely in-process. The same tests can be pointed at a real database instance to validate behavior on the target engine.

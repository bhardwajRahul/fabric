# SQL CRUD Microservices

A common building block in any application is a microservice that persists and retrieves domain objects in a SQL database. Microbus provides a dedicated [coding agent skill](coding-agents.md) that scaffolds these microservices end-to-end - from the database table and migration scripts to a full REST API with type-safe client stubs.

## Code Generation

The coding agent generates the entire microservice in one pass: the object definition, column mappings, query filtering, bulk operations, REST endpoints, [integration tests](integration-testing.md), and a [mock](integration-testing.md#mocking) for use by upstream services. Subsequent skills add individual features such as configuration properties, events, or additional endpoints. The generated code follows the same [uniform structure](uniform-code.md) as any other Microbus microservice.

## Cross-Database Support

Generated CRUD microservices are database-agnostic. They rely on the [Sequel](https://github.com/microbus-io/sequel) library to abstract away dialect differences across MySQL, PostgreSQL, Microsoft SQL Server and SQLite. Sequel rewrites queries at runtime - converting parameter placeholders and expanding virtual functions like `NOW_UTC()` and `LIMIT_OFFSET()` into driver-native syntax - so the application code contains a single set of SQL statements that works everywhere.

## Schema Migration

Each microservice manages its own database schema through numbered migration scripts (`1.sql`, `2.sql`, ...) stored in the `resources/sql` directory. Sequel applies them in order on startup and tracks which scripts have already run. When the SQL syntax differs between databases, driver-specific blocks within the same script handle the divergence. Adding a new migration is as simple as creating the next numbered file.

## Multi-Tenancy

CRUD microservices include a tenant discriminator column that is automatically applied to every SQL statement. This provides row-level isolation between tenants without requiring separate databases or schemas. The tenant identifier is derived from the actor's claims and is transparent to callers of the service.

## Optimistic Concurrency

Every persisted object carries a revision number that increments on each update. Callers that need conflict detection can use revision-conditional updates - if another actor modified the record in the meantime, the update is rejected rather than silently overwriting the change.

## In-Memory Testing

Because the generated code is database-agnostic, tests run against an ephemeral in-memory SQLite database by default. No external database server is needed - `go test` exercises the full CRUD lifecycle, including migrations and transactions, entirely in-process. The same tests can be pointed at a real MySQL, PostgreSQL or SQL Server instance to validate behavior on the target engine.

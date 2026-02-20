# Developing SQL CRUD Microservices with the Microbus Framework

## Instructions for Agents

**CRITICAL**: These instructions pertain to SQL CRUD microservices only. Do not apply them to other flavors of microservices.

## Overview

SQL CRUD microservices are Microbus microservices that expose a CRUD API to persist and retrieve objects in and out of a SQL database.

## Key Concepts

### Multi-Tenant Architecture

SQL CRUD microservices use a tenant discriminator column `tenant_id` to guarantee isolation between tenants.
- `tenant_id` is included in every SQL statement: `INSERT`, `UPDATE`, `DELETE` or `SELECT`
- `tenant_id` is used as the first column in the primary (or clustering) key to contain table fragmentation on a per-tenant basis
- `tenant_id` is used as the first column in all composite indices to contain index fragmentation on a per-tenant basis

By default, the tenant ID is obtained from the actor claim `tenant` or `tid` in the `ctx` by the `Frame` in `tenantOf`. Universal tables that are shared among all tenants should explicitly return a tenant ID of `0` from `tenantOf`.

Solutions that are not multi-tenant where there isn't expected to be a `tenant` or `tid` claim, can safely ignore the tenant concept. The tenant discriminator column will default to `0`.

The tenant discriminator column is an integer.

### Encapsulation of Persistence Layer

Microservices encapsulate their persistence layer and expose functionality only via their public API. To avoid tight coupling, no microservice should refer to another microservice's persistence layer nor make any assumptions based on its internals. This means that a microservice should not define foreign key constraints, nor `JOIN` queries, that refer to the database table of another microservice.

Because direct `JOIN`s across microservice boundaries are not possible, an upstream microservice that needs data from a downstream microservice has three strategies:

- **On-demand querying** — Use the downstream microservice's bulk operations (e.g. `BulkLoad`, `List`) to fetch the needed records on demand. This is simple and always consistent, but adds latency from the extra round-trip.
- **Caching** — Maintain a cache of downstream objects in memory, e.g. in the upstream microservice's distributed cache. Subscribe to the downstream microservice's outbound events (e.g. `OnObjectStored`, `OnObjectDeleted`) to invalidate stale cache entries or as a trigger to fetch the latest data.
- **Denormalization** — Add columns to the upstream microservice's own database table that duplicate selected fields from the downstream microservice. Subscribe to the downstream microservice's outbound events to keep the denormalized columns up to date. This enables local `JOIN`s and filtering without cross-service calls, at the cost of eventual consistency and additional storage.

| Strategy | Consistency | Latency | Storage |
|---|---|---|---|
| On-demand querying | Strong | Higher (round-trip) | None |
| Caching | Eventual | Low (cache hit) | Volatile (memory) |
| Denormalization | Eventual | None (local query) | Durable (database) |

If the downstream data is immutable, the local copy never becomes stale and there is no need to subscribe to events for invalidation. For both caching and denormalization, a ticker can be used as an alternative or supplement to events, running queries on a schedule to periodically refresh the local data.

### Object Definition

The definition of the object's struct is in `object.go` in the API directory of the microservice. The object must be marshalable to and from JSON and should therefore contain only fields that are themselves marshalable. JSON tags should use camelCase.

### Object Key Definition

The object's key is defined in `objectkey.go` in the API directory of the microservice. Internally, the key is represented an a numerical ID.

By default, the key is encrypted when marshaled to JSON and decrypted when unmarshaled so as not to expose the table's cardinality to external users. Set `cipherEnabled` to `false` to disable the encryption. Do not alter the cipher key nor its nonce.

### Database Compatibility

SQL CRUD microservices must support three database drivers: `mysql` (MySQL), `pgx` (PostgreSQL) and `mssql` (SQL Server). This requirement affects both migration scripts and runtime code.

In migration scripts, use the `-- DRIVER: drivername` prefix when SQL syntax differs between databases. Common differences include column type modifiers (`MODIFY COLUMN` vs `ALTER COLUMN`), string types (`VARCHAR` vs `NVARCHAR`), and auto-increment syntax.

In runtime code, use `svc.db.DriverName()` to switch on the driver when SQL behavior diverges. Key areas where the databases differ:

- **Returning affected rows**: PostgreSQL supports `RETURNING id`, SQL Server supports `OUTPUT INSERTED.id`, while MySQL requires a `SELECT ... FOR UPDATE` within a transaction
- **Date arithmetic**: MySQL uses `DATE_ADD(NOW(), INTERVAL ? SECOND)`, PostgreSQL uses `NOW()+MAKE_INTERVAL(secs => ?)`, SQL Server uses `DATEADD(SECOND, ?, GETUTCDATE())`
- **Pagination**: MySQL and PostgreSQL use `LIMIT ? OFFSET ?`, SQL Server uses `OFFSET ? ROWS FETCH NEXT ? ROWS ONLY`
- **Parameter placeholders**: Use `?` placeholders in all queries and call `svc.db.ConformArgPlaceholders` to convert them to the driver-specific format (e.g. `$1, $2` for PostgreSQL)
- **Current time**: Use `svc.db.NowUTC()` to get a driver-appropriate SQL expression for the current UTC time, rather than hardcoding `NOW()` or `GETUTCDATE()`

### Revisions

SQL CRUD microservices use a `revision` integer column for optimistic concurrency control. The revision auto-increments on every update via `revision=(1+revision)` and is exposed on the object as the `Revision` field.

Two update strategies are available:

- `Store` / `BulkStore` update unconditionally — the revision is ignored and the update always succeeds (assuming the record exists). Use this when the caller has exclusive access to the record or does not need conflict detection.
- `Revise` / `BulkRevise` update conditionally — the `WHERE` clause includes `AND revision=?` per row, so the update only succeeds if the record's current revision matches the caller's copy. If another actor modified the record in the meantime, the revision will have advanced and the update is a no-op. Use this when concurrent modifications are possible and the caller must not silently overwrite another actor's changes.

`MustRevise` is a convenience wrapper around `Revise` that returns a `409 Conflict` error if the revision does not match, rather than a boolean.

### Bulk Operations

Every CRUD operation has a bulk counterpart (`BulkCreate`, `BulkStore`, `BulkRevise`, `BulkDelete`, `BulkReserve`, etc.) that processes multiple records in a single batched SQL statement. The singular operations delegate to their bulk counterparts with a single-element slice.

Prefer bulk operations over looping with singular operations to avoid the 1+N query pattern. For example, to delete a list of records, call `BulkDelete` once rather than calling `Delete` in a loop. Bulk operations reduce the number of database round-trips, which is the dominant cost in most CRUD workloads.

## Common Patterns

### Migration Scripts

SQL CRUD microservices use migration scripts to prepare the schema of the database table into which the object is stored. The migrations scripts are executed sequentially to enable evolution of the schema over time. A migration script will only executes once on a given database instance. The `resources/sql` directory of the microservice contains migrations scripts that are named with a numeric file name such as `1.sql`, `2.sql` etc. that represents their execution order.

**IMPORTANT**: Create a new migration script whenever you are tasked with making changes to the schema. Do not modify existing scripts unless explicitly told to do so.

To add a migration script, identify the file name with the largest value and create a new `.sql` file with that value incremented by one. For example, if the largest file name is `14.sql` name the new file `15.sql`.

Use SQL statements such as `CREATE TABLE` and `ALTER TABLE` to create or alter the schema of the database table. Separate statements with a `;` followed by a new line. Use snake_case for all database identifiers, including column names, index names and table names. Use UPPERCASE for SQL keywords.

It is necessary to enter different SQL statements for each of the supported database driver names when they use a different syntax. The prefix `-- DRIVER: drivername` before a statement indicates to run that statement only on the named database: `mysql` (MySQL), `pgx` (Postgres) or `mssql` (SQL Server).

```sql
-- DRIVER: mysql
ALTER TABLE my_table MODIFY COLUMN modified_column VARCHAR(384) NOT NULL DEFAULT '';

-- DRIVER: pgx
ALTER TABLE my_table ALTER COLUMN modified_column VARCHAR(384) NOT NULL DEFAULT '';

-- DRIVER: mssql
ALTER TABLE my_table ALTER COLUMN modified_column NVARCHAR(384) NOT NULL DEFAULT '';
```

### Column Mappings

Column mapping bridge the divide between database columns and Go object fields. Column mapping happens in four case: on create, on store, on read and on query.

`mapColumnsOnInsert` maps column names to their values during the initial `Create` action.
- All `NOT NULL` columns that do not have a `DEFAULT` value define in the database schema must be mapped to a value
- Values typically come from the corresponding field of the input `obj` but can be sources elsewhere
- Wrap a value in `sequel.Nullify` if the database column is `NULL`-able
- Wrap a string in `sequel.UnsafeSQL` to set the value using a SQL statement
- Use `sequel.UnsafeSQL(db.NowUTC())` to set the value to the database's current time in UTC
- When setting a value of `time.Time`, convert it to UTC first
- Exclude columns that the actor is not allowed to set on creation

```go
columnMapping = map[string]any{
	"first_name": obj.FirstName,
	"last_name":  obj.LastName,
	"time_zone":  sequel.Nullify(obj.TimeZone),
	"created_at": sequel.UnsafeSQL(db.NowUTC()),
	"updated_at": sequel.UnsafeSQL(db.NowUTC()),
}
```

`mapColumnsOnUpdate` maps column names to their values during followup `Store` actions.
- Only modifiable columns need be mapped to a value
- Values typically come from the corresponding field of the input `obj` but can be sources elsewhere
- Wrap a value in `sequel.Nullify` if the database column is `NULL`-able
- Wrap a string in `sequel.UnsafeSQL` to set the value using a SQL statement
- Use `sequel.UnsafeSQL(db.NowUTC())` to set the value to the database's current time in UTC
- When setting a value of `time.Time`, convert it to UTC first.
- Exclude columns that the actor is not allowed to modify

```go
columnMapping = map[string]any{
	"first_name": obj.FirstName,
	"last_name":  obj.LastName,
	"time_zone":  sequel.Nullify(obj.TimeZone),
	"updated_at": sequel.UnsafeSQL(db.NowUTC()),
}
```

`mapColumnsOnSelect` maps column names to their object fields during `List` actions.
- Wrap the object field reference in `sequel.Nullable` if the database column is `NULL`-able but the Go type of the field is not
- Use `sequel.Bind` to transform and apply the value manually to the object.
- Exclude columns that the actor is not allowed to read

```go
columnMapping = map[string]any{
	"id": &obj.ID,
	"tags": sequel.Bind(func(tags string) {
		return json.Unmarshal([]byte(tags), &obj.Tags)
	}),
	"birthday": sequel.Bind(func(modifiedTime time.Time) {
		obj.Year, obj.Month, obj.Day = modifiedTime.Date()
		return nil
	}),
}
```

`prepareWhereClauses` prepares the conditions to add to the `WHERE` clause of the `SELECT` statement based on the input `Query`.
- Conditions are `AND`ed together so all conditions must be met for a database record to match the query
- **IMPORTANT**: Add `WHERE` conditions only for non-zero filtering option in the `Query`
- Exclude columns that the actor is not allowed to filter on

```go
if query.Title != "" {
	conditions = append(conditions, "title=?")
	args = append(args, query.Title)
}
if !query.UpdatedAtGTE.IsZero() {
	conditions = append(conditions, "updated_at>=?")
	args = append(args, query.UpdatedAtGTE.UTC())
}
if !query.UpdatedAtLT.IsZero() {
	conditions = append(conditions, "updated_at<?")
	args = append(args, query.UpdatedAtLT.UTC())
}
```

Column mapping and query conditions can be made to be dependent on the claims associated with the actor of the request. For example, an admin may be allowed to read additional columns from the `user` table, or a guest user may be applies a `WHERE` condition in order to restrict their view of the results.

```go
var actor Actor
frame.Of(ctx).ParseActor(&actor)
if actor.IsAdmin() {
	// ...
} else {
	// ...
}
```

### Query Filtering Options

The `Query` struct specifies filtering options which are translated to `WHERE` conditions by `prepareWhereClauses` and applied to the `SELECT` SQL statement in the `List` functional endpoint.

```go
type Query struct {
	Name string         `json:"name,omitzero"`
	AgeGTE int          `json:"ageGte,omitzero"`
	AgeLT int           `json:"ageLte,omitzero"`
	OnlyCitizen bool    `json:"onlyCitizen,omitzero"`
	OnlyNotCitizen bool `json:"onlyNotCitizen,omitzero"`
	States []string     `json:"states,omitzero"`
}
```

Zero-valued filtering options should not result in a `WHERE` condition because an empty `Query` should select all records.

```go
func (svc *Service) prepareWhereClauses(ctx context.Context, query bookapi.Query) (conditions []string, args []any, err error) {
	// String filtering option
	if query.Name != "" {
		conditions = append(conditions, "name=?")
		args = append(args, query.Name)
	}
	// Range filtering option
	if query.AgeGTE != 0 {
		conditions = append(conditions, "age>=?")
		args = append(args, query.AgeGTE)
	}
	if query.AgeLT != 0 {
		conditions = append(conditions, "age<?")
		args = append(args, query.AgeLT)
	}
	// Boolean filtering option
	if query.OnlyCitizen {
		conditions = append(conditions, "citizen=1")
	}
	if query.OnlyNotCitizen {
		conditions = append(conditions, "citizen=0")
	}
	// List filtering option
	if query.States != nil {
		if len(query.States) > 0 {
			conditions = append(conditions, "states IN (" + strings.Repeat("?,", len(query.States)-1) + "?)")
			args = append(args, query.States...)
		} else {
			conditions = append(conditions, "1=0") // Empty array should yield no result
		}
	}
	return conditions, args, nil
}
```

### REST API

SQL CRUD microservices expose a REST API by defining thin functional endpoints that delegate to the core CRUD operations and translate results into HTTP semantics. These endpoints use the magic variable names `httpRequestBody`, `httpResponseBody` and `httpStatusCode` for automatic request/response body marshaling and status code control.

Five endpoints cover the standard REST verbs:

| Endpoint | Method | Route | Delegates to |
|---|---|---|---|
| `CreateREST` | `POST` | `/objects` | `Create` |
| `StoreREST` | `PUT` | `/objects/{key}` | `Store` |
| `DeleteREST` | `DELETE` | `/objects/{key}` | `Delete` |
| `LoadREST` | `GET` | `/objects/{key}` | `Load` |
| `ListREST` | `GET` | `/objects` | `List` |

The route uses the plural form of the object name in kebab-case (e.g. `/bus-stops` or `/busstops`). The `{key}` path argument is automatically parsed into the object's key type.

### Reservation

SQL CRUD microservices can optionally support reservation of records via a `reserved_before` timestamp column. Reservation is a form of optimistic locking that prevents concurrent actors from processing the same record, without blocking normal CRUD operations (`Create`, `Store`, `Delete`, `Load`, `List`, etc.).

A record is considered reserved when `reserved_before > NOW`. Reservations are time-limited and expire automatically — no explicit release is required, though calling `Reserve` with a duration of `0` effectively releases the reservation by setting `reserved_before` to `NOW`.

Two pairs of endpoints implement this pattern:

- `TryReserve` / `TryBulkReserve` — conditional reservation. Only reserves records whose reservation has expired (`reserved_before < NOW`). Returns which records were successfully reserved. Use this when competing actors should not interfere with each other's work.
- `Reserve` / `BulkReserve` — forceful reservation. Sets `reserved_before` regardless of current state. Returns which records exist. Use this to forcefully extend or reset a reservation.

```go
// Reserve the record for 5 minutes
reserved, err := myserviceapi.NewClient(svc).TryReserve(ctx, objKey, 5*time.Minute)
if err != nil {
	return errors.Trace(err)
}
if !reserved {
	return errors.New("record is already reserved")
}
defer func() {
	// Release the reservation when done
	myserviceapi.NewClient(svc).Reserve(ctx, objKey, 0)
}()
// Perform the operation while holding the reservation...
```

### Configuring the Datasource Name for Testing

Running tests require the microservices under test to be able to connect to the SQL database. The data source name is configured in `config.local.yaml` at the root of the project. If tests fail to connect to the database, prompt the user to update `config.local.yaml` with the appropriate credentials.

```yaml
all:
  SQLDataSourceName: root:root@tcp(127.0.0.1:3306)/
```

Example data source names for each of the supported drivers:
- mysql: `root:root@tcp(127.0.0.1:3306)/`
- pgx: `postgres://postgres:postgres@127.0.0.1:5432/`
- mssql: `sqlserver://sa:sa@127.0.0.1:1433`

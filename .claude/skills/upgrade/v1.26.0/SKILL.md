---
name: Upgrading a Project to v1.26.0
description: Upgrades the project to v1.26.0. Adds workflow scaffolding (Executor, marshalTask) to client.go, task/graph HINTs to intermediate.go, workflow import guards to mock.go, and SQLite variants to SQL migration scripts.
---

## Workflow

Copy this checklist and track your progress:

```
Upgrade a Microbus project to v1.26.0:
- [ ] Step 1: Find all microservices to upgrade
- [ ] Step 2: Upgrade client.go
- [ ] Step 3: Upgrade intermediate.go
- [ ] Step 4: Upgrade mock.go
- [ ] Step 5: Add SQLite variants to SQL migration scripts
- [ ] Step 6: Add SQLite support to SQL CRUD microservices
- [ ] Step 7: Update manifests
```

#### Step 1: Find All Microservices to Upgrade

Find all microservice directories in the project that contain a `*api/client.go` file. Exclude files under `.claude/skills/` - those templates are maintained separately. Also exclude microservices whose `client.go` already contains `type Executor struct` - those are already upgraded.

#### Step 2: Upgrade `client.go`

For each `*api/client.go` file, make the following changes:

Add these imports if not already present:

- `"github.com/microbus-io/fabric/workflow"` - in the Microbus packages group (after other `fabric/` imports)

In the `var` block that contains the import guards (the block with `_ context.Context`, `_ json.Encoder`, etc.), add the following lines after `_ = marshalFunction`:

```go
	_ = marshalTask
	_ = marshalWorkflow
	_ workflow.Flow
```

Append the following code block to the end of the `client.go`:

```go
// WorkflowRunner executes a workflow by name with initial state, blocking until termination.
// foremanapi.Client satisfies this interface.
type WorkflowRunner interface {
	Run(ctx context.Context, workflowName string, initialState any) (status string, state map[string]any, err error)
}

// Executor runs tasks and workflows synchronously, blocking until termination.
// It is primarily intended for integration tests.
type Executor struct {
	svc     service.Publisher
	host    string
	opts    []pub.Option
	inFlow  *workflow.Flow
	outFlow *workflow.Flow
	runner  WorkflowRunner
}

// NewExecutor creates a new executor proxy to the microservice.
func NewExecutor(caller service.Publisher) Executor {
	return Executor{svc: caller, host: Hostname}
}

// ForHost returns a copy of the executor with a different hostname to be applied to requests.
func (_c Executor) ForHost(host string) Executor {
	return Executor{svc: _c.svc, host: host, opts: _c.opts, inFlow: _c.inFlow, outFlow: _c.outFlow, runner: _c.runner}
}

// WithOptions returns a copy of the executor with options to be applied to requests.
func (_c Executor) WithOptions(opts ...pub.Option) Executor {
	return Executor{svc: _c.svc, host: _c.host, opts: append(_c.opts, opts...), inFlow: _c.inFlow, outFlow: _c.outFlow, runner: _c.runner}
}

// WithInputFlow returns a copy of the executor with an input flow to use for task execution.
// The input flow's state is available to the task in addition to the typed input arguments.
func (_c Executor) WithInputFlow(flow *workflow.Flow) Executor {
	return Executor{svc: _c.svc, host: _c.host, opts: _c.opts, inFlow: flow, outFlow: _c.outFlow, runner: _c.runner}
}

// WithOutputFlow returns a copy of the executor with an output flow to populate after task execution.
// The output flow captures the full flow state including control signals (Goto, Retry, Interrupt, Sleep).
func (_c Executor) WithOutputFlow(flow *workflow.Flow) Executor {
	return Executor{svc: _c.svc, host: _c.host, opts: _c.opts, inFlow: _c.inFlow, outFlow: flow, runner: _c.runner}
}

// WithWorkflowRunner returns a copy of the executor with a workflow runner for executing workflows.
// foremanapi.NewClient(svc) satisfies the WorkflowRunner interface.
func (_c Executor) WithWorkflowRunner(runner WorkflowRunner) Executor {
	return Executor{svc: _c.svc, host: _c.host, opts: _c.opts, inFlow: _c.inFlow, outFlow: _c.outFlow, runner: runner}
}

// marshalTask supports task execution via the Executor.
func marshalTask(ctx context.Context, svc service.Publisher, opts []pub.Option, host string, method string, route string, in any, out any, inFlow *workflow.Flow, outFlow *workflow.Flow) (err error) {
	flow := inFlow
	if flow == nil {
		flow = workflow.NewFlow()
	}
	err = flow.SetState(in)
	if err != nil {
		return errors.Trace(err)
	}
	body, err := json.Marshal(flow)
	if err != nil {
		return errors.Trace(err)
	}
	u := httpx.JoinHostAndPath(host, route)
	httpRes, err := svc.Request(
		ctx,
		pub.Method(method),
		pub.URL(u),
		pub.Body(body),
		pub.ContentType("application/json"),
		pub.Options(opts...),
	)
	if err != nil {
		return err // No trace
	}
	flow = workflow.NewFlow()
	err = json.NewDecoder(httpRes.Body).Decode(flow)
	if err != nil {
		return errors.Trace(err)
	}
	if outFlow != nil {
		*outFlow = *flow
	}
	if out != nil {
		err = flow.ParseState(out)
		return errors.Trace(err)
	}
	return nil
}

// marshalWorkflow supports workflow execution via the Executor.
func marshalWorkflow(ctx context.Context, runner WorkflowRunner, workflowURL string, in any, out any) (status string, err error) {
	status, state, err := runner.Run(ctx, workflowURL, in)
	if err != nil {
		return status, err // No trace
	}
	if out != nil && state != nil {
		data, err := json.Marshal(state)
		if err != nil {
			return status, errors.Trace(err)
		}
		err = json.Unmarshal(data, out)
		if err != nil {
			return status, errors.Trace(err)
		}
	}
	return status, nil
}

```

#### Step 3: Upgrade `intermediate.go`

For each microservice's `intermediate.go` file, make the following changes:

Add `"github.com/microbus-io/fabric/workflow"` to the import block if not already present, in the Microbus packages group (after other `fabric/` imports).

In the `var` block that contains the import guards, add the following line at the end (before the closing `)`):

```go
	_ *workflow.Flow
```

In the `NewIntermediate` constructor, add the following two comment lines after `// HINT: Add inbound event sinks here` (and after any existing inbound event sink subscriptions):

```go
	// HINT: Add task endpoints here

	// HINT: Add graph endpoints here
```

#### Step 4: Upgrade `mock.go`

For each microservice's `mock.go` file, make the following changes:

Add the following imports to the import block if not already present, in the Microbus packages group (after other `fabric/` imports, before the blank line and local imports):

- `"encoding/json"` - in the standard library group
- `"github.com/microbus-io/fabric/httpx"`
- `"github.com/microbus-io/fabric/utils"`
- `"github.com/microbus-io/fabric/workflow"`

In the `var` block that contains the import guards, add the following lines before the `*api.Client` guard:

```go
	_ json.Encoder
	_ httpx.BodyReader
	_ = utils.RandomIdentifier
	_ *workflow.Flow
```

#### Step 5: Add SQLite Variants to SQL Migration Scripts

Starting with v1.26.0, the `sequel` package supports SQLite as a fourth database driver (`sqlite`). All existing SQL migration scripts must be updated to include `-- DRIVER: sqlite` variants so that tests can run against an in-memory SQLite database without requiring a MySQL, PostgreSQL, or SQL Server instance.

Find all `.sql` files under `resources/sql/` directories in the project. Exclude files under `.claude/skills/` - those templates are maintained separately. Skip any `.sql` file that already contains `-- DRIVER: sqlite`.

For each migration script, read the file and append `-- DRIVER: sqlite` variants for every statement. Use the MySQL (`-- DRIVER: mysql`) variant as the reference and apply the following type mappings:

**Column types:**

| MySQL | SQLite |
|---|---|
| `BIGINT` | `INTEGER` |
| `INT` | `INTEGER` |
| `TINYINT` | `INTEGER` |
| `SMALLINT` | `INTEGER` |
| `BIGINT NOT NULL AUTO_INCREMENT` | `INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT` |
| `VARCHAR(n)` | `TEXT` |
| `CHAR(n)` | `TEXT` |
| `TEXT` | `TEXT` |
| `MEDIUMTEXT` | `TEXT` |
| `LONGTEXT` | `TEXT` |
| `JSON` | `TEXT` |
| `BLOB` | `BLOB` |
| `MEDIUMBLOB` | `BLOB` |
| `LONGBLOB` | `BLOB` |
| `DATETIME(n)` | `DATETIME` |
| `DATETIME` | `DATETIME` |
| `BOOL` | `INTEGER` |

**Structural differences:**

- SQLite uses `INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT` for auto-increment columns. When the auto-increment column is the primary key, the `PRIMARY KEY` constraint must be on the column itself (inline), not in a separate constraint clause.
- If the MySQL variant uses a composite primary key where the auto-increment column is not the first column (e.g. `PRIMARY KEY (tenant_id, id)` with `id AUTO_INCREMENT`), the SQLite variant must instead use `INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT` on the `id` column inline and omit the composite `PRIMARY KEY` constraint. Add a separate `CREATE UNIQUE INDEX` to enforce the composite uniqueness.
- Inline index definitions inside `CREATE TABLE` (e.g. `INDEX idx_name (col)`) are not supported in SQLite. Convert them to separate `CREATE INDEX` statements, each prefixed with `-- DRIVER: sqlite`.
- `UNIQUE INDEX` inside `CREATE TABLE` must also be converted to separate `CREATE UNIQUE INDEX` statements.
- Named `CONSTRAINT` clauses for primary keys (e.g. `CONSTRAINT pk_name PRIMARY KEY (...)`) should be simplified to just `PRIMARY KEY (...)`, or moved inline to the column if it is the auto-increment column.
- `IF NOT EXISTS` is supported by SQLite for both `CREATE TABLE` and `CREATE INDEX`.
- Partial indexes with `WHERE` clauses are supported by SQLite.

**ALTER TABLE statements:**

SQLite has limited `ALTER TABLE` support. Only one column can be added per `ALTER TABLE` statement. If the MySQL variant adds multiple columns in a single `ALTER TABLE`, split them into separate `ALTER TABLE ... ADD COLUMN` statements for SQLite, each terminated by `;` and a newline.

```sql
-- DRIVER: sqlite
ALTER TABLE my_noun ADD COLUMN my_field_integer INTEGER NOT NULL DEFAULT 0;
-- DRIVER: sqlite
ALTER TABLE my_noun ADD COLUMN my_field_nullable TEXT;
-- DRIVER: sqlite
ALTER TABLE my_noun ADD COLUMN my_field_time DATETIME;
```

**Index statements:**

`CREATE INDEX` and `CREATE UNIQUE INDEX` statements translate directly. The `USING btree` clause (PostgreSQL-specific) should be omitted. Partial indexes with `WHERE` clauses are supported.

```sql
-- DRIVER: sqlite
CREATE INDEX my_noun_idx_my_field ON my_noun (tenant_id, my_field);

-- DRIVER: sqlite
CREATE UNIQUE INDEX my_noun_idx_email ON my_noun (tenant_id, email);
```

**Example - CREATE TABLE:**

Given the MySQL variant:
```sql
-- DRIVER: mysql
CREATE TABLE person (
	tenant_id INT NOT NULL,
	id BIGINT NOT NULL AUTO_INCREMENT,
	revision BIGINT NOT NULL DEFAULT 0,
	first_name VARCHAR(64) NOT NULL DEFAULT '',
	tags JSON NULL,
	created_at DATETIME(3) NOT NULL,
	updated_at DATETIME(3) NOT NULL,

	CONSTRAINT person_pk PRIMARY KEY (tenant_id, id),
	UNIQUE INDEX person_idx_id (id),
	INDEX person_idx_created_at (tenant_id, created_at)
);
```

The SQLite variant should be:
```sql
-- DRIVER: sqlite
CREATE TABLE person (
	tenant_id INTEGER NOT NULL,
	id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
	revision INTEGER NOT NULL DEFAULT 0,
	first_name TEXT NOT NULL DEFAULT '',
	tags TEXT,
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL
);
-- DRIVER: sqlite
CREATE UNIQUE INDEX person_idx_tenant_id ON person (tenant_id, id);
-- DRIVER: sqlite
CREATE UNIQUE INDEX person_idx_id ON person (id);
-- DRIVER: sqlite
CREATE INDEX person_idx_created_at ON person (tenant_id, created_at);
```

#### Step 6: Add SQLite Support to SQL CRUD Microservices

Find all SQL CRUD microservices in the project - these are microservices whose `service.go` contains `svc.db.DriverName()` switch statements. Exclude files under `.claude/skills/`. Apply the following changes to each microservice's `service.go` and `service_test.go`.

**service.go - Add SQLite to RETURNING id paths:**

SQLite supports `RETURNING` like PostgreSQL. In every `switch svc.db.DriverName()` or `if svc.db.DriverName() == "pgx"` that appends `RETURNING id` or uses a `RETURNING`-based query path, add `"sqlite"` alongside `"pgx"`:

- `case "pgx":` → `case "pgx", "sqlite":`
- `svc.db.DriverName() == "pgx"` → `svc.db.DriverName() == "pgx" || svc.db.DriverName() == "sqlite"`

This applies to:
- `BulkDelete` - the `case "pgx":` that builds `DELETE ... RETURNING id`
- `bulkUpdate` (BulkStore/BulkRevise) - both the `if ... == "pgx"` that appends `RETURNING id` to the UPDATE statement, and the `case "pgx":` in the execution switch
- `BulkCreate` - the `case "pgx":` that appends `RETURNING id` to the INSERT statement
- `bulkReserve` (TryBulkReserve/BulkReserve) - the `case "pgx":` that builds `UPDATE ... RETURNING id`

Do **not** add `"sqlite"` to `case "mysql":` paths that use `FOR UPDATE` or `LastInsertId` - SQLite should fall through to the `RETURNING`-based default/pgx path instead.

**service_test.go - Add sleep before Store assertions on updated_at:**

SQLite timestamps have millisecond precision. Tests that assert `modifiedObj.UpdatedAt.After(originalObj.UpdatedAt)` can fail if both operations complete within the same millisecond. Add `time.Sleep(2 * time.Millisecond)` before calls to `client.Store` in test cases that subsequently assert `UpdatedAt` advanced. There are typically two places:

1. In the `create_and_store` subtest of `TestMyNoun_Store`
2. In `TestMyNoun_ColumnMappings`

Search for `UpdatedAt.After` in `service_test.go` to locate the exact positions.

#### Step 7: Update Manifests

Update the `frameworkVersion` in all `manifest.yaml` files in the project to `1.26.0`.

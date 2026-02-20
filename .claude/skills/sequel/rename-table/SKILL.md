---
name: Renaming the database table in a SQL CRUD microservice
description: Renames the database table used by a SQL CRUD microservice to persist objects. Use when explicitly asked by the user to rename the table.
---

**CRITICAL**: Read and analyze this microservice before starting. Do NOT explore or analyze other microservices unless explicitly instructed to do so. The instructions in this skill are self-contained to this microservice.

**IMPORTANT**: `old_table_name` and `new_table_name` are placeholders for the actual old and new table names.

## Workflow

Copy this checklist and track your progress:

```
Renaming the database table:
- [ ] Step 1: Update Table Name Const
- [ ] Step 2: Update Database Schema
- [ ] Step 3: Housekeeping
```

#### Step 1: Update Table Name Const

Update the `tableName` const in `service.go` to the new table name. Do NOT change the `sequenceName`.

```go
const (
	tableName        = "new_table_name"
	sequenceName     = "old_table_name@0f7ce540" // Do not change
	// ...
)
```

#### Step 2: Update Database Schema

Create a new migration script file in `resources/sql` with an incremental file name. **IMPORTANT**: Do not edit an existing migration file.

```sql
-- DRIVER: mysql
RENAME TABLE old_table_name TO new_table_name;

-- DRIVER: pgx
ALTER TABLE old_table_name RENAME TO new_table_name;

-- DRIVER: mssql
EXEC sp_rename 'old_table_name', 'new_table_name';
```

Refer to the older `.sql` migration files to identify what if any indices were associated with the old table name. Append statements to the migration script to rename these indices, replacing the old table name with the new one.

```sql
-- DRIVER: mysql
ALTER TABLE new_table_name RENAME INDEX old_table_name_idx_field TO new_table_name_idx_field;

-- DRIVER: pgx
ALTER INDEX old_table_name_idx_field RENAME TO new_table_name_idx_field;

-- DRIVER: mssql
EXEC sp_rename 'old_table_name_idx_field', 'new_table_name_idx_field', 'INDEX';
```

#### Step 3: Housekeeping

Follow the `microbus/housekeeping` skill. Skip the manifest, topology and tidy up steps.

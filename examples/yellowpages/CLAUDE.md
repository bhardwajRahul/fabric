# yellowpages.example

## Agent Instructions

This microservice is a SQL CRUD microservice. See `.claude/rules/sequel.txt` for the conventions.

This microservice does not maintain a `PROMPTS.md`. Skip the prompts step when running housekeeping.

## Purpose

The `yellowpages.example` microservice is a SQL CRUD service that persists `Person` objects in a relational database. It exposes standard CRUD operations (Create, Store, Delete, Load, List) along with bulk variants and a REST API. The service supports MySQL, PostgreSQL, and SQL Server via the sequel library.

### Person Fields

- `FirstName` (string, required, max 64 runes, trimmed, searchable)
- `LastName` (string, required, max 64 runes, trimmed, searchable)
- `Email` (string, required, max 256 runes, unique, trimmed, searchable)
- `Birthday` (time.Time, must be in the past if set)

The email column has a unique index (`person_idx_email`) scoped by tenant. Query supports filtering by FirstName, LastName, and Email.

## Create yellowpages microservice

Create a SQL CRUD microservice named "yellowpages" in the `examples/yellowpages/` directory with hostname `yellowpages.example`. The object is called `Person`. The microservice was scaffolded using the `sequel/add-microservice` skill template. Tests were skipped for speed.

## Add Person fields

Add fields to the Person object: FirstName (string, required, max 64 runes), LastName (string, required, max 64 runes), Email (string, required, max 256 runes, unique), and Birthday (time.Time, must be in the past). All string fields are trimmed and searchable. Query supports filtering by FirstName, LastName, and Email. An index was added on the email column.

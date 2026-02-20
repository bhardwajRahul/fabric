# Package `examples/yellowpages`

The yellow pages microservice is an example of a SQL CRUD microservice that persists `Person` records in a SQL database.

### Build with Skills

SQL CRUD microservices are first-class citizens of the Microbus framework. Rather than manually adding SQL support, defining endpoints, and writing boilerplate, the dedicated `sequel` skills generate a fully functional CRUD microservice in minutes:

> HEY CLAUDE...
>
> Create a SQL CRUD microservice to store Persons. Use the hostname yellowpages.example and create it in examples/yellowpages/. Skip tests.
>
> Add the fields: FirstName (required, max 64 runes), LastName (required, max 64 runes), Email (required, max 256 runes, unique) and Birthday (time.Time, must be in the past).

The generated microservice includes a comprehensive set of endpoints out of the box:

- **CRUD operations** — `Create`, `Store`, `Delete`, `Load`, `List`, `Lookup` (and their `Must` variants that error instead of returning not-found)
- **Bulk operations** — `BulkCreate`, `BulkStore`, `BulkRevise`, `BulkLoad`, `BulkDelete`
- **Query operations** — `List`, `Lookup`, `Count`, `Purge`
- **Revision control** — `Revise`, `MustRevise`, `BulkRevise` for optimistic concurrency
- **REST API** — Standard REST endpoints at `/persons` and `/persons/{key}`

### Connecting to the Database

This example requires a MariaDB database instance. If you don't already have one installed, you can add it to Docker using:

```shell
docker pull mariadb
docker run -p 3306:3306 --name mariadb-1 -e MARIADB_ROOT_PASSWORD=root -d mariadb
```

Next, create a database named `microbus`.

From the `Exec` panel of the `mariadb-1` container, type:

```shell
mysql -uroot -proot
```

And then use the SQL command prompt to create the database:

```sql
CREATE DATABASE microbus;
```

Set the connection string to the database in `config.local.yaml` at the root of the project.

```yaml
all:
  SQLDataSourceName: "root:root@tcp(127.0.0.1:3306)/microbus"
```

### Web UI

The yellow pages microservice includes a `WebUI` web endpoint that provides a simple browser-based form supporting `GET`, `POST`, `PUT` and `DELETE` requests. This is useful because RESTful APIs leverage HTTP methods other than just `GET`, which are impossible to call directly from the browser's address bar.

Open the web UI at http://localhost:8080/yellowpages.example/web-ui

To create a new person, `POST` to `/persons`:

```json
{
    "firstName": "Harry",
    "lastName": "Potter",
    "email": "harry.potter@hogwarts.edu.wiz",
    "birthday": "1980-07-31T00:00:00Z"
}
```

The server will respond with the new person's key:

```json
{
    "objKey": "a1b2c3"
}
```

To update a person, `PUT` to `/persons/{key}` using the key returned from the create:

```json
{
    "firstName": "Harry",
    "lastName": "Potter",
    "email": "harry.potter@hogwarts.edu.wiz",
    "birthday": "1980-07-31T00:00:00Z"
}
```

To list all persons, `GET` from `/persons`.

To query by email, `GET` from `/persons?q.Email=harry.potter@hogwarts.edu.wiz`.

To load a specific person, `GET` from `/persons/{key}`.

To delete a person, `DELETE` at `/persons/{key}`.

### OpenAPI

Alternatively, use the OpenAPI document of the microservice to interact with the yellow pages microservice. Fetch the OpenAPI document at:

http://localhost:8080/yellowpages.example/openapi.json

Copy the JSON and paste it at https://editor-next.swagger.io to parse it. You'll see all endpoints of the microservice listed to the right-hand side. Click on any of them to expand. Press the `Try it out` button, enter the appropriate data, and `Execute`.

---
name: add-sql-microservice
description: TRIGGER when user asks to create a CRUD microservice, database-backed service, or a service to persist/store objects in SQL (MySQL, Postgres, MSSQL).
---

**CRITICAL**: Do NOT explore or analyze other microservices unless explicitly instructed to do so. The instructions in this skill are self-contained to this microservice.

**IMPORTANT**: `MyNoun`, `MyNounKey`, `mynoun`, and `mynounapi` are placeholders for the actual object, its key, directory, and API package of the microservice.

**IMPORTANT**: Do not remove the `Example` field or code related to it from the code since it is required by various tests.

**IMPORTANT**: Read `.claude/rules/sequel.txt` for SQL CRUD conventions before proceeding.

**CRITICAL**: The `busstop` template ships only the hand-written source files - `mynounapi/definition.go` (the API spec), the domain types, `service.go`, the tests, and `resources/`. After the copy, `cmd/genservice` produces the boilerplate (Step 5).

## Workflow

Copy this checklist and track your progress:

```
Creating a new microservice:
- [ ] Step 1: Determine the Noun and Hostname
- [ ] Step 2: Create a Directory Structure for the New Microservice
- [ ] Step 3: Copy Template Files
- [ ] Step 4: Find and Replace
- [ ] Step 5: Generate the Boilerplate
- [ ] Step 6: Add to Main App
- [ ] Step 7: Add to config file
- [ ] Step 8: Housekeeping
- [ ] Step 9: Propose Object Fields
```

#### Step 1: Determine the Noun and Hostname

Determine the singular noun representing the object being persisted, for example, "User", "Notebook", "Sales Order", etc. In subsequent steps, `mynoun` is used as a placeholder for the lowercase form of this noun with spaces removed (e.g. `user`, `notebook`, `salesorder`).

Determine the hostname by following the `name-microservice` skill (e.g. `mynoun.path.some.myproject`). A CRUD store's leading segment is normally the object noun, and may fold in `crud` when the microservice is dedicated purely to persistence (e.g. `orderscrud`).

#### Step 2: Create a Directory Structure for the New Microservice

Each microservice must be placed in a separate directory. Create a new directory for the new microservice.
For the directory name, use `mynoun` with only lowercase letters `a` through `z`, for example, `user`, `notebook` or `salesorder`.

In smaller projects, place the new directory under the root directory of the project.
In larger projects, consider using a nested directory structure to group similar microservices together.

```shell
mkdir -p mynoun
```

#### Step 3: Copy Template Files

The `.claude/skills/sequel/add-sql-microservice/busstop` directory contains the canonical implementation code for the noun `BusStop`. Use `cp` or a similar tool to copy verbatim the content of the `busstop` template directory to the microservice's directory `mynoun`. Do not read the files.

The directory structure should look like this.

```
myproject/
└── mynoun/
    ├── busstopapi/
    └── resources/
```

**IMPORTANT**: File names in the following steps are relative to the new microservice directory `mynoun`, unless indicated otherwise.

Rename the directory `busstopapi` to `mynounapi`.

The directory structure should look like this.

```
myproject/
└── mynoun/
    ├── mynounapi/
    └── resources/
```

#### Step 4: Find and Replace

**CRITICAL**: This step must be scoped to the microservice directory only. Do not perform it on the project root.

Use `sed` or a similar tool to perform the following **case-sensitive** find-and-replace operations on ALL files in the microservice directory. Do not read the files.

Perform these replacements in order:

1. Replace `github.com/microbus-io/fabric/busstop` with the package path of this microservice, e.g. `github.com/mycompany/myproject/mynoun`
2. Replace `busstopapi` with the name of the API directory `mynounapi`
3. Replace `busstop.hostname` with the hostname of this microservice
4. Replace `BusStop` with the singular noun of this microservice in PascalCase, i.e. `MyNoun`
5. Replace `bus stops` with the plural noun of this microservice in lower case (with spaces between words), i.e. `my nouns`
6. Replace `bus stop` with the singular noun of this microservice in lower case (with spaces between words), i.e. `my noun`
7. Replace `busstop` with the singular noun of this microservice in lowercase (no spaces between words), i.e. `mynoun`
8. Replace `bus_stop` with the singular noun of this microservice in snake_case, i.e. `my_noun`
9. Replace `bus-stop` with the singular noun of this microservice in kebab-case, i.e. `my-noun`
10. Replace `_CIPHER_KEY_____________________` with a unique 32-character random base64 string (characters `A-Z`, `a-z`, `0-9`, `+`, `/`)
11. Replace `_CIPHER_NONCE___________________` with a 32-character random base64 string (characters `A-Z`, `a-z`, `0-9`, `+`, `/`)
12. Replace `_SEQUENCE_` with an 8-character random hexadecimal string

**IMPORTANT**: The cipher key and nonce are base64 and may contain the `/` character, so a `sed`
substitution using the default `/` delimiter (`s/old/new/`) can break. Use an alternate delimiter
that cannot appear in base64, e.g. `sed 's|_CIPHER_KEY_____________________|...|g'`, or apply those
two replacements with a tool that takes the replacement as a literal argument rather than parsing it.

Run `go fmt` on the microservice directory to reformat the Go source files.

#### Step 5: Generate the Boilerplate

From the new microservice's directory, run the generator. It reads `mynounapi/definition.go` and writes `mynounapi/client.go`, `intermediate.go`, `mock.go`, `mock_test.go`, and `manifest.yaml`.

```shell
go run github.com/microbus-io/fabric/cmd/genservice .
```

Then, from the project root, bring the module's dependencies up to date and verify the microservice compiles:

```shell
go mod tidy
go vet ./...
```

Run `go mod tidy` first: a SQL CRUD microservice pulls the `sequel` package and the database drivers, which may not yet be in `go.sum`, which makes `go vet` fail with `missing go.sum entry` until the module is tidied.

#### Step 6: Add to Main App

Find `main/main.go` relative to the project root. Add the new microservice to the app in the `main` function. Add the appropriate import statement at the top of the file.

```go
import (
	// ...
	"github.com/mycompany/myproject/mynoun"
)

func main() {
	// ...
	app.Add(
		// HINT: Add solution microservices here
		mynoun.NewService(),
	)
	// ...
}
```

#### Step 7: Add to Config File

Look for `config.local.yaml` at the root of the project. If the file does not exist, create it.

If a value already exists for `SQLDataSourceName` under `all` in `config.local.yaml`, skip the remainder of this step.

Add the data source name secrets under `all`.

```yaml
all:
  # SQLDataSourceName: root:root@tcp(127.0.0.1:3306)/microbus
  # SQLDataSourceName: postgres://postgres:postgres@127.0.0.1:5432/microbus
  # SQLDataSourceName: sqlserver://sa:Password123@127.0.0.1:1433?database=microbus
```

#### Step 8: Housekeeping

Follow the `housekeeping` skill.

#### Step 9: Propose Object Fields

Ask the user if they'd like you to propose a design for the microservice. If the user declines, skip the remainder of this step.

Use any of the context provided by the user and propose a list of fields that the object should include, with the following properties. You may engage the user and ask for additional information if needed.

- Name
- Description
- Go data type, e.g. `string`, `int`, `time.Time`, etc.
- Validation rules such as whether or not the field is required, maximum length (if string), acceptable range (if numeric), etc.
- Whether or not it is a unique identifier of the object

Do not recommend `CreatedAt` or `UpdatedAt` timestamp fields. These are already built-in.

Save the proposed design to `DESIGN.md`, then show it to the user. Ask the user if they'd like you to implement any of the proprosals. Do not implement without explicit approval from the user.

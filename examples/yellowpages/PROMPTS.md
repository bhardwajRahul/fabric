# Prompts

## sequel/add-microservice

Create a SQL CRUD microservice named "yellowpages" in the `examples/yellowpages/` directory with hostname `yellowpages.example` to persist the noun "Person". Use the `sequel/add-microservice` skill to scaffold the microservice from the busstop template.

## sequel/add-fields

Add fields to the Person object: FirstName (string, required, max 64 runes), LastName (string, required, max 64 runes), Email (string, required, max 256 runes, unique), and Birthday (time.Time, must be in the past). All string fields are trimmed and searchable. Query supports filtering by FirstName, LastName, and Email. An index was added on the email column.

## microbus/add-web

Add a web endpoint `WebUI` at route `/web-ui` using method `ANY`.

## Implement WebUI

Implement the `WebUI` handler and `webui.html` template as a self-contained REST API explorer for the Person microservice.

**Handler (`service.go`):** On POST, the handler reads form values `method`, `path`, and `body`, then makes an internal `svc.Request` to `https://{Hostname}/{path}` using the selected HTTP method. For POST/PUT methods, the body form value is sent as `application/json`; for GET/DELETE it is omitted. The response status code and body (or error with `%+v` formatting) are captured. The handler renders `webui.html` with a data struct containing `Method`, `Path`, `Body` (to repopulate the form), `StatusCode`, and `Response`.

**Template (`resources/webui.html`):** A single-page HTML form with:
- A reference list of the REST API routes (GET/POST/PUT/DELETE `/persons` and `/persons/{key}`, plus GET `/persons?q.Email={email}`)
- A form (method POST) containing: a `<select>` for HTTP method (GET/POST/PUT/DELETE) with the previous selection preserved via `{{ if eq .Method "..." }}selected{{ end }}`; a text input for the path (placeholder `/persons/...`); a `<textarea>` for the JSON body; and a submit button
- JavaScript that toggles the body textarea visibility based on the selected method (hidden for GET/DELETE, visible for POST/PUT), running on page load and on method change
- Below the form: the response status code and a `<pre>` block with the response body

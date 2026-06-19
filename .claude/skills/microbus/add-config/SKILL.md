---
name: add-config
description: TRIGGER when user asks to add or modify a configuration property or setting, or to make a value configurable.
---

**CRITICAL**: Do NOT explore or analyze other microservices unless explicitly instructed to do so. The instructions in this skill are self-contained to this microservice.

**CRITICAL**: A config property is declared as a `define.Config` var in `<name>api/definition.go`; its callback, if any, is implemented in `service.go`. Add the declaration and run `cmd/genservice`.

**CRITICAL**: Keep the `// MARKER: Name` comment on the `define.Config` var.

## Workflow

Copy this checklist and track your progress:

```
Creating or modifying a configuration property:
- [ ] Step 1: Read local CLAUDE.md file
- [ ] Step 2: Determine the type
- [ ] Step 3: Determine the properties
- [ ] Step 4: Declare the config in definition.go
- [ ] Step 5: Implement the callback
- [ ] Step 6: Generate the boilerplate
- [ ] Step 7: Use the config
- [ ] Step 8: Test the callback
- [ ] Step 9: Add to config file
- [ ] Step 10: Housekeeping
```

#### Step 1: Read Local `CLAUDE.md` File

Read the local `CLAUDE.md` file in the microservice's directory. It contains microservice-specific instructions that should take precedence over global instructions.

#### Step 2: Determine the Type

Determine the type of the configuration property. It must be one of `string`, `int`, `bool`, `float64`, or `time.Duration`.

#### Step 3: Determine the Properties

Determine the properties of the configuration property:
- **Description**: explains the purpose of the property. It becomes the godoc on the `define.Config` var and starts with the property name
- **Default value**: an optional default, always written as a string (it flows through the same validate-and-convert path as a configured value)
- **Validation**: an optional validation rule (see below)
- **Secret**: whether the value is a secret that should not be logged
- **Callback**: whether `OnChanged<Name>` should fire when the value changes, for example to reopen a connection to an external resource

Validation rules can be any of the following:
- `str` followed by a regexp: `str ^[a-zA-Z0-9]+$`
- `bool`
- `int` followed by an open, closed or mixed interval: `int [0,60]`
- `float` followed by an open, closed or mixed interval: `float [0.0,1.0)`
- `dur` followed by an open, closed or mixed interval of Go durations: `dur (0s,24h]`
- `set` followed by a pipe-separated list of values: `set Red|Green|Blue`
- `url`
- `email`
- `json`

#### Step 4: Declare the Config in `definition.go`

Append the `define.Config` var to `myserviceapi/definition.go`. The godoc is the description from Step 3.

```go
// MyConfig is X.
var MyConfig = define.Config{ // MARKER: MyConfig
	Value:      int(0),
	Default:    "1",
	Validation: "int (1,100]",
	Secret:     true,
	Callback:   true,
}
```

- `Value` is a type carrier declaring the property's type: `string("")`, `int(0)`, `bool(false)`, `float64(0)`, or `time.Duration(0)`. The generator reads its type, never the value
- For a `time.Duration` config, add the `"time"` import to `definition.go`
- `Default` is the optional default as a string; omit when there is none
- `Validation` is the optional rule from Step 3; omit when there is none
- `Secret: true` marks a value that is never logged; omit when false
- `Callback: true` makes `OnChanged<Name>` fire on change; omit when false

#### Step 5: Implement the Callback

Skip this step if the config does not have a callback.

Define the callback in `service.go`. The generator adds it to the `ToDo` interface and wires the change dispatcher; you only write the handler.

```go
// OnChangedMyConfig is called when the MyConfig config property changes.
func (svc *Service) OnChangedMyConfig(ctx context.Context) (err error) { // MARKER: MyConfig
	// Implement handling of the new value here
	return nil
}
```

#### Step 6: Generate the Boilerplate

From the microservice's directory, run the generator. It regenerates `intermediate.go` (the getter, setter, validation, and change dispatcher), `mock.go`, `mock_test.go`, and `manifest.yaml` from the updated `definition.go`.

```shell
go run github.com/microbus-io/fabric/cmd/genservice .
```

Then verify the microservice compiles with `go vet ./...` from the project root.

#### Step 7: Use the Config

The config has no implementation of its own. Read its value from within other endpoints using the generated getter.

```go
myConfig := svc.MyConfig()
```

Use `svc.SetMyConfig(value)` to set the value programmatically, for example in tests.

#### Step 8: Test the Callback

Skip this step if the config does not have a callback.

Append the integration test to `service_test.go`.

```go
func TestMyService_OnChangedMyConfig(t *testing.T) { // MARKER: MyConfig
	t.Parallel()
	ctx := t.Context()
	_ = ctx

	// Initialize the microservice under test
	svc := NewService()

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
	)
	app.RunInTest(t)

	/*
		HINT: Use the following pattern for each test case

		t.Run("test_case_name", func(t *testing.T) {
			assert := testarossa.For(t)

			err := svc.SetMyConfig(newValue)
			assert.NoError(err)
		})
	*/
}
```

Skip the remainder of this step if instructed to be "quick" or to skip tests.

Insert test cases at the bottom of the integration test function using the recommended pattern. Do not remove the `HINT` comments.

```go
t.Run("test_case_name", func(t *testing.T) {
	assert := testarossa.For(t)

	err := svc.SetMyConfig(newValue)
	assert.NoError(err)
})
```

#### Step 9: Add to Config File

Add a commented-out entry for the new configuration property to the appropriate config file at the root of the project, nested under the hostname of the microservice. Use the default value if one was defined, or leave it blank otherwise.

If the config is secret, add it to `config.local.yaml`. If the config is not secret, add it to `config.yaml`. Create the file if it does not exist.

If a section for the hostname already exists in the file, add the new property to that section. Otherwise, create a new section.

```yaml
my.service.hostname:
  # MyConfig: default
```

#### Step 10: Housekeeping

Follow the `housekeeping` skill.

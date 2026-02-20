---
name: Adding a Configuration Property
description: Creates or modify a configuration property of a microservice. Use when explicitly asked by the user to create or modify a configuration property of a microservice, or when it makes sense to externalize a certain setting of the microservice.
---

**CRITICAL**: Do NOT explore or analyze other microservices unless explicitly instructed to do so. The instructions in this skill are self-contained to this microservice.

**CRITICAL**: Do not omit the `MARKER` comments when generating the code. They are intended as waypoints for future edits.

## Workflow

Copy this checklist and track your progress:

```
Creating or modifying a configuration property:
- [ ] Step 1: Read local AGENTS.md file
- [ ] Step 2: Determine type
- [ ] Step 3: Determine properties
- [ ] Step 4: Extend the ToDo interface
- [ ] Step 5: Define the config
- [ ] Step 6: Implement the getter and setter
- [ ] Step 7: Wire up the config change dispatcher
- [ ] Step 8: Implement the callback
- [ ] Step 9: Use the config
- [ ] Step 10: Extend the mock
- [ ] Step 11: Test the callback
- [ ] Step 12: Housekeeping
```

#### Step 1: Read Local `AGENTS.md` File

Read the local `AGENTS.md` file in the microservice's directory. It contains microservice-specific instructions that should take precedence over global instructions.

#### Step 2: Determine Type

Determine the return type of the configuration property. It must be one of:
- `string`
- `int`
- `bool`
- `float64`
- `time.Duration`

#### Step 3: Determine Properties

Determine the properties of the configuration property:
- **Description**: explains the purpose of the property. It should start with the name of the property
- **Default value**: a default value for the property (optional)
- **Validation**: an optional validation rule (see below)
- **Secret**: whether the value is a secret that should not be logged
- **Callback**: whether a callback should be triggered when the value changes, for example to reopen a connection to an external resource

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

#### Step 4: Extend the `ToDo` Interface

Skip this step if the config does not have a callback.

Extend the `ToDo` interface in `intermediate.go`.

```go
type ToDo interface {
	// ...
	OnChangedMyConfig(ctx context.Context) (err error) // MARKER: MyConfig
}
```

#### Step 5: Define the Config

Define the configuration property in `NewIntermediate` in `intermediate.go`, after the corresponding `HINT` comment.

- Include `cfg.Description`, `cfg.DefaultValue`, `cfg.Validation`, and `cfg.Secret` as appropriate
- Omit options that are empty or false
- The config name is PascalCase
- In `cfg.Description`, replace `MyConfig is X` with the description of the configuration property

```go
func NewIntermediate(impl ToDo) *Intermediate {
	// ...
	svc.DefineConfig( // MARKER: MyConfig
		"MyConfig",
		cfg.Description(`MyConfig is X.`),
		cfg.DefaultValue("1"),
		cfg.Validation("int (1,100]"),
		cfg.Secret(),
	)
}
```

#### Step 6: Implement the Getter and Setter

Append the getter and setter methods to `intermediate.go`.

- Set an appropriate comment describing the config property
- The getter converts the string value to the appropriate type
- The setter converts the value to a string using type-specific code

If the config type is `string`:

```go
/*
MyConfig is X.
*/
func (svc *Intermediate) MyConfig() (value string) { // MARKER: MyConfig
	return svc.Config("MyConfig")
}

/*
SetMyConfig sets the value of the configuration property.
*/
func (svc *Intermediate) SetMyConfig(value string) (err error) { // MARKER: MyConfig
	return svc.SetConfig("MyConfig", value)
}
```

If the config type is `int`:

```go
/*
MyConfig is X.
*/
func (svc *Intermediate) MyConfig() (value int) { // MARKER: MyConfig
	_val := svc.Config("MyConfig")
	_i, _ := strconv.ParseInt(_val, 10, 64)
	return int(_i)
}

/*
SetMyConfig sets the value of the configuration property.
*/
func (svc *Intermediate) SetMyConfig(value int) (err error) { // MARKER: MyConfig
	return svc.SetConfig("MyConfig", strconv.Itoa(value))
}
```

If the config type is `bool`:

```go
/*
MyConfig is X.
*/
func (svc *Intermediate) MyConfig() (value bool) { // MARKER: MyConfig
	_val := svc.Config("MyConfig")
	_b, _ := strconv.ParseBool(_val)
	return _b
}

/*
SetMyConfig sets the value of the configuration property.
*/
func (svc *Intermediate) SetMyConfig(value bool) (err error) { // MARKER: MyConfig
	return svc.SetConfig("MyConfig", strconv.FormatBool(value))
}
```

If the config type is `float64`:

```go
/*
MyConfig is X.
*/
func (svc *Intermediate) MyConfig() (value float64) { // MARKER: MyConfig
	_val := svc.Config("MyConfig")
	_f, _ := strconv.ParseFloat(_val, 64)
	return _f
}

/*
SetMyConfig sets the value of the configuration property.
*/
func (svc *Intermediate) SetMyConfig(value float64) (err error) { // MARKER: MyConfig
	return svc.SetConfig("MyConfig", strconv.FormatFloat(value, 'f', -1, 64))
}
```

If the config type is `time.Duration`:

```go
/*
MyConfig is X.
*/
func (svc *Intermediate) MyConfig() (value time.Duration) { // MARKER: MyConfig
	_val := svc.Config("MyConfig")
	_dur, _ := time.ParseDuration(_val)
	return _dur
}

/*
SetMyConfig sets the value of the configuration property.
*/
func (svc *Intermediate) SetMyConfig(value time.Duration) (err error) { // MARKER: MyConfig
	return svc.SetConfig("MyConfig", value.String())
}
```

#### Step 7: Wire Up the Config Change Dispatcher

Skip this step if the config does not have a callback.

Add a dispatch entry in `doOnConfigChanged` in `intermediate.go`.

```go
// doOnConfigChanged is called when the config of the microservice changes.
func (svc *Intermediate) doOnConfigChanged(ctx context.Context, changed func(string) bool) (err error) {
	// ...
	if changed("MyConfig") { // MARKER: MyConfig
		err = svc.OnChangedMyConfig(ctx)
	}
	return err
}
```

#### Step 8: Implement the Callback

Skip this step if the config does not have a callback.

Define a callback in `service.go` that handles the change.

```go
func (svc *Service) OnChangedMyConfig(ctx context.Context) (err error) { // MARKER: MyConfig
	// Implement handling of the new value here
	return nil
}
```

#### Step 9: Use the Config

The config itself does not have an implementation. Rather, read its value from within other endpoints using its getter.

```go
myConfig := svc.MyConfig()
```

Use `svc.SetMyConfig(value)` to set the value programmatically, for example in tests.

#### Step 10: Extend the Mock

Skip this step if the config does not have a callback.

Add a field to the `Mock` structure definition in `mock.go` to hold a mock handler.

```go
type Mock struct {
	// ...
	mockOnChangedMyConfig func(ctx context.Context) (err error) // MARKER: MyConfig
}
```

Add the stub to the `Mock`.

```go
// MockOnChangedMyConfig sets up a mock handler for OnChangedMyConfig.
func (svc *Mock) MockOnChangedMyConfig(handler func(ctx context.Context) (err error)) *Mock { // MARKER: MyConfig
	svc.mockOnChangedMyConfig = handler
	return svc
}

// OnChangedMyConfig executes the mock handler.
func (svc *Mock) OnChangedMyConfig(ctx context.Context) (err error) { // MARKER: MyConfig
	if svc.mockOnChangedMyConfig == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	err = svc.mockOnChangedMyConfig(ctx)
	return errors.Trace(err)
}
```

Add a test case in `TestMyService_Mock`.

```go
t.Run("on_changed_my_config", func(t *testing.T) { // MARKER: MyConfig
	assert := testarossa.For(t)

	err := mock.OnChangedMyConfig(ctx)
	assert.Contains(err.Error(), "not implemented")
	mock.MockOnChangedMyConfig(func(ctx context.Context) (err error) {
		return nil
	})
	err = mock.OnChangedMyConfig(ctx)
	assert.NoError(err)
})
```

#### Step 11: Test the Callback

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

Insert test cases at the bottom of the integration test function using the recommended pattern.

```go
t.Run("test_case_name", func(t *testing.T) {
	assert := testarossa.For(t)

	err := svc.SetMyConfig(newValue)
	assert.NoError(err)
})
```

Do not remove the `HINT` comments.

#### Step 12: Housekeeping

Follow the `microbus/housekeeping` skill.

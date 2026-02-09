# Configuration

It's a common practice for microservices to define properties whose values can be configured without code changes, often by operators. Connection strings to a database, number of rows to display in a table, a timeout value, are all examples of configuration properties.

In `Microbus`, the microservice owns the definition of its config properties, while [the configurator core microservice](../structure/coreservices-configurator.md) owns their values. The former means that microservices are self-descriptive and independent, which is important in a distributed development environment. The latter means that managing configuration values and pushing them to a live system are centrally controlled, which is important because configs can contain secrets and because pushing the wrong config values can destabilize a deployment.

### Definition

Configuration properties must first be defined by the microservice using `DefineConfig`. In the following example, the property is named `Foo` and given the default value `Bar` and a regexp requirement that the value is comprised of at least one letter and only letters. The case-sensitive property name is required. Various [options](../structure/cfg.md) allow setting a default value, enforcing validation, and more.

```go
con.New("www.example.com")
con.DefineConfig("Foo", cfg.DefaultValue("Bar"), cfg.Validation("str ^[A-Za-z]+$"))
```

### Initial Values

The initial value of any configuration property is its default value.

On startup, the microservice looks for `config.yaml` or `config.local.yaml` files in the current working directory and its ancestor directories. Precedence is given to YAML files in a subdirectory over its ancestor directory. Within a given directory, `config.local.yaml` takes precedence over `config.yaml`.

A property value is applicable to a microservice that defines a property with the same name, and whose hostname equals or is a sub-domain of the domain name. The special domain `all` applies to all microservices.

```yaml
www.example.com:
  Foo: Baz
example.com:
  Foo: Bam
com:
  Foo: Bax
all:
  Foo: Bat
```

### Fetching Values From the Configurator

On startup, the microservice contacts the configurator microservice to ask for the values of its configuration properties. If an override value is available at the configurator, it is set as the new value of the config property; otherwise, the default value of the config property is set instead.

Configs are accessed using the `Config` method of the `Connector`. 

```go
foo := con.Config("Foo")
```

The microservice keeps listening for a command on the [control subscription](../tech/control-subs.md) `:888/config-refresh` and will respond by refetching config values from the configurator. The configurator issues this command on startup and on a periodic basis (every 20 minutes) to ensure that all microservices always have the latest config. If new values are received by the microservice, they will be set appropriately and the `OnConfigChanged` callback will be invoked.

```go
con.SetOnConfigChanged(func (ctx context.Context, changed func(string) bool) error {
	if changed("Foo") {
		con.LogInfo(ctx, "Foo changed", 
			"to", con.Config("Foo"),
		)
	}
	return nil
})
```

More commonly, the configuration property is defined by the [coding agent](../blocks/coding-agents.md) who takes care of generating the boilerplate code and creating an accessor method named after the configuration property, e.g. `svc.Foo()`, and the skeleton code of the `OnChangedFoo` callback.

### Notes

Fetching config values from the configurator is disabled in the `TESTING` [deployment environment](../tech/deployments.md).

Config values can be set programmatically using `SetConfig`, however such values will be overridden on the next fetch of config from the configurator. It is advisable to limit use of this action to testing scenarios when fetching values from the configurator is disabled.

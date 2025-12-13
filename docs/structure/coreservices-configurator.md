# Package `coreservices/configurator`

The configurator is a core microservice of `Microbus` and it must be included with practically all applications. Microservices that define config properties will not start if they cannot reach the configurator. This is why you'll see the configurator included first in most self-contained apps, such as in `main/main.go`:

```go
func main() {
	app := application.New()
	app.Add(
		// Configurator should start first
		configurator.NewService(),
	)
	app.Add(
		// Other microservices...
	)
	err := app.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%+v", err)
		os.Exit(19)
	}
}
```

The configurator is the owner of the values of the configuration properties. It loads these values from `config.yaml` and `config.local.yaml` files located in the current working directory and its ancestor directories. Precedence is given to YAML files in a subdirectory over its ancestor directory. Within a given directory, `config.local.yaml` takes precedence over `config.yaml`.

```yaml
domain:
  PropertyName: PropertyValue
```

A property value is applicable to a microservice that defines a property with the same name, and whose hostname equals or is a sub-domain of the domain name. The special domain `all` applies to all microservices. For example, in the following example, the value of `Foo` is applicable only to the `www.example.com` microservice, while the value of `Moo` is applicable to both `www.example.com` and `zzz.example.com`. The value of `Zoo` is applicable only to `zzz.example.com`.

```yaml
www.example.com:
  Foo: Bar
example.com:
  Moo: Cow
all:
  Zoo: Keeper
```

```go
www := connector.New("www.example.com")
www.DefineConfig("Foo")
www.DefineConfig("Moo")

zzz := connector.New("zzz.example.com")
zzz.DefineConfig("Moo")
zzz.DefineConfig("Zoo")
```

Note that both domain names and case insensitive, while property names are case sensitive.

Every 20 minutes the configurator broadcasts the command `https://all:888/config-refresh` to instruct all microservices to refresh their config. They respond by calling the configurator's `:888/values` endpoint to fetch the current values. This guarantees that microservices do not fall out of sync with their configuration, at least not for long.

The `:444/refresh` endpoint can be called manually to force a refresh at any time.

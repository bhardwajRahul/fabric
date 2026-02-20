---
name: Externalizing and Translating Text
description: Externalizes user-facing text to a resource bundle where they can be easily translated. Use to externalize static strings that are shown to the end user.
---

**CRITICAL**: Read and analyze this microservice before starting. Do NOT explore or analyze other microservices. The instructions in this skill are self-contained to this microservice.

## Workflow

Copy this checklist and track your progress:

```
Externalizing and translating strings:
- [ ] Step 1: Create string bundle
- [ ] Step 2: Transfer strings
- [ ] Step 3: Translate strings for requested languages
- [ ] Step 4: Update references in Go files
- [ ] Step 5: Update references in templates
- [ ] Step 6: Housekeeping
```

#### Step 1: Create String Bundle

Create `resources/text.yaml`, if one does not already exist.

#### Step 2: Transfer Strings

Locate static strings in the microservice that are ultimately shown to the end user. These are likely to be in `service.go` or in HTML or text templates in the `resources` directory.

For each string, create an entry in `resources/text.yaml` that maps a unique key to its localized value on a per language basis. Use PascalCase for the string key (e.g. `MyString`) and ISO 639 language codes for each language (e.g. `en-US`).

```yaml
HelloWorld:
  en: Hello World
```

#### Step 3: Translate Strings for Requested Languages

Update `resources/text.yaml` and add a localized value for each explicitly requested language.
Use the ISO 639 language code under the key of each localization.

```yaml
HelloWorld:
  en: Hello World
  en-AU: G'day World
  fr: Bonjour le Monde
  es: Hola Mundo
```

The `en` or `en-US` localizations are used by default when no other language matches the request's context.
If neither localization is included, a `default` value should be provided instead.

```yaml
HolaMundo:
  fr: Bonjour le Monde
  es: Hola Mundo
  default: Hola Mundo
```

#### Step 4: Update References in Go Files

Use `svc.MustLoadResString` to load strings in Go files such as `service.go`.

Before:

```go
func (svc *Service) HelloWorld(w http.ResponseWriter, r *http.Request) (err error) {
	w.Write([]byte("Hello World"))
	return err
}
```

After:

```go
func (svc *Service) HelloWorld(w http.ResponseWriter, r *http.Request) (err error) {
	ctx := r.Context()
	textHelloWorld := svc.MustLoadResString(ctx, "HelloWorld")
	w.Write([]byte(textHelloWorld))
	return err
}
```

#### Step 5: Update References in Templates

To use localized strings in HTML templates, load all strings with `svc.MustLoadResStrings` into a map, pass the map as part of the data to the template, and use the map in the template to obtain the string by key, instead of the static string.

Before:

```go
func (svc *Service) HelloWorld(w http.ResponseWriter, r *http.Request) (err error) {
	data := struct{
		OtherData int
	}{
		OtherData: 5,
	}
	err = svc.WriteResTemplate(w, "mytemplate.html", data)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}
```

```html
<b>Hello World</b>
```

After:

```go
func (svc *Service) HelloWorld(w http.ResponseWriter, r *http.Request) (err error) {
	ctx := r.Context()
	data := struct{
		OtherData int
		Text map[string]string
	}{
		OtherData: 5,
		Text: svc.MustLoadResStrings(ctx),
	}
	err = svc.WriteResTemplate(w, "mytemplate.html", data)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}
```

```html
<b>{{ .Text.HelloWorld }}</b>
```

#### Step 6: Housekeeping

Follow the `microbus/housekeeping` skill.

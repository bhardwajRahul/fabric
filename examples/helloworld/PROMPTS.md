## Upgrade to v2

Upgrade the helloworld microservice from the v1 code-generated pattern to the v2 agent-maintained pattern using the `upgrade-to-v2` skill. This involved deleting generated files (`doc.go`, `service-gen.go`, `clients-gen.go`, `version-gen_test.go`, the `intermediate/` subdirectory), and creating new `intermediate.go`, `mock.go`, and `helloworldapi/client.go` files.

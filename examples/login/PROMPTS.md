## Upgrade to v2

Upgrade the login microservice from the v1 code-generated pattern to the v2 agent-maintained pattern using the `upgrade-to-v2` skill. This involved deleting generated files (`doc.go`, `service-gen.go`, `clients-gen.go`, `version-gen_test.go`, the `intermediate/` subdirectory), creating new `intermediate.go`, `mock.go`, and `loginapi/client.go` files, and removing the `contentType` parameter from web endpoint client signatures.

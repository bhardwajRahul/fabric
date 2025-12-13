# Out of Scope

`Microbus`'s focus is on building and operating microservices at scale. The following areas are currently out of scope:

* __User interface__ - `Microbus` is a backend framework. Its link to the front-end are microservice endpoints that respond with JSON to single-page front-end applications, and an OpenAPI document that catalogs those endpoints. It is possible for microservices to generate HTML but `Microbus` provides no tooling in this area.
* __SDLC automation__ - Automating the SDLC of `Microbus` solutions is currently out of scope. 
* __Databases__ - The choice of database depends heavily on the use case of the solution and as an enabling technology, `Microbus` takes no sides on this topic. [`Sequel`](https://github.com/microbus-io/sequel) is a companion project to `Microbus` that facilitates the development of CRUD microservices in a SQL database.
* __AI__ - Artificial intelligence is currently out of scope, except in the context of coding agents.

Some of these areas have room for contributors to step in. Contact us if you want to [get involved](../../README.md#-get-involved).

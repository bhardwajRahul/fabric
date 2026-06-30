## Prompt: Worked example for `add-python-microservice`

Build the canonical example that proves the Go-Python bridge works end-to-end. It should:

- Load a real sentence-transformers model (`all-MiniLM-L6-v2`) in an in-process Python virtual environment via [`github.com/microbus-io/pyvenv`](https://github.com/microbus-io/pyvenv).
- Expose two typed Go endpoints: `Embed(text) -> []float64` returns the embedding vector, and `Similarity(a, b) -> float64` returns the cosine similarity between two strings.
- Serve a small `/demo` HTML page styled like the other tour examples (chatbox, calculator) that exercises both endpoints from a browser.
- Use the manual-subscription pattern: `OnStartup` constructs the `*pyvenv.Venv` and launches `svc.venv.Start(ctx)` in a background goroutine via `svc.Go`; `Embed` and `Similarity` are subscribed with `sub.Manual()` and `sub.Tag("python")` so callers see clean 404 ack-timeouts until the venv reaches `StateReady` and the LivenessCallback brings the python-tagged subs onto the bus.
- Not be bundled in `main/main.go` by default. The guided tour adds it explicitly so dev environments without Python remain unaffected.

The example serves both as documentation for the `add-python-microservice` skill and as a smoke test for the manual-subscription primitive in a realistic Python-backed microservice.

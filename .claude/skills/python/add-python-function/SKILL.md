---
name: add-python-function
description: TRIGGER when user asks to add a Python-backed function endpoint to an existing Python-backed microservice. The Go side is a normal Microbus function endpoint; the body delegates to a Python function via svc.venv.CallAndAwait. Affects intermediate.go, *api/endpoints.go, *api/client.go, mock.go, service.py, manifest.yaml.
---

**CRITICAL**: Read `.claude/rules/python.txt` before proceeding. It explains the Go-Python boundary, the manual-subscription pattern, args/result conventions, and how the in-process pyvenv module is wired into the microservice.

**CRITICAL**: Do NOT explore or analyze other microservices unless explicitly instructed to do so. The instructions in this skill are self-contained to this microservice.

## Workflow

Copy this checklist and track your progress:

```
Adding a Python-backed function endpoint:
- [ ] Step 1: Verify the microservice is Python-backed
- [ ] Step 2: Determine the Go signature
- [ ] Step 3: Run add-function with sub.Manual() and sub.Tag("python")
- [ ] Step 4: Replace the handler body with svc.venv.CallAndAwait
- [ ] Step 5: Add the Python function to service.py
- [ ] Step 6: Housekeeping
```

#### Step 1: Verify the Microservice Is Python-Backed

The microservice must already have `python.go` and a `service.py` at its root. If not, run `add-python-microservice` first (it works both for fresh microservices and for extending an existing one without overwriting business logic).

#### Step 2: Determine the Go Signature

Determine the Go signature of the function endpoint. The constraints from `add-function` apply (typed inputs and outputs, `ctx context.Context` first, `err error` last, etc.).

```go
func MyFunction(ctx context.Context, input1 string, input2 int) (output1 float64, err error)
```

Python sees the inputs as a dict keyed by the field names in `MyFunctionIn` (driven by their `json:"..."` tags). The dict is passed to a Python function whose signature mirrors the Go one (excluding `ctx`).

#### Step 3: Run `add-function` with `sub.Manual()` and `sub.Tag("python")`

Run the `add-function` skill, with these overrides applied as you reach each of its steps:

- **Step 10 (Implement the Logic)**: skip. Step 4 below replaces it.
- **Step 12 (Bind the Marshaler Function to the Microservice)**: add `sub.Manual()` and `sub.Tag("python")` to the `svc.Subscribe(...)` options block in `intermediate.go`:

  ```go
  svc.Subscribe( // MARKER: MyFunction
      "MyFunction", svc.doMyFunction,
      sub.At(myserviceapi.MyFunction.Method, myserviceapi.MyFunction.Route),
      sub.Description(`...`),
      sub.Function(myserviceapi.MyFunctionIn{}, myserviceapi.MyFunctionOut{}),
      sub.Manual(),
      sub.Tag("python"),
  )
  ```

- **Step 13 (regenerate the mock)**: run normally.
- **Step 14 (test the function)**: run normally, then add a one-line opt-in HINT immediately after the `app.RunInTest(t)` line so a future reader can switch the test from mock-only to real-Python without hunting through docs:

  ```go
  app.RunInTest(t)
  
  // HINT: Uncomment to spin up real Python and exercise actual execution (slow on first run)
  // svc.StartPyVenv(ctx)
  ```

- **Housekeeping step**: skip; housekeeping runs once at the end of this skill instead.

When `add-function` finishes, return here for Step 4.

#### Step 4: Replace the Handler Body with `svc.venv.CallAndAwait`

In `service.go`, replace the handler body that `add-function` left as a stub with a delegation to `svc.venv.CallAndAwait`. Pass `MyFunctionIn` directly as `args` (its `json:"..."` tags drive the wire format the Python function sees as a dict); the result is unmarshaled into the typed `MyFunctionOut`.

```go
func (svc *Service) MyFunction(ctx context.Context, input1 string, input2 int) (output1 float64, err error) { // MARKER: MyFunction
    if svc.venv == nil || !svc.venv.Ready() {
        return 0, errors.New("venv not ready", http.StatusServiceUnavailable)
    }
    in := myserviceapi.MyFunctionIn{
        Input1: input1,
        Input2: input2,
    }
    var out myserviceapi.MyFunctionOut
    err = svc.venv.CallAndAwait(ctx, "my_function", in, &out)
    if err != nil {
        return 0, errors.Trace(err)
    }
    return out.Output1, nil
}
```

`CallAndAwait` is the synchronous shorthand: it does `Call` (which returns a callID and the Python work starts running) followed by `Await(ctx, callID, &out)` on the same goroutine. For a function endpoint the caller's ctx is the call's deadline; if it expires, the Python work keeps running until completion and the result eventually ages out of the cache.

#### Step 5: Add the Python Function to `service.py`

Append a Python function to `service.py` (at the microservice's root) whose name is the snake_case form of the Go function name, accepting a dict and returning a dict. The docstring is the same description text passed to `sub.Description(...)` on the Go side.

```python
def my_function(args):  # MARKER: MyFunction
    """MyFunction does X."""
    input1 = args["input1"]
    input2 = args["input2"]
    # ... compute ...
    return {"output1": 42.0}
```

#### Step 6: Housekeeping

Follow the `housekeeping` skill.

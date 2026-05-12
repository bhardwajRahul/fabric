---
name: add-python-task
description: TRIGGER when user asks to add a Python-backed workflow task to an existing Python-backed microservice. The Go side is a normal Microbus workflow task endpoint on :428; the body uses svc.venv.Call + Await with the callID persisted in flow state so a long-running Python call survives task retries across the framework's 15-minute hop ceiling. Affects intermediate.go, *api/endpoints.go, *api/client.go, mock.go, service.py, manifest.yaml.
---

**CRITICAL**: Read `.claude/rules/python.txt` and `.claude/rules/workflows.txt` before proceeding. The former covers the Go-Python bridge; the latter covers task lifecycle, flow state, retries, and OnTimeout/OnError routing.

**CRITICAL**: Do NOT explore or analyze other microservices unless explicitly instructed to do so. The instructions in this skill are self-contained to this microservice.

## Workflow

Copy this checklist and track your progress:

```
Adding a Python-backed workflow task:
- [ ] Step 1: Verify the microservice is Python-backed
- [ ] Step 2: Determine the task signature
- [ ] Step 3: Run add-task with sub.Manual() and sub.Tag("python")
- [ ] Step 4: Replace the task body with the Call+Await durability pattern
- [ ] Step 5: Add the Python function to service.py
- [ ] Step 6: Housekeeping
```

#### Step 1: Verify the Microservice Is Python-Backed

The microservice must already have `python.go` and a `service.py` at its root. If not, run `add-python-microservice` first (it works both for fresh microservices and for extending an existing one without overwriting business logic).

#### Step 2: Determine the Task Signature

Determine the Go task signature. Task signatures include `flow *workflow.Flow` after `ctx` but before the typed inputs. Outputs that share a name with an input must use the `Out` suffix on the output side (read-modify-write pattern).

```go
func MyTask(ctx context.Context, flow *workflow.Flow, input1 string, input2 int) (output1 float64, err error)
```

Python sees the inputs as a dict keyed by the field names in `MyTaskIn`. Python does not see `ctx` or `flow`.

#### Step 3: Run `add-task` with `sub.Manual()` and `sub.Tag("python")`

Run the `add-task` skill, with these overrides applied as you reach each of its steps:

- **Step 10 (Implement the Logic)**: skip. Step 4 below replaces it.
- **Step 12 (Bind the Marshaler Function to the Microservice)**: add `sub.Manual()` and `sub.Tag("python")` to the `svc.Subscribe(...)` options block in `intermediate.go`:

  ```go
  svc.Subscribe( // MARKER: MyTask
      "MyTask", svc.doMyTask,
      sub.At(myserviceapi.MyTask.Method, myserviceapi.MyTask.Route),
      sub.Description(`...`),
      sub.Task(myserviceapi.MyTaskIn{}, myserviceapi.MyTaskOut{}),
      sub.Manual(),
      sub.Tag("python"),
  )
  ```

- **Step 13 (regenerate the mock)**: run normally.
- **Step 14 (test the task)**: run normally, then add a one-line opt-in HINT immediately after the `app.RunInTest(t)` line so a future reader can switch the test from mock-only to real-Python without hunting through docs:

  ```go
  app.RunInTest(t)
  
  // HINT: Uncomment to spin up real Python and exercise actual execution (slow on first run)
  // svc.StartPyVenv(ctx)
  ```

- **Housekeeping step**: skip; housekeeping runs once at the end of this skill instead.

When `add-task` finishes, return here for Step 4.

#### Step 4: Replace the Task Body with the Call+Await Durability Pattern

In `service.go`, replace the task body that `add-task` left as a stub with the durable Call+Await pattern. The callID is persisted in flow state under the key `pyCallID`; if the task is re-entered (via `flow.RetryNowOnTimeout` after the step's time budget expires), the existing callID is re-Awaited rather than a fresh call being issued. This lets a Python computation that exceeds the framework's 15-minute hop ceiling survive across task retries.

```go
func (svc *Service) MyTask(ctx context.Context, flow *workflow.Flow, input1 string, input2 int) (output1 float64, err error) { // MARKER: MyTask
    if svc.venv == nil || !svc.venv.Ready() {
        return 0, errors.New("venv not ready", http.StatusServiceUnavailable)
    }

    callID := flow.GetString("pyCallID")
    if callID == "" {
        in := myserviceapi.MyTaskIn{
            Input1: input1,
            Input2: input2,
        }
        callID, err = svc.venv.Call(ctx, "my_task", in)
        if err != nil {
            return 0, errors.Trace(err)
        }
        flow.SetString("pyCallID", callID)
    }

    var out myserviceapi.MyTaskOut
    err = svc.venv.Await(ctx, callID, &out)
    if err != nil {
        if flow.RetryNowOnTimeout(err) {
            return 0, nil
        }
        flow.SetString("pyCallID", "") // clear on terminal error so downstream steps don't see it
        return 0, errors.Trace(err)
    }
    flow.SetString("pyCallID", "") // clear on success so downstream steps don't see it
    return out.Output1, nil
}
```

How the pattern composes:

- **First entry**: `pyCallID` is empty, the task issues `Call`, persists the returned callID, then Awaits. If the step time budget allows the call to complete, the result is returned and `pyCallID` is cleared.
- **Retry entry (after 408 timeout)**: `pyCallID` is the previously-issued callID. The Python work has been running this whole time. The task skips `Call` and goes straight to `Await`, which either completes immediately (Python finished) or waits up to the next step time budget.
- **Terminal error or completion**: `pyCallID` is cleared so the workflow's later steps don't see the now-consumed callID in flow state.

#### Step 5: Add the Python Function to `service.py`

Append a Python function to `service.py` (at the microservice's root) whose name is the snake_case form of the Go task name, accepting a dict and returning a dict. The docstring is the same description text passed to `sub.Description(...)` on the Go side.

```python
def my_task(args):  # MARKER: MyTask
    """MyTask does X."""
    input1 = args["input1"]
    input2 = args["input2"]
    # ... compute ...
    return {"output1": 42.0}
```

#### Step 6: Housekeeping

Follow the `housekeeping` skill.

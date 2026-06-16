/*
Copyright (c) 2026 Microbus LLC and various contributors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package foreman

import (
	"context"
	"encoding/json"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/microbus-io/dwarf/workflow"
	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/application"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/sub"
	"github.com/microbus-io/testarossa"

	"github.com/microbus-io/fabric/coreservices/foreman/foremanapi"
)

// This integration test exercises the foreman end-to-end over the Microbus bus against a real downstream
// microservice that serves workflow graphs and task endpoints. Unlike the engine's in-process fixtures
// (which inject a TestProxy Host), this drives the actual adapter seam: host.go's LoadGraph GETs a graph
// over the bus and decodes the {"graph": …} envelope, and ExecuteTask marshals the *workflow.Flow to JSON,
// POSTs it to the task, and unmarshals the returned flow - so the downstream endpoints below do their own
// (un)marshaling, mirroring what a generated workflow microservice's intermediate.go does.

// graphHandler turns a graph builder into a bus endpoint that serves the {"graph": …} envelope the
// foreman's LoadGraph expects.
func graphHandler(build func() *workflow.Graph) sub.HTTPHandler {
	return func(w http.ResponseWriter, r *http.Request) error {
		g := build()
		if err := g.Validate(); err != nil {
			return errors.Trace(err)
		}
		w.Header().Set("Content-Type", "application/json")
		return errors.Trace(json.NewEncoder(w).Encode(struct {
			Graph *workflow.Graph `json:"graph"`
		}{Graph: g}))
	}
}

// taskHandler turns a task body into a bus endpoint: decode the posted flow carrier, run the body (which
// reads/writes state and may arm control signals), and encode the flow back for the engine to read.
func taskHandler(body func(f *workflow.Flow) error) sub.HTTPHandler {
	return func(w http.ResponseWriter, r *http.Request) error {
		var flow workflow.Flow
		if err := json.NewDecoder(r.Body).Decode(&flow); err != nil {
			return errors.Trace(err)
		}
		if err := body(&flow); err != nil {
			return err // no trace: preserve the status code so the foreman can classify it
		}
		w.Header().Set("Content-Type", "application/json")
		return errors.Trace(json.NewEncoder(w).Encode(&flow))
	}
}

// subscribeGraph registers a workflow graph endpoint (GET on :428). The sub.Workflow feature option is
// required for the subscription to activate; its zero-struct args are only for OpenAPI reflection, which
// the foreman does not use (it GETs the graph by URL directly).
func subscribeGraph(c *connector.Connector, name, route string, build func() *workflow.Graph) {
	c.Subscribe(name, graphHandler(build), sub.At("GET", route), sub.Workflow(struct{}{}, struct{}{}))
}

// subscribeTask registers a workflow task endpoint (POST on :428). sub.Task is the required feature option.
func subscribeTask(c *connector.Connector, name, route string, body func(f *workflow.Flow) error) {
	c.Subscribe(name, taskHandler(body), sub.At("POST", route), sub.Task(struct{}{}, struct{}{}))
}

// linearGraph is A -> B -> C -> END, used by several subtests as a trivial happy path.
func linearGraph(host string) *workflow.Graph {
	g := workflow.NewGraph("Linear", host+":428/linear")
	g.AddTask("A", host+":428/a")
	g.AddTask("B", host+":428/b")
	g.AddTask("C", host+":428/c")
	g.AddTransition("A", "B")
	g.AddTransition("B", "C")
	g.AddTransition("C", workflow.END)
	return g
}

// registerLinear wires the linear graph + its A/B/C tasks (n := ((n+1)*10)+5) onto a connector.
func registerLinear(c *connector.Connector, host string) {
	subscribeGraph(c, "LinearGraph", ":428/linear", func() *workflow.Graph { return linearGraph(host) })
	subscribeTask(c, "A", ":428/a", func(f *workflow.Flow) error { f.SetInt("n", f.GetInt("n")+1); return nil })
	subscribeTask(c, "B", ":428/b", func(f *workflow.Flow) error { f.SetInt("n", f.GetInt("n")*10); return nil })
	subscribeTask(c, "C", ":428/c", func(f *workflow.Flow) error { f.SetInt("n", f.GetInt("n")+5); return nil })
}

func TestForemanIntegration(t *testing.T) {
	ctx := context.Background()
	const host = "inttest.flows"

	var breakerAttempts atomic.Int32

	wf := connector.New(host).Init(func(c *connector.Connector) error {
		registerLinear(c, host)

		// Subgraph: Parent P -> END, where P calls flow.Subgraph(Child) and adopts its output. Child K -> END.
		subscribeGraph(c, "ParentGraph", ":428/parent", func() *workflow.Graph {
			g := workflow.NewGraph("Parent", host+":428/parent")
			g.AddTask("P", host+":428/p")
			g.AddTransition("P", workflow.END)
			return g
		})
		subscribeGraph(c, "ChildGraph", ":428/child", func() *workflow.Graph {
			g := workflow.NewGraph("Child", host+":428/child")
			g.AddTask("K", host+":428/k")
			g.AddTransition("K", workflow.END)
			return g
		})
		subscribeTask(c, "P", ":428/p", func(f *workflow.Flow) error {
			var out map[string]any
			yield, err := f.Subgraph(host+":428/child", map[string]any{"v": f.GetInt("x")}, &out)
			if yield || err != nil {
				return err
			}
			w, _ := out["w"].(float64)
			f.SetInt("result", int(w))
			return nil
		})
		subscribeTask(c, "K", ":428/k", func(f *workflow.Flow) error { f.SetInt("w", f.GetInt("v")*2); return nil })

		// Interrupt: I -> END. First pass parks via flow.Interrupt; on resume it records the answer.
		subscribeGraph(c, "InterruptGraph", ":428/interrupt", func() *workflow.Graph {
			g := workflow.NewGraph("Interrupt", host+":428/interrupt")
			g.AddTask("I", host+":428/i")
			g.AddTransition("I", workflow.END)
			return g
		})
		subscribeTask(c, "I", ":428/i", func(f *workflow.Flow) error {
			var resume map[string]any
			yield, _ := f.Interrupt(map[string]any{"need": "info"}, &resume)
			if yield {
				return nil
			}
			ans, _ := resume["answer"].(float64)
			f.SetInt("answer", int(ans))
			return nil
		})

		// Breaker: Bk -> END. The task returns 503 on its first dispatch and succeeds afterward, so the
		// foreman classifies the failure as a breaker trip (ErrUnavailable), parks+probes, and recovers.
		subscribeGraph(c, "BreakerGraph", ":428/breaker", func() *workflow.Graph {
			g := workflow.NewGraph("Breaker", host+":428/breaker")
			g.AddTask("Bk", host+":428/bk")
			g.AddTransition("Bk", workflow.END)
			return g
		})
		subscribeTask(c, "Bk", ":428/bk", func(f *workflow.Flow) error {
			if breakerAttempts.Add(1) == 1 {
				return errors.New("temporarily unavailable", http.StatusServiceUnavailable)
			}
			f.SetBool("served", true)
			return nil
		})
		return nil
	})

	tester := connector.New("inttest.client")

	app := application.New()
	app.Add(NewService(), wf, tester)
	app.RunInTest(t)

	client := foremanapi.NewClient(tester)

	t.Run("linear_happy_path", func(t *testing.T) {
		assert := testarossa.For(t)
		out, err := client.Run(ctx, host+":428/linear", map[string]any{"n": 1}, nil)
		if !assert.NoError(err) {
			return
		}
		assert.Equal(workflow.StatusCompleted, out.Status)
		n, _ := out.State["n"].(float64)
		assert.Equal(25, int(n)) // ((1+1)*10)+5
	})

	t.Run("subgraph_call_and_return", func(t *testing.T) {
		assert := testarossa.For(t)
		out, err := client.Run(ctx, host+":428/parent", map[string]any{"x": 3}, nil)
		if !assert.NoError(err) {
			return
		}
		assert.Equal(workflow.StatusCompleted, out.Status)
		r, _ := out.State["result"].(float64)
		assert.Equal(6, int(r)) // child computed w = v*2 = 6, parent adopted it
	})

	t.Run("interrupt_then_resume", func(t *testing.T) {
		assert := testarossa.For(t)
		out, err := client.Run(ctx, host+":428/interrupt", nil, nil)
		if !assert.NoError(err) {
			return
		}
		// Run awaits, and an interrupt is a stop: it returns interrupted with the payload.
		assert.Equal(workflow.StatusInterrupted, out.Status)
		assert.Equal("info", out.InterruptPayload["need"])

		if !assert.NoError(client.Resume(ctx, out.FlowKey, map[string]any{"answer": 42})) {
			return
		}
		final, err := client.Await(ctx, out.FlowKey)
		if !assert.NoError(err) {
			return
		}
		assert.Equal(workflow.StatusCompleted, final.Status)
		ans, _ := final.State["answer"].(float64)
		assert.Equal(42, int(ans))
	})

	t.Run("breaker_classification_and_recovery", func(t *testing.T) {
		assert := testarossa.For(t)
		out, err := client.Run(ctx, host+":428/breaker", nil, nil)
		if !assert.NoError(err) {
			return
		}
		// The first dispatch 503'd (breaker trip), the probe re-dispatched and succeeded: the flow recovers
		// to completed rather than failing.
		assert.Equal(workflow.StatusCompleted, out.Status)
		served, _ := out.State["served"].(bool)
		assert.True(served)
		assert.True(breakerAttempts.Load() >= 2, "expected a retry after the 503 trip")
	})
}

// TestForemanIntegration_CrossReplica runs a flow against two foreman replicas sharing one plane (hence one
// set of shard databases). Work created via one replica's client is dispatched and completed across the
// pair, and the awaiting client is woken via the cross-replica statusChange Signal - exercising the
// SignalPeers multicast and the inbound Signal self-delivery filter end-to-end.
func TestForemanIntegration_CrossReplica(t *testing.T) {
	ctx := context.Background()
	const host = "inttest.xr"

	wf := connector.New(host).Init(func(c *connector.Connector) error {
		registerLinear(c, host)
		return nil
	})
	tester := connector.New("inttest.xr.client")

	app := application.New()
	app.Add(NewService(), NewService(), wf, tester) // two foreman.core replicas
	app.RunInTest(t)

	client := foremanapi.NewClient(tester)
	out, err := client.Run(ctx, host+":428/linear", map[string]any{"n": 1}, nil)
	if !testarossa.For(t).NoError(err) {
		return
	}
	assert := testarossa.For(t)
	assert.Equal(workflow.StatusCompleted, out.Status)
	n, _ := out.State["n"].(float64)
	assert.Equal(25, int(n))
}

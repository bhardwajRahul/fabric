/*
Copyright (c) 2023-2026 Microbus LLC and various contributors

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

package shell

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/cfg"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/fabric/sub"
	"github.com/microbus-io/fabric/utils"
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/coreservices/shell/resources"
	"github.com/microbus-io/fabric/coreservices/shell/shellapi"
)

var (
	_ context.Context
	_ json.Encoder
	_ http.Request
	_ strconv.NumError
	_ time.Duration
	_ errors.TracedError
	_ cfg.Option
	_ httpx.BodyReader
	_ sub.Option
	_ utils.SyncMap[string, string]
	_ shellapi.Client
	_ *workflow.Flow
)

const (
	Hostname = shellapi.Hostname
	Version  = 4
)

// ToDo is implemented by the service or mock.
// The intermediate delegates handling to this interface.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
	Execute(ctx context.Context, cmd string, workDir string, stdin string, envars map[string]string) (exitCode int, stdout string, stderr string, err error) // MARKER: Execute
}

// NewService creates a new instance of the microservice.
func NewService() *Service {
	svc := &Service{}
	svc.Intermediate = NewIntermediate(svc)
	return svc
}

// Init enables a single-statement pattern for initializing the microservice.
func (svc *Service) Init(initializer func(svc *Service) (err error)) *Service {
	svc.Connector.Init(func(_ *connector.Connector) (err error) {
		return initializer(svc)
	})
	return svc
}

// Intermediate extends and customizes the generic base connector.
type Intermediate struct {
	*connector.Connector
	ToDo
}

// NewIntermediate creates a new instance of the intermediate.
func NewIntermediate(impl ToDo) *Intermediate {
	svc := &Intermediate{
		Connector: connector.New(Hostname),
		ToDo:      impl,
	}
	svc.SetVersion(Version)
	svc.SetDescription(`Shell enables running shell commands on a host.`)
	svc.SetOnStartup(svc.OnStartup)
	svc.SetOnShutdown(svc.OnShutdown)
	svc.SetResFS(resources.FS)
	svc.SetOnObserveMetrics(svc.doOnObserveMetrics)
	svc.SetOnConfigChanged(svc.doOnConfigChanged)

	// HINT: Add functional endpoints here
	svc.Subscribe( // MARKER: Execute
		"Execute", svc.doExecute,
		sub.At(shellapi.Execute.Method, shellapi.Execute.Route),
		sub.Description(`Execute runs a shell command on the host and returns the exit code, standard output, and standard error.

Input:
  - cmd: the shell command to execute
  - workDir: optional working directory for the command
  - stdin: optional standard input to feed to the command
  - envars: optional environment variables to set for the command, as key-value pairs

Output:
  - exitCode: the exit code of the command
  - stdout: the standard output of the command
  - stderr: the standard error of the command`),
		sub.Function(shellapi.ExecuteIn{}, shellapi.ExecuteOut{}),
	)

	// HINT: Add web endpoints here

	// HINT: Add metrics here

	// HINT: Add tickers here

	// HINT: Add configs here
	svc.DefineConfig( // MARKER: MaxOutputBytes
		"MaxOutputBytes",
		cfg.Description(`MaxOutputBytes caps the number of bytes retained from each of stdout and stderr. When a stream
exceeds this cap, the head and tail are kept and the middle is elided with a marker. The command is
not killed; excess output is silently discarded so the child can run to completion.`),
		cfg.DefaultValue("262144"),
		cfg.Validation("int [1024,]"),
	)

	// HINT: Add inbound event sinks here

	// HINT: Add task endpoints here

	// HINT: Add graph endpoints here

	_ = marshalFunction
	return svc
}

// doOnObserveMetrics is called when metrics are produced.
func (svc *Intermediate) doOnObserveMetrics(ctx context.Context) (err error) {
	return svc.Parallel(
	// HINT: Call JIT observers to record the metric here
	)
}

// doOnConfigChanged is called when the config of the microservice changes.
func (svc *Intermediate) doOnConfigChanged(ctx context.Context, changed func(string) bool) (err error) {
	// HINT: Call named callbacks here
	return nil
}

// doExecute handles marshaling for Execute.
func (svc *Intermediate) doExecute(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Execute
	var in shellapi.ExecuteIn
	var out shellapi.ExecuteOut
	err = marshalFunction(w, r, shellapi.Execute.Route, &in, &out, func(_ any, _ any) error {
		out.ExitCode, out.Stdout, out.Stderr, err = svc.Execute(r.Context(), in.Cmd, in.WorkDir, in.Stdin, in.Envars)
		return err // No trace
	})
	return err // No trace
}

/*
MaxOutputBytes caps the number of bytes retained from each of stdout and stderr. When a stream
exceeds this cap, the head and tail are kept and the middle is elided with a marker. The command is
not killed; excess output is silently discarded so the child can run to completion.
*/
func (svc *Intermediate) MaxOutputBytes() (value int) { // MARKER: MaxOutputBytes
	_val := svc.Config("MaxOutputBytes")
	_i, _ := strconv.ParseInt(_val, 10, 64)
	return int(_i)
}

/*
SetMaxOutputBytes sets the value of the configuration property.
*/
func (svc *Intermediate) SetMaxOutputBytes(value int) (err error) { // MARKER: MaxOutputBytes
	return svc.SetConfig("MaxOutputBytes", strconv.Itoa(value))
}

// marshalFunction handles marshaling for functional endpoints.
func marshalFunction(w http.ResponseWriter, r *http.Request, route string, in any, out any, execute func(in any, out any) error) error {
	err := httpx.ReadInputPayload(r, route, in)
	if err != nil {
		return errors.Trace(err)
	}
	err = execute(in, out)
	if err != nil {
		return err // No trace
	}
	err = httpx.WriteOutputPayload(w, out)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

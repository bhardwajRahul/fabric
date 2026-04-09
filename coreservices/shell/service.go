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
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/coreservices/shell/shellapi"
)

var (
	_ context.Context
	_ http.Request
	_ errors.TracedError
	_ *workflow.Flow
	_ shellapi.Client
)

/*
Service implements the shell.core microservice.

Shell enables running shell commands on a host machine. It executes commands
in a subprocess and returns the exit code, standard output, and standard error.
*/
type Service struct {
	*Intermediate // IMPORTANT: Do not remove

	// HINT: Add member variables here
}

// OnStartup is called when the microservice is started up.
func (svc *Service) OnStartup(ctx context.Context) (err error) {
	return nil
}

// OnShutdown is called when the microservice is shut down.
func (svc *Service) OnShutdown(ctx context.Context) (err error) {
	return nil
}

/*
Execute runs a shell command on the host and returns the exit code, standard output, and standard error.

Input:
  - cmd: the shell command to execute
  - workDir: optional working directory for the command
  - stdin: optional standard input to feed to the command
  - envars: optional environment variables to set for the command, as key-value pairs

Output:
  - exitCode: the exit code of the command
  - stdout: the standard output of the command
  - stderr: the standard error of the command
*/
func (svc *Service) Execute(ctx context.Context, cmd string, workDir string, stdin string, envars map[string]string) (exitCode int, stdout string, stderr string, err error) { // MARKER: Execute
	if cmd == "" {
		return 0, "", "", errors.New("cmd is required", http.StatusBadRequest)
	}
	loggedCmd := cmd
	if len(loggedCmd) > 1024 {
		loggedCmd = loggedCmd[:1024] + "..."
	}
	defer func() {
		args := []any{
			"cmd", loggedCmd,
			"workDir", workDir,
			"exitCode", exitCode,
			"stdoutBytes", len(stdout),
			"stderrBytes", len(stderr),
		}
		if err != nil {
			args = append(args, "error", err.Error())
		}
		svc.LogInfo(ctx, "Shell command executed", args...)
	}()
	var c *exec.Cmd
	if runtime.GOOS == "windows" {
		c = exec.CommandContext(ctx, "cmd", "/C", cmd)
	} else {
		c = exec.CommandContext(ctx, "sh", "-c", cmd)
	}
	configureProcessGroup(c)
	if workDir == "" {
		workDir, _ = os.Getwd()
	}
	c.Dir = workDir
	if stdin != "" {
		c.Stdin = strings.NewReader(stdin)
	}
	if len(envars) > 0 {
		c.Env = os.Environ()
		for k, v := range envars {
			c.Env = append(c.Env, k+"="+v)
		}
	}
	outBuf := newCappedBuffer(svc.MaxOutputBytes())
	errBuf := newCappedBuffer(svc.MaxOutputBytes())
	c.Stdout = outBuf
	c.Stderr = errBuf
	runErr := c.Run()
	stdout = outBuf.String()
	stderr = errBuf.String()
	if ctxErr := ctx.Err(); ctxErr != nil {
		svc.LogWarn(ctx, "Shell command killed by context", "cause", ctxErr.Error())
	}
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			return 0, "", "", errors.Trace(runErr)
		}
	}
	return exitCode, stdout, stderr, nil
}

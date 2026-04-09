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
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/application"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/fabric/pub"
	"github.com/microbus-io/fabric/sub"
	"github.com/microbus-io/fabric/workflow"
	"github.com/microbus-io/testarossa"

	"github.com/microbus-io/fabric/coreservices/shell/shellapi"
)

var (
	_ context.Context
	_ io.Reader
	_ *http.Request
	_ *testing.T
	_ jwt.MapClaims
	_ application.Application
	_ connector.Connector
	_ frame.Frame
	_ pub.Option
	_ sub.Option
	_ *errors.TracedError
	_ *workflow.Flow
	_ testarossa.Asserter
	_ shellapi.Client
)

func TestShell_Mock(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	mock := NewMock()
	mock.SetDeployment(connector.TESTING)

	t.Run("on_startup", func(t *testing.T) {
		assert := testarossa.For(t)
		err := mock.OnStartup(ctx)
		assert.NoError(err)

		mock.SetDeployment(connector.PROD)
		err = mock.OnStartup(ctx)
		assert.Error(err)
		mock.SetDeployment(connector.TESTING)
	})

	t.Run("on_shutdown", func(t *testing.T) {
		assert := testarossa.For(t)
		err := mock.OnShutdown(ctx)
		assert.NoError(err)
	})

	t.Run("execute", func(t *testing.T) { // MARKER: Execute
		assert := testarossa.For(t)

		exampleCmd := "echo hello"
		exampleWorkDir := ""
		exampleStdin := ""
		exampleEnvars := map[string]string{}
		expectedExitCode := 0
		expectedStdout := "hello\n"
		expectedStderr := ""

		_, _, _, err := mock.Execute(ctx, exampleCmd, exampleWorkDir, exampleStdin, exampleEnvars)
		assert.Contains(err.Error(), "not implemented")
		mock.MockExecute(func(ctx context.Context, cmd string, workDir string, stdin string, envars map[string]string) (exitCode int, stdout string, stderr string, err error) {
			return expectedExitCode, expectedStdout, expectedStderr, nil
		})
		exitCode, stdout, stderr, err := mock.Execute(ctx, exampleCmd, exampleWorkDir, exampleStdin, exampleEnvars)
		assert.Expect(
			exitCode, expectedExitCode,
			stdout, expectedStdout,
			stderr, expectedStderr,
			err, nil,
		)
	})
}

func TestShell_ExecuteUnix(t *testing.T) { // MARKER: Execute
	if runtime.GOOS == "windows" {
		t.Skip("unix-only test")
	}
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("tester.client")
	client := shellapi.NewClient(tester)

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	t.Run("echo", func(t *testing.T) {
		assert := testarossa.For(t)

		exitCode, stdout, stderr, err := client.Execute(ctx, "echo hello", "", "", nil)
		assert.Expect(
			exitCode, 0,
			stdout, "hello\n",
			stderr, "",
			err, nil,
		)
	})

	t.Run("exit_code", func(t *testing.T) {
		assert := testarossa.For(t)

		exitCode, _, _, err := client.Execute(ctx, "exit 42", "", "", nil)
		assert.Expect(
			exitCode, 42,
			err, nil,
		)
	})

	t.Run("envars", func(t *testing.T) {
		assert := testarossa.For(t)

		exitCode, stdout, stderr, err := client.Execute(ctx, "echo $MY_VAR", "", "", map[string]string{"MY_VAR": "world"})
		assert.Expect(
			exitCode, 0,
			stdout, "world\n",
			stderr, "",
			err, nil,
		)
	})

	t.Run("envars_inherit_parent_env", func(t *testing.T) {
		assert := testarossa.For(t)

		// With caller envars supplied, PATH must still be inherited from the parent process.
		// If c.Env weren't seeded from os.Environ, ${#PATH} would expand to 0.
		exitCode, stdout, _, err := client.Execute(ctx, "printf 'caller=%s path_len=%d\\n' \"$MY_VAR\" \"${#PATH}\"", "", "", map[string]string{"MY_VAR": "from-caller"})
		assert.Expect(exitCode, 0, err, nil)
		assert.Contains(stdout, "caller=from-caller")
		assert.NotContains(stdout, "path_len=0")
	})

	t.Run("work_dir", func(t *testing.T) {
		assert := testarossa.For(t)

		tmpDir := t.TempDir()
		exitCode, stdout, stderr, err := client.Execute(ctx, "pwd", tmpDir, "", nil)
		assert.Expect(
			exitCode, 0,
			stdout, tmpDir+"\n",
			stderr, "",
			err, nil,
		)
	})

	t.Run("stdin", func(t *testing.T) {
		assert := testarossa.For(t)

		exitCode, stdout, stderr, err := client.Execute(ctx, "cat", "", "hello from stdin", nil)
		assert.Expect(
			exitCode, 0,
			stdout, "hello from stdin",
			stderr, "",
			err, nil,
		)
	})

	t.Run("output_truncation", func(t *testing.T) {
		assert := testarossa.For(t)

		err := svc.SetMaxOutputBytes(2048)
		assert.NoError(err)
		defer svc.SetMaxOutputBytes(262144)

		// printf 'X' 10000 times is ~10 KiB, well over the 2 KiB cap.
		exitCode, stdout, _, err := client.Execute(ctx, "printf 'X%.0s' $(seq 1 10000)", "", "", nil)
		assert.Expect(exitCode, 0, err, nil)
		assert.True(len(stdout) < 4096, "stdout should be bounded near the cap, got %d bytes", len(stdout))
		assert.Contains(stdout, "truncated")
		assert.True(strings.HasPrefix(stdout, "XXXX"), "head should be preserved")
		assert.True(strings.HasSuffix(stdout, "XXXX"), "tail should be preserved")
	})

	t.Run("metachar_passthrough", func(t *testing.T) {
		// Documents that the shell endpoint is `sh -c` semantics by design - metacharacters
		// run as expected. This is not a security claim; it pins the contract so a future
		// "sanitize input" change cannot silently break callers that rely on shell features.
		assert := testarossa.For(t)
		exitCode, stdout, _, err := client.Execute(ctx, "echo foo; echo bar", "", "", nil)
		assert.Expect(exitCode, 0, err, nil)
		assert.Equal(stdout, "foo\nbar\n")
	})

	t.Run("cancel_kills_pgroup", func(t *testing.T) {
		// A shell that backgrounds a grandchild writing to a sentinel file. When ctx fires,
		// `Setpgid` + `c.Cancel` SIGKILLs the entire pgroup - without it, the grandchild
		// outlives `sh` and keeps writing. We call svc.Execute directly so the test exercises
		// the cancel path without the bus indirection (caller-ctx doesn't propagate cleanly
		// through NATS short-circuit on cancel).
		assert := testarossa.For(t)

		dir := t.TempDir()
		sentinel := dir + "/heartbeat"
		cmd := "( while true; do echo tick >> " + sentinel + "; sleep 0.1; done ) & sleep 30"

		cancelCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		defer cancel()
		_, _, _, _ = svc.Execute(cancelCtx, cmd, "", "", nil)

		sizeAfterCancel := fileSize(t, sentinel)
		time.Sleep(500 * time.Millisecond)
		sizeLater := fileSize(t, sentinel)

		assert.Equal(sizeLater, sizeAfterCancel, "grandchild kept writing after cancel - pgroup not killed (after=%d, later=%d)", sizeAfterCancel, sizeLater)
	})
}

func fileSize(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

func TestShell_ExecuteWindows(t *testing.T) { // MARKER: Execute
	if runtime.GOOS != "windows" {
		t.Skip("windows-only test")
	}
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("tester.client")
	client := shellapi.NewClient(tester)

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	t.Run("echo", func(t *testing.T) {
		assert := testarossa.For(t)

		exitCode, stdout, stderr, err := client.Execute(ctx, "echo hello", "", "", nil)
		assert.Expect(
			exitCode, 0,
			stdout, "hello\r\n",
			stderr, "",
			err, nil,
		)
	})

	t.Run("exit_code", func(t *testing.T) {
		assert := testarossa.For(t)

		exitCode, _, _, err := client.Execute(ctx, "exit /B 42", "", "", nil)
		assert.Expect(
			exitCode, 42,
			err, nil,
		)
	})

	t.Run("envars", func(t *testing.T) {
		assert := testarossa.For(t)

		exitCode, stdout, stderr, err := client.Execute(ctx, "echo %MY_VAR%", "", "", map[string]string{"MY_VAR": "world"})
		assert.Expect(
			exitCode, 0,
			stdout, "world\r\n",
			stderr, "",
			err, nil,
		)
	})

	t.Run("envars_inherit_parent_env", func(t *testing.T) {
		assert := testarossa.For(t)

		// With caller envars supplied, PATH must still be inherited from the parent process.
		// If c.Env weren't seeded from os.Environ, %PATH% would expand literally on Windows.
		exitCode, stdout, _, err := client.Execute(ctx, "echo caller=%MY_VAR% path=%PATH%", "", "", map[string]string{"MY_VAR": "from-caller"})
		assert.Expect(exitCode, 0, err, nil)
		assert.Contains(stdout, "caller=from-caller")
		assert.NotContains(stdout, "path=%PATH%")
	})

	t.Run("work_dir", func(t *testing.T) {
		assert := testarossa.For(t)

		tmpDir := t.TempDir()
		exitCode, stdout, stderr, err := client.Execute(ctx, "cd", tmpDir, "", nil)
		assert.Expect(
			exitCode, 0,
			stdout, tmpDir+"\r\n",
			stderr, "",
			err, nil,
		)
	})

	t.Run("stdin", func(t *testing.T) {
		assert := testarossa.For(t)

		exitCode, stdout, stderr, err := client.Execute(ctx, "findstr \"^\"", "", "hello from stdin", nil)
		assert.Expect(
			exitCode, 0,
			stdout, "hello from stdin",
			stderr, "",
			err, nil,
		)
	})
}

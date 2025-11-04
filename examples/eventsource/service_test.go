/*
Copyright (c) 2023-2025 Microbus LLC and various contributors

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

package eventsource

import (
	"context"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/microbus-io/fabric/application"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/testarossa"

	"github.com/microbus-io/fabric/examples/eventsink"
	"github.com/microbus-io/fabric/examples/eventsource/eventsourceapi"
)

var (
	_ context.Context
	_ io.Closer
	_ http.Handler
	_ testing.TB
	_ *application.Application
	_ *connector.Connector
	_ *frame.Frame
	_ testarossa.TestingT
	_ *eventsourceapi.Client
)

func TestEventsource_Register(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("eventsource.register.tester")
	client := eventsourceapi.NewClient(tester)
	hook := eventsourceapi.NewHook(tester)

	// Run the testing app
	app := application.NewTesting()
	app.Add(
		// Add microservices or mocks required for this test
		eventsink.NewService(), // Disallows gmail.com and hotmail.com registrations
		svc,
		tester,
	)
	app.RunInTest(t)

	t.Run("blocked_by_sink_hotmail", func(t *testing.T) {
		assert := testarossa.For(t)
		// hotmail.com is disallowed by eventsink service
		allowed, err := client.Register(ctx, "brian@hotmail.com")
		assert.Expect(
			allowed, false,
			err, nil,
		)
	})

	t.Run("blocked_by_sink_gmail", func(t *testing.T) {
		assert := testarossa.For(t)
		// gmail.com is disallowed by eventsink service
		allowed, err := client.Register(ctx, "sarah@gmail.com")
		assert.Expect(
			allowed, false,
			err, nil,
		)
	})

	t.Run("allowed_example_domain", func(t *testing.T) {
		assert := testarossa.For(t)
		// example.com is allowed
		allowed, err := client.Register(ctx, "brian@example.com")
		assert.Expect(
			allowed, true,
			err, nil,
		)
	})

	t.Run("allowed_multiple_users", func(t *testing.T) {
		assert := testarossa.For(t)

		testCases := []string{
			"mandy@example.com",
			"john@company.org",
			"alice@test.io",
		}

		for _, email := range testCases {
			allowed, err := client.Register(ctx, email)
			assert.Expect(
				allowed, true,
				err, nil,
			)
		}
	})

	t.Run("case_insensitive_blocking", func(t *testing.T) {
		assert := testarossa.For(t)
		// Gmail with mixed case should still be blocked
		allowed, err := client.Register(ctx, "User@GMAIL.com")
		assert.Expect(
			allowed, false,
			err, nil,
		)
	})

	t.Run("invalid_email_format", func(t *testing.T) {
		assert := testarossa.For(t)
		// Invalid email should be rejected by sink
		allowed, err := client.Register(ctx, "invalid-email")
		assert.Expect(allowed, false, err, nil)
	})

	t.Run("partial_approval_is_blocked", func(t *testing.T) {
		assert := testarossa.For(t)

		hook.OnAllowRegister(func(ctx context.Context, email string) (allow bool, err error) {
			return false, nil
		})
		defer hook.OnAllowRegister(nil)

		// The eventsink service approves, but the local hook blocks
		allowed, err := client.Register(ctx, "mixed@example.com")
		assert.Expect(allowed, false, err, nil)
	})

	t.Run("sink_errors_out", func(t *testing.T) {
		assert := testarossa.For(t)

		hook.OnAllowRegister(func(ctx context.Context, email string) (allow bool, err error) {
			return false, errors.New("oops")
		})
		defer hook.OnAllowRegister(nil)

		_, err := client.Register(ctx, "error@example.com")
		assert.Contains(err, "oops")
	})
}

func TestEventsource_OnAllowRegister(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("eventsource.onallowregister.tester")
	client := eventsourceapi.NewClient(tester)
	trigger := eventsourceapi.NewMulticastTrigger(tester)
	hook := eventsourceapi.NewHook(tester)
	_ = client

	// Run the testing app
	app := application.NewTesting()
	app.Add(
		// Add microservices or mocks required for this test
		eventsink.NewService(), // Disallows gmail.com and hotmail.com registrations
		svc,
		tester,
	)
	app.RunInTest(t)

	t.Run("sink_allows_registration", func(t *testing.T) {
		assert := testarossa.For(t)

		testCases := []string{
			"barb@example.com",
			"admin@company.org",
			"dev@test.io",
		}

		for _, tc := range testCases {
			for i := range trigger.OnAllowRegister(ctx, tc) {
				allow, err := i.Get()
				assert.Expect(allow, true, err, nil)
			}
		}
	})

	t.Run("sink_blocks_registration", func(t *testing.T) {
		assert := testarossa.For(t)

		testCases := []string{
			"josh@gmail.com",
			"rachel@hotmail.com",
			"peter!example.com",
		}

		for _, tc := range testCases {
			for i := range trigger.OnAllowRegister(ctx, tc) {
				allow, err := i.Get()
				assert.Expect(allow, false, err, nil)
			}
		}
	})

	t.Run("hook_receives_event", func(t *testing.T) {
		assert := testarossa.For(t)

		// Install a hook to verify it receives the event
		hookReceived := false
		hook.OnAllowRegister(func(ctx context.Context, email string) (allow bool, err error) {
			assert.Expect(email, "test@example.com")
			hookReceived = true
			// Hook returns true, but sink will also be consulted
			return true, nil
		})
		defer hook.OnAllowRegister(nil)

		// Trigger the event - both hook and sink receive it
		count := 0
		for i := range trigger.OnAllowRegister(ctx, "test@example.com") {
			count++
			allow, err := i.Get()
			assert.Expect(allow, true, err, nil)
		}
		assert.True(hookReceived)
		assert.Equal(2, count)
	})

	t.Run("case_insensitive_blocking", func(t *testing.T) {
		assert := testarossa.For(t)

		testCases := []string{
			"User@GMAIL.COM",
		}

		for _, tc := range testCases {
			for i := range trigger.OnAllowRegister(ctx, tc) {
				allow, err := i.Get()
				assert.Expect(allow, false, err, nil)
			}
		}
	})
}

func TestEventsource_OnRegistered(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("eventsource.onregistered.tester")
	client := eventsourceapi.NewClient(tester)
	trigger := eventsourceapi.NewMulticastTrigger(tester)
	hook := eventsourceapi.NewHook(tester)
	_ = trigger

	// Run the testing app
	app := application.NewTesting()
	app.Add(
		// Add microservices or mocks required for this test
		eventsink.NewService(), // Disallows gmail.com and hotmail.com registrations
		svc,
		tester,
	)
	app.RunInTest(t)

	t.Run("allowed_registration_triggers_event", func(t *testing.T) {
		assert := testarossa.For(t)

		// Verify the OnRegistered event is fired with the correct email
		hook.OnRegistered(func(ctx context.Context, email string) (err error) {
			assert.Expect(email, "peter@example.com")
			return nil
		})
		defer hook.OnRegistered(nil)

		// Register a user
		allowed, err := client.Register(ctx, "peter@example.com")
		assert.Expect(
			allowed, true,
			err, nil,
		)
	})

	t.Run("blocked_registration_no_event", func(t *testing.T) {
		assert := testarossa.For(t)

		// OnRegistered should NOT fire for blocked registrations
		// The service only fires the event after successful registration
		hook.OnRegistered(func(ctx context.Context, email string) (err error) {
			t.Fatal("OnRegistered should not fire for blocked registrations")
			return nil
		})
		defer hook.OnRegistered(nil)

		// Try to register a blocked user
		allowed, err := client.Register(ctx, "paul@gmail.com")
		assert.Expect(
			allowed, false,
			err, nil,
		)
	})

	t.Run("multiple_registrations", func(t *testing.T) {
		assert := testarossa.For(t)

		// Track that events are fired for each registration
		expectedEmails := []string{"alice@example.com", "bob@company.org", "carol@test.io"}
		emailIndex := 0

		hook.OnRegistered(func(ctx context.Context, email string) (err error) {
			// Verify each email matches expected order
			if emailIndex < len(expectedEmails) {
				assert.Expect(email, expectedEmails[emailIndex])
				emailIndex++
			}
			return nil
		})
		defer hook.OnRegistered(nil)

		// Register multiple users
		for _, email := range expectedEmails {
			allowed, err := client.Register(ctx, email)
			assert.Expect(
				allowed, true,
				err, nil,
			)
		}
	})

	t.Run("event_receives_original_email", func(t *testing.T) {
		assert := testarossa.For(t)

		// Verify the event receives the email as submitted (not normalized)
		hook.OnRegistered(func(ctx context.Context, email string) (err error) {
			// The service passes the email as-is to the event
			// Normalization happens in the event sink
			assert.Expect(email, "David@EXAMPLE.com")
			return nil
		})
		defer hook.OnRegistered(nil)

		// Register with mixed case
		allowed, err := client.Register(ctx, "David@EXAMPLE.com")
		assert.Expect(
			allowed, true,
			err, nil,
		)
	})
}

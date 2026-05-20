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

package creditflow

import (
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/microbus-io/fabric/application"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/fabric/pub"
	"github.com/microbus-io/fabric/sub"
	"github.com/microbus-io/fabric/workflow"
	"github.com/microbus-io/testarossa"

	"github.com/microbus-io/fabric/coreservices/foreman"
	"github.com/microbus-io/fabric/coreservices/foreman/foremanapi"
	"github.com/microbus-io/fabric/examples/creditflow/creditflowapi"
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
	_ *workflow.Flow
	_ testarossa.Asserter
	_ creditflowapi.Client
)

// MARKER: SubmitCreditApplication

// MARKER: VerifyCredit

// MARKER: VerifyEmployment

// MARKER: VerifySSN

// MARKER: VerifyAddress

// MARKER: VerifyPhoneNumber

// MARKER: IdentityDecision

// MARKER: HandleCreditError

// MARKER: Decision

// MARKER: RequestMoreInfo

// MARKER: ReviewCredit

// MARKER: CreditApproval

// Mock the workflow behavior

// Graph endpoint should now return a valid graph

// MARKER: IdentityVerification

// Mock the workflow behavior

// Graph endpoint should now return a valid graph

// MARKER: Demo

func TestCreditFlow_SubmitCreditApplication(t *testing.T) { // MARKER: SubmitCreditApplication
	t.Parallel()
	ctx := t.Context()
	_ = ctx

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("tester.client")
	exec := creditflowapi.NewExecutor(tester)

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	/*
		HINT: Use the following pattern for each test case.
		Use WithOutputFlow to also verify control signals (Goto, Retry, Interrupt, Sleep) if applicable.

		t.Run("test_case_name", func(t *testing.T) {
			assert := testarossa.For(t)

			var outFlow workflow.Flow
			_, _, _, _, _, err := exec.WithOutputFlow(&outFlow).SubmitCreditApplication(ctx, creditflowapi.Applicant{...})
			assert.NoError(err)
		})
	*/

	t.Run("submit", func(t *testing.T) {
		assert := testarossa.For(t)

		applicant := creditflowapi.Applicant{
			ApplicantName: "Alice",
			SSN:           "123-45-6789",
			Address:       "123 Main St",
			Phone:         "555-123-4567",
			Employers:     []string{"Acme Corp", "Globex"},
			CreditScore:   750,
		}
		applicantName, ssn, address, phone, employers, creditScore, err := exec.SubmitCreditApplication(ctx, applicant)
		if assert.NoError(err) {
			assert.Expect(
				applicantName, "Alice",
				ssn, "123-45-6789",
				address, "123 Main St",
				phone, "555-123-4567",
				employers, []string{"Acme Corp", "Globex"},
				creditScore, 750,
			)
		}
	})
}

func TestCreditFlow_VerifyCredit(t *testing.T) { // MARKER: VerifyCredit
	t.Parallel()
	ctx := t.Context()
	_ = ctx

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("tester.client")
	exec := creditflowapi.NewExecutor(tester)

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	/*
		HINT: Use the following pattern for each test case.
		Use WithOutputFlow to also verify control signals (Goto, Retry, Interrupt, Sleep) if applicable.

		t.Run("test_case_name", func(t *testing.T) {
			assert := testarossa.For(t)

			var outFlow workflow.Flow
			creditVerified, err := exec.WithOutputFlow(&outFlow).VerifyCredit(ctx, creditScore)
			if assert.NoError(err) {
				assert.Expect(creditVerified, true)
			}
		})
	*/

	t.Run("good_score", func(t *testing.T) {
		assert := testarossa.For(t)

		creditVerified, err := exec.VerifyCredit(ctx, 750)
		if assert.NoError(err) {
			assert.Expect(creditVerified, true)
		}
	})

	t.Run("bad_score", func(t *testing.T) {
		assert := testarossa.For(t)

		creditVerified, err := exec.VerifyCredit(ctx, 540)
		if assert.NoError(err) {
			assert.Expect(creditVerified, false)
		}
	})
}

func TestCreditFlow_VerifyEmployment(t *testing.T) { // MARKER: VerifyEmployment
	t.Parallel()
	ctx := t.Context()
	_ = ctx

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("tester.client")
	exec := creditflowapi.NewExecutor(tester)

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	/*
		HINT: Use the following pattern for each test case.
		Use WithOutputFlow to also verify control signals (Goto, Retry, Interrupt, Sleep) if applicable.

		t.Run("test_case_name", func(t *testing.T) {
			assert := testarossa.For(t)

			var outFlow workflow.Flow
			sumEmploymentFailuresOut, err := exec.WithOutputFlow(&outFlow).VerifyEmployment(ctx, applicantName, employerName)
			if assert.NoError(err) {
				assert.Expect(sumEmploymentFailuresOut, 0)
			}
		})
	*/

	t.Run("employed", func(t *testing.T) {
		assert := testarossa.For(t)

		sumEmploymentFailuresOut, err := exec.VerifyEmployment(ctx, "Alice", "Acme Corp")
		if assert.NoError(err) {
			assert.Expect(sumEmploymentFailuresOut, 0)
		}
	})

	t.Run("empty_name", func(t *testing.T) {
		assert := testarossa.For(t)

		sumEmploymentFailuresOut, err := exec.VerifyEmployment(ctx, "Alice", "")
		if assert.NoError(err) {
			assert.Expect(sumEmploymentFailuresOut, 1)
		}
	})
}

func TestCreditFlow_VerifySSN(t *testing.T) { // MARKER: VerifySSN
	t.Parallel()
	ctx := t.Context()

	svc := NewService()
	tester := connector.New("tester.client")
	exec := creditflowapi.NewExecutor(tester)

	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	/*
		HINT: Use the following pattern for each test case.
		Use WithOutputFlow to also verify control signals (Goto, Retry, Interrupt, Sleep) if applicable.

		t.Run("test_case_name", func(t *testing.T) {
			assert := testarossa.For(t)

			var outFlow workflow.Flow
			ssnVerified, err := exec.WithOutputFlow(&outFlow).VerifySSN(ctx, ssn)
			if assert.NoError(err) {
				assert.Expect(ssnVerified, true)
			}
		})
	*/

	t.Run("valid", func(t *testing.T) {
		assert := testarossa.For(t)
		ssnVerified, err := exec.VerifySSN(ctx, "123-45-6789")
		if assert.NoError(err) {
			assert.Expect(ssnVerified, true)
		}
	})

	t.Run("empty_ssn", func(t *testing.T) {
		assert := testarossa.For(t)
		ssnVerified, err := exec.VerifySSN(ctx, "")
		if assert.NoError(err) {
			assert.Expect(ssnVerified, false)
		}
	})
}

func TestCreditFlow_VerifyAddress(t *testing.T) { // MARKER: VerifyAddress
	t.Parallel()
	ctx := t.Context()

	svc := NewService()
	tester := connector.New("tester.client")
	exec := creditflowapi.NewExecutor(tester)

	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	/*
		HINT: Use the following pattern for each test case.
		Use WithOutputFlow to also verify control signals (Goto, Retry, Interrupt, Sleep) if applicable.

		t.Run("test_case_name", func(t *testing.T) {
			assert := testarossa.For(t)

			var outFlow workflow.Flow
			addressVerified, err := exec.WithOutputFlow(&outFlow).VerifyAddress(ctx, address)
			if assert.NoError(err) {
				assert.Expect(addressVerified, true)
			}
		})
	*/

	t.Run("valid", func(t *testing.T) {
		assert := testarossa.For(t)
		addressVerified, err := exec.VerifyAddress(ctx, "123 Main St")
		if assert.NoError(err) {
			assert.Expect(addressVerified, true)
		}
	})
}

func TestCreditFlow_VerifyPhoneNumber(t *testing.T) { // MARKER: VerifyPhoneNumber
	t.Parallel()
	ctx := t.Context()

	svc := NewService()
	tester := connector.New("tester.client")
	exec := creditflowapi.NewExecutor(tester)

	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	/*
		HINT: Use the following pattern for each test case.
		Use WithOutputFlow to also verify control signals (Goto, Retry, Interrupt, Sleep) if applicable.

		t.Run("test_case_name", func(t *testing.T) {
			assert := testarossa.For(t)

			var outFlow workflow.Flow
			phoneVerified, err := exec.WithOutputFlow(&outFlow).VerifyPhoneNumber(ctx, phone)
			if assert.NoError(err) {
				assert.Expect(phoneVerified, true)
			}
		})
	*/

	t.Run("valid", func(t *testing.T) {
		assert := testarossa.For(t)
		phoneVerified, err := exec.VerifyPhoneNumber(ctx, "555-123-4567")
		if assert.NoError(err) {
			assert.Expect(phoneVerified, true)
		}
	})
}

func TestCreditFlow_IdentityDecision(t *testing.T) { // MARKER: IdentityDecision
	t.Parallel()
	ctx := t.Context()

	svc := NewService()
	tester := connector.New("tester.client")
	exec := creditflowapi.NewExecutor(tester)

	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	/*
		HINT: Use the following pattern for each test case.
		Use WithOutputFlow to also verify control signals (Goto, Retry, Interrupt, Sleep) if applicable.

		t.Run("test_case_name", func(t *testing.T) {
			assert := testarossa.For(t)

			var outFlow workflow.Flow
			identityVerified, err := exec.WithOutputFlow(&outFlow).IdentityDecision(ctx, ssnVerified, addressVerified, phoneVerified)
			if assert.NoError(err) {
				assert.Expect(identityVerified, true)
			}
		})
	*/

	t.Run("all_verified", func(t *testing.T) {
		assert := testarossa.For(t)
		identityVerified, err := exec.IdentityDecision(ctx, true, true, true)
		if assert.NoError(err) {
			assert.Expect(identityVerified, true)
		}
	})

	t.Run("ssn_failed", func(t *testing.T) {
		assert := testarossa.For(t)
		identityVerified, err := exec.IdentityDecision(ctx, false, true, true)
		if assert.NoError(err) {
			assert.Expect(identityVerified, false)
		}
	})
}

func TestCreditFlow_RequestMoreInfo(t *testing.T) { // MARKER: RequestMoreInfo
	t.Parallel()
	ctx := t.Context()
	_ = ctx

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("tester.client")
	exec := creditflowapi.NewExecutor(tester)

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	/*
		HINT: Use the following pattern for each test case.
		Use WithOutputFlow to also verify control signals (Goto, Retry, Interrupt, Sleep) if applicable.

		t.Run("test_case_name", func(t *testing.T) {
			assert := testarossa.For(t)

			var outFlow workflow.Flow
			reviewAttemptsOut, err := exec.WithOutputFlow(&outFlow).RequestMoreInfo(ctx, reviewAttempts)
			if assert.NoError(err) {
				assert.Expect(reviewAttemptsOut, 1)
			}
		})
	*/

	t.Run("increments_counter", func(t *testing.T) {
		assert := testarossa.For(t)

		reviewAttemptsOut, err := exec.RequestMoreInfo(ctx, 0)
		if assert.NoError(err) {
			assert.Expect(reviewAttemptsOut, 1)
		}
	})

	t.Run("increments_again", func(t *testing.T) {
		assert := testarossa.For(t)

		reviewAttemptsOut, err := exec.RequestMoreInfo(ctx, 1)
		if assert.NoError(err) {
			assert.Expect(reviewAttemptsOut, 2)
		}
	})
}

func TestCreditFlow_ReviewCredit(t *testing.T) { // MARKER: ReviewCredit
	t.Parallel()
	ctx := t.Context()
	_ = ctx

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("tester.client")
	exec := creditflowapi.NewExecutor(tester)

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	/*
		HINT: Use the following pattern for each test case.
		Use WithOutputFlow to also verify control signals (Goto, Retry, Interrupt, Sleep) if applicable.

		t.Run("test_case_name", func(t *testing.T) {
			assert := testarossa.For(t)

			var outFlow workflow.Flow
			creditVerifiedOut, err := exec.WithOutputFlow(&outFlow).ReviewCredit(ctx, creditScore, creditVerified, reviewAttempts)
			if assert.NoError(err) {
				assert.Expect(creditVerifiedOut, true)
			}
		})
	*/

	t.Run("high_borderline_approved", func(t *testing.T) {
		assert := testarossa.For(t)

		creditVerifiedOut, err := exec.ReviewCredit(ctx, 580, false, 0)
		if assert.NoError(err) {
			assert.Expect(creditVerifiedOut, true)
		}
	})

	t.Run("too_low_rejected", func(t *testing.T) {
		assert := testarossa.For(t)

		creditVerifiedOut, err := exec.ReviewCredit(ctx, 400, false, 0)
		if assert.NoError(err) {
			assert.Expect(creditVerifiedOut, false)
		}
	})
}

func TestCreditFlow_Decision(t *testing.T) { // MARKER: Decision
	t.Parallel()
	ctx := t.Context()
	_ = ctx

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("tester.client")
	exec := creditflowapi.NewExecutor(tester)

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	/*
		HINT: Use the following pattern for each test case.
		Use WithOutputFlow to also verify control signals (Goto, Retry, Interrupt, Sleep) if applicable.

		t.Run("test_case_name", func(t *testing.T) {
			assert := testarossa.For(t)

			var outFlow workflow.Flow
			approved, err := exec.WithOutputFlow(&outFlow).Decision(ctx, creditVerified, sumEmploymentFailures, identityVerified)
			if assert.NoError(err) {
				assert.Expect(approved, true)
			}
		})
	*/

	t.Run("all_pass", func(t *testing.T) {
		assert := testarossa.For(t)

		approved, err := exec.Decision(ctx, true, 0, true)
		if assert.NoError(err) {
			assert.Expect(approved, true)
		}
	})

	t.Run("credit_fails", func(t *testing.T) {
		assert := testarossa.For(t)

		approved, err := exec.Decision(ctx, false, 0, true)
		if assert.NoError(err) {
			assert.Expect(approved, false)
		}
	})
}

func TestCreditFlow_CreditApproval(t *testing.T) { // MARKER: CreditApproval
	ctx := t.Context()
	_ = ctx

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("tester.client")
	foremanClient := foremanapi.NewClient(tester)
	exec := creditflowapi.NewExecutor(tester).WithWorkflowRunner(foremanClient)

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		foreman.NewService(),
		tester,
	)
	app.RunInTest(t)

	/*
		HINT: Use the following pattern for each test case.
		Use WithOutputState to also inspect the full state map if applicable.

		t.Run("test_case_name", func(t *testing.T) {
			assert := testarossa.For(t)

			approved, creditVerified, sumEmploymentFailures, identityVerified, status, err := exec.CreditApproval(ctx, creditflowapi.Applicant{...})
			assert.Expect(
				err, nil,
				status, workflow.StatusCompleted,
				approved, expectedApproved,
			)
		})
	*/

	t.Run("good_score_approved", func(t *testing.T) {
		assert := testarossa.For(t)

		// Score 750: passes VerifyCredit, review passes through, approved
		approved, creditVerified, _, identityVerified, status, err := exec.CreditApproval(ctx, creditflowapi.Applicant{
			ApplicantName: "Alice",
			SSN:           "123-45-6789",
			Address:       "123 Main St",
			Phone:         "555-123-4567",
			Employers:     []string{"Acme Corp", "Globex"},
			CreditScore:   750,
		})
		if assert.NoError(err) {
			assert.Expect(
				status, workflow.StatusCompleted,
				approved, true,
				creditVerified, true,
				identityVerified, true,
			)
		}
	})

	t.Run("low_score_rejected", func(t *testing.T) {
		assert := testarossa.For(t)

		// Score 400: fails VerifyCredit, review cannot save it, rejected
		approved, _, _, _, status, err := exec.CreditApproval(ctx, creditflowapi.Applicant{
			ApplicantName: "Bob",
			SSN:           "987-65-4321",
			Address:       "456 Oak Ave",
			Phone:         "555-987-6543",
			Employers:     []string{"Globex"},
			CreditScore:   400,
		})
		if assert.NoError(err) {
			assert.Expect(
				status, workflow.StatusCompleted,
				approved, false,
			)
		}
	})

	t.Run("borderline_with_review_approved", func(t *testing.T) {
		assert := testarossa.For(t)

		// Score 580: passes VerifyCredit, routed to ReviewCredit which approves 580+
		approved, creditVerified, _, _, status, err := exec.CreditApproval(ctx, creditflowapi.Applicant{
			ApplicantName: "Charlie",
			SSN:           "111-22-3333",
			Address:       "789 Elm Dr",
			Phone:         "(555) 111-2222",
			Employers:     []string{"Initech"},
			CreditScore:   580,
		})
		if assert.NoError(err) {
			assert.Expect(
				status, workflow.StatusCompleted,
				approved, true,
				creditVerified, true,
			)
		}
	})

	t.Run("very_borderline_goto_loop", func(t *testing.T) {
		assert := testarossa.For(t)

		// Score 560: passes VerifyCredit, review requests more info twice via goto, then approves
		approved, creditVerified, _, _, status, err := exec.CreditApproval(ctx, creditflowapi.Applicant{
			ApplicantName: "Diana",
			SSN:           "444-55-6666",
			Address:       "321 Pine Ln",
			Phone:         "555-444-3333",
			Employers:     []string{"Umbrella Corp"},
			CreditScore:   560,
		})
		if assert.NoError(err) {
			assert.Expect(
				status, workflow.StatusCompleted,
				approved, true,
				creditVerified, true,
			)
		}
	})

	t.Run("employment_failure", func(t *testing.T) {
		assert := testarossa.For(t)

		// Good credit but one empty employer name causes employment failure
		approved, _, _, _, status, err := exec.CreditApproval(ctx, creditflowapi.Applicant{
			ApplicantName: "Eve",
			SSN:           "555-66-7777",
			Address:       "654 Maple Ct",
			Phone:         "555-555-5555",
			Employers:     []string{"Acme Corp", ""},
			CreditScore:   750,
		})
		if assert.NoError(err) {
			assert.Expect(
				status, workflow.StatusCompleted,
				approved, false,
			)
		}
	})

	t.Run("empty_applicant_rejected", func(t *testing.T) {
		assert := testarossa.For(t)

		// Empty applicant name, invalid address, and bad phone fail identity verification
		approved, _, _, identityVerified, status, err := exec.CreditApproval(ctx, creditflowapi.Applicant{
			SSN:         "000-00-0000",
			Address:     "Nowhere",
			Phone:       "bad",
			Employers:   []string{"Acme Corp"},
			CreditScore: 750,
		})
		if assert.NoError(err) {
			assert.Expect(
				status, workflow.StatusCompleted,
				approved, false,
				identityVerified, false,
			)
		}
	})

	// time_budget_exceeded: covered by verify/timebudgetflow.

	t.Run("breakpoint", func(t *testing.T) {
		assert := testarossa.For(t)

		foremanClient := foremanapi.NewClient(tester)

		// Create the flow
		flowKey, err := foremanClient.Create(ctx, creditflowapi.CreditApproval.URL(), creditflowapi.CreditApprovalIn{
			Applicant: creditflowapi.Applicant{
				ApplicantName: "Frank",
				SSN:           "123-45-6789",
				Address:       "123 Main St",
				Phone:         "555-123-4567",
				Employers:     []string{"Acme Corp"},
				CreditScore:   750,
			},
		}, nil)

		assert.NoError(err)

		// Set a breakpoint on the ReviewCredit task (runs after fan-in, before Decision)
		err = foremanClient.BreakBefore(ctx, flowKey, "reviewCredit", true)
		assert.NoError(err)

		// Start the flow
		err = foremanClient.Start(ctx, flowKey)
		assert.NoError(err)

		// Wait for the flow to hit the breakpoint (interrupted status)
		outcome, err := foremanClient.Await(ctx, flowKey)

		status, state := outcomeStatusState(outcome)
		assert.Expect(
			err, nil,
			status, workflow.StatusInterrupted,
			state["creditVerified"], true, // The flow should be paused before Decision, so creditVerified should be set from prior steps
		)

		// Resume the flow past the breakpoint
		err = foremanClient.Resume(ctx, flowKey, nil)
		assert.NoError(err)

		// Wait for completion
		outcome, err = foremanClient.Await(ctx, flowKey)

		status, state = outcomeStatusState(outcome)
		assert.Expect(
			err, nil,
			status, workflow.StatusCompleted,
			state["approved"], true,
		)
	})

	t.Run("cancel_with_subgraph", func(t *testing.T) {
		assert := testarossa.For(t)

		foremanClient := foremanapi.NewClient(tester)

		// Create the parent flow and set a breakpoint on IdentityDecision
		// (a task inside the identity-verification subgraph).
		// Breakpoints are copied from parent to child on subgraph creation.
		flowKey, err := foremanClient.Create(ctx, creditflowapi.CreditApproval.URL(), creditflowapi.CreditApprovalIn{
			Applicant: creditflowapi.Applicant{
				ApplicantName: "Grace",
				SSN:           "123-45-6789",
				Address:       "123 Main St",
				Phone:         "555-123-4567",
				Employers:     []string{"Acme Corp"},
				CreditScore:   750,
			},
		}, nil)

		assert.NoError(err)
		err = foremanClient.BreakBefore(ctx, flowKey, creditflowapi.IdentityDecision.URL(), true)
		assert.NoError(err)

		// Start the parent flow
		err = foremanClient.Start(ctx, flowKey)
		assert.NoError(err)

		// The subgraph breakpoint propagates up - the parent flow becomes interrupted
		outcome, err := foremanClient.Await(ctx, flowKey)

		status := outcomeStatus(outcome)
		if !assert.Expect(err, nil, status, workflow.StatusInterrupted) {
			return
		}

		// Cancel the parent flow - cancellation cascades to the subgraph
		err = foremanClient.Cancel(ctx, flowKey, "")
		assert.NoError(err)

		// Verify the parent is cancelled
		outcome, err = foremanClient.Await(ctx, flowKey)

		parentStatus := outcomeStatus(outcome)
		assert.Expect(
			err, nil,
			parentStatus, workflow.StatusCancelled,
		)
	})

	// The following fault-injection subtests were removed and are covered by:
	//   subgraph_interrupt_resume -> verify/interruptflow + verify/subgraphflow
	//   retry                     -> verify/retryflow
	//   sleep                     -> verify/sleepflow
	//   bad_goto                  -> verify/gotoflow (BadGoto workflow)
	//   error_transition          -> verify/errorflow + verify/fanouterrorflow
}

// TestCreditFlow_DynamicSubgraph: covered by verify/dynamicsubgraphflow.

func TestCreditFlow_StartNotify(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("tester.client")
	foremanClient := foremanapi.NewClient(tester)

	// Run the testing app
	app := application.New()
	app.Add(svc, foreman.NewService(), tester)
	app.RunInTest(t)

	// Subscribe to OnFlowStopped targeted at the tester's hostname
	type notification struct {
		flowKey  string
		status   string
		snapshot map[string]any
	}
	notifyCh := make(chan notification, 10)
	assert := testarossa.For(t)
	unsub, err := foremanapi.NewHook(tester).ForHost(tester.Hostname()).OnFlowStopped(
		func(ctx context.Context, outcome *workflow.FlowOutcome) (err error) {
			if outcome == nil {
				return nil
			}
			notifyCh <- notification{flowKey: outcome.FlowKey, status: outcome.Status, snapshot: outcome.State}
			return nil
		},
	)
	assert.NoError(err)
	defer unsub()

	waitForNotification := func(assert *testarossa.Asserter, flowKey string, expectedStatus string) notification {
		for {
			select {
			case n := <-notifyCh:
				if n.flowKey == flowKey && n.status == expectedStatus {
					return n
				}
			case <-time.After(10 * time.Second):
				assert.True(false, "timeout waiting for %s notification on flow %s", expectedStatus, flowKey)
				return notification{}
			}
		}
	}

	goodApplicant := creditflowapi.Applicant{
		ApplicantName: "Alice",
		SSN:           "123-45-6789",
		Address:       "123 Main St",
		Phone:         "555-123-4567",
		Employers:     []string{"Acme Corp"},
		CreditScore:   750,
	}

	t.Run("completed", func(t *testing.T) {
		assert := testarossa.For(t)

		flowKey, err := foremanClient.Create(ctx, creditflowapi.CreditApproval.URL(), creditflowapi.CreditApprovalIn{
			Applicant: goodApplicant,
		}, nil)

		assert.NoError(err)
		err = foremanClient.StartNotify(ctx, flowKey, tester.Hostname())
		assert.NoError(err)

		n := waitForNotification(assert, flowKey, workflow.StatusCompleted)
		assert.Expect(n.status, workflow.StatusCompleted)
		assert.NotNil(n.snapshot)
	})

	// interrupted: covered by verify/interruptflow.

	t.Run("cancelled", func(t *testing.T) {
		assert := testarossa.For(t)

		// Use a breakpoint to pause the flow, then cancel it
		flowKey, err := foremanClient.Create(ctx, creditflowapi.CreditApproval.URL(), creditflowapi.CreditApprovalIn{
			Applicant: goodApplicant,
		}, nil)

		assert.NoError(err)
		err = foremanClient.BreakBefore(ctx, flowKey, "reviewCredit", true)
		assert.NoError(err)

		err = foremanClient.StartNotify(ctx, flowKey, tester.Hostname())
		assert.NoError(err)

		// Wait for breakpoint interrupt notification
		waitForNotification(assert, flowKey, workflow.StatusInterrupted)

		// Cancel the flow
		err = foremanClient.Cancel(ctx, flowKey, "")
		assert.NoError(err)

		n := waitForNotification(assert, flowKey, workflow.StatusCancelled)
		assert.Expect(n.status, workflow.StatusCancelled)
		assert.NotNil(n.snapshot)
	})

	// failed: covered by verify/timebudgetflow (foreman timeout) and verify/retryflow (retry exhaustion).
}

func TestCreditFlow_Demo(t *testing.T) { // MARKER: Demo
	t.Parallel()
	ctx := t.Context()
	_ = ctx

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the tester client
	tester := connector.New("tester.client")
	client := creditflowapi.NewClient(tester)
	_ = client

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		foreman.NewService(),
		tester,
	)
	app.RunInTest(t)

	/*
		HINT: Use the following pattern for each test case

		t.Run("test_case_name", func(t *testing.T) {
			assert := testarossa.For(t)

			actor := jwt.MapClaims{}
			res, err := client.WithOptions(pub.Actor(actor)).Demo(ctx, "GET", "", payload)
			if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
				body, err := io.ReadAll(res.Body)
				if assert.NoError(err) {
					assert.HTMLMatch(body, "DIV.class > DIV#id", "substring")
					assert.Contains(body, "substring")
				}
			}
		})
	*/

	t.Run("get_form", func(t *testing.T) {
		assert := testarossa.For(t)

		res, err := client.Demo(ctx, "GET", "", nil)
		if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if assert.NoError(err) {
				assert.Contains(body, "Credit Approval Workflow Demo")
				assert.Contains(body, "Alice")
			}
		}
	})

	t.Run("post_happy_path", func(t *testing.T) {
		assert := testarossa.For(t)

		form := "name=Alice&ssn=123-45-6789&address=123+Main+St&phone=555-123-4567&employers=Acme+Corp&score=750&fault="
		res, err := client.Demo(ctx, "POST", "", form)
		if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if assert.NoError(err) {
				assert.Contains(body, "completed")
				assert.Contains(body, "submitCreditApplication")
			}
		}
	})
}

func BenchmarkCreditFlow_CreditApproval(b *testing.B) {
	svc := NewService()
	tester := connector.New("tester.client")
	foremanClient := foremanapi.NewClient(tester)

	app := application.New()
	app.Add(svc, foreman.NewService(), tester)
	app.RunInTest(b)

	ctx := b.Context()
	applicant := creditflowapi.CreditApprovalIn{
		Applicant: creditflowapi.Applicant{
			ApplicantName: "Alice",
			SSN:           "123-45-6789",
			Address:       "123 Main St",
			Phone:         "555-123-4567",
			Employers:     []string{"Acme Corp"},
			CreditScore:   750,
		},
	}

	b.ResetTimer()
	for b.Loop() {
		outcome, err := foremanClient.Run(ctx, creditflowapi.CreditApproval.URL(), applicant, nil)

		_ = outcomeStatus(outcome)
		if err != nil {
			b.Fatal(err)
		}
	}

	/*
		Postgres: approx 12.8 workflows/sec or 141 tasks/sec

		goos: darwin
		goarch: arm64
		pkg: github.com/microbus-io/fabric/examples/creditflow
		cpu: Apple M1 Pro
		BenchmarkCreditFlow_CreditApproval-10    	      13	  78078000 ns/op	 1831651 B/op	   23091 allocs/op
	*/
}

func BenchmarkCreditFlow_CreditApprovalParallel(b *testing.B) {
	svc := NewService()
	tester := connector.New("tester.client")
	foremanClient := foremanapi.NewClient(tester)

	app := application.New()
	app.Add(svc, foreman.NewService(), tester)
	app.RunInTest(b)

	ctx := b.Context()
	applicant := creditflowapi.CreditApprovalIn{
		Applicant: creditflowapi.Applicant{
			ApplicantName: "Alice",
			SSN:           "123-45-6789",
			Address:       "123 Main St",
			Phone:         "555-123-4567",
			Employers:     []string{"Acme Corp"},
			CreditScore:   750,
		},
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			outcome, err := foremanClient.Run(ctx, creditflowapi.CreditApproval.URL(), applicant, nil)

			_ = outcomeStatus(outcome)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	/*
		Postgres: approx 38.5 workflows/sec or 424 tasks/sec

		goos: darwin
		goarch: arm64
		pkg: github.com/microbus-io/fabric/examples/creditflow
		cpu: Apple M1 Pro
		BenchmarkCreditFlow_CreditApprovalParallel-10    	     39	  25952910 ns/op	 1589837 B/op	   20449 allocs/op

		SQLite: approx 110 workflows/sec or 1206 tasks/sec

		goos: darwin
		goarch: arm64
		pkg: github.com/microbus-io/fabric/examples/creditflow
		cpu: Apple M1 Pro
		BenchmarkCreditFlow_CreditApprovalParallel-10    	    150	   9121310 ns/op	 1395101 B/op	   19108 allocs/op
	*/
}

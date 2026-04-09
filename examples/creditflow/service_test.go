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

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/application"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/fabric/httpx"
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

func TestCreditFlow_Mock(t *testing.T) {
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

	t.Run("submit_credit_application", func(t *testing.T) { // MARKER: SubmitCreditApplication
		assert := testarossa.For(t)

		applicant := creditflowapi.Applicant{
			ApplicantName: "Alice",
			SSN:           "123-45-6789",
			Address:       "123 Main St",
			Phone:         "555-123-4567",
			Employers:     []string{"Acme Corp"},
			CreditScore:   700,
		}
		_, _, _, _, _, _, err := mock.SubmitCreditApplication(ctx, nil, applicant)
		assert.Contains(err.Error(), "not implemented")
		mock.MockSubmitCreditApplication(func(ctx context.Context, flow *workflow.Flow, applicant creditflowapi.Applicant) (applicantName string, ssn string, address string, phone string, employers []string, creditScore int, err error) {
			return applicant.ApplicantName, applicant.SSN, applicant.Address, applicant.Phone, applicant.Employers, applicant.CreditScore, nil
		})
		_, _, _, _, _, _, err = mock.SubmitCreditApplication(ctx, nil, applicant)
		assert.NoError(err)
	})

	t.Run("verify_credit", func(t *testing.T) { // MARKER: VerifyCredit
		assert := testarossa.For(t)

		_, err := mock.VerifyCredit(ctx, nil, 700, "")
		assert.Contains(err.Error(), "not implemented")
		mock.MockVerifyCredit(func(ctx context.Context, flow *workflow.Flow, creditScore int, faultInjection string) (creditVerified bool, err error) {
			return true, nil
		})
		creditVerified, err := mock.VerifyCredit(ctx, nil, 700, "")
		assert.Expect(
			creditVerified, true,
			err, nil,
		)
	})

	t.Run("verify_employment", func(t *testing.T) { // MARKER: VerifyEmployment
		assert := testarossa.For(t)

		_, err := mock.VerifyEmployment(ctx, nil, "Alice", "Acme Corp")
		assert.Contains(err.Error(), "not implemented")
		mock.MockVerifyEmployment(func(ctx context.Context, flow *workflow.Flow, applicantName string, employerName string) (employmentFailures int, err error) {
			return 0, nil
		})
		employmentFailures, err := mock.VerifyEmployment(ctx, nil, "Alice", "Acme Corp")
		assert.Expect(
			employmentFailures, 0,
			err, nil,
		)
	})

	t.Run("verify_ssn", func(t *testing.T) { // MARKER: VerifySSN
		assert := testarossa.For(t)

		_, err := mock.VerifySSN(ctx, nil, "123-45-6789", "")
		assert.Contains(err.Error(), "not implemented")
		mock.MockVerifySSN(func(ctx context.Context, flow *workflow.Flow, ssn string, faultInjection string) (ssnVerified bool, err error) {
			return true, nil
		})
		ssnVerified, err := mock.VerifySSN(ctx, nil, "123-45-6789", "")
		assert.Expect(
			ssnVerified, true,
			err, nil,
		)
	})

	t.Run("verify_address", func(t *testing.T) { // MARKER: VerifyAddress
		assert := testarossa.For(t)

		_, err := mock.VerifyAddress(ctx, nil, "123 Main St")
		assert.Contains(err.Error(), "not implemented")
		mock.MockVerifyAddress(func(ctx context.Context, flow *workflow.Flow, address string) (addressVerified bool, err error) {
			return true, nil
		})
		addressVerified, err := mock.VerifyAddress(ctx, nil, "123 Main St")
		assert.Expect(
			addressVerified, true,
			err, nil,
		)
	})

	t.Run("verify_phone_number", func(t *testing.T) { // MARKER: VerifyPhoneNumber
		assert := testarossa.For(t)

		_, err := mock.VerifyPhoneNumber(ctx, nil, "555-123-4567", "")
		assert.Contains(err.Error(), "not implemented")
		mock.MockVerifyPhoneNumber(func(ctx context.Context, flow *workflow.Flow, phone string, faultInjection string) (phoneVerified bool, err error) {
			return true, nil
		})
		phoneVerified, err := mock.VerifyPhoneNumber(ctx, nil, "555-123-4567", "")
		assert.Expect(
			phoneVerified, true,
			err, nil,
		)
	})

	t.Run("identity_decision", func(t *testing.T) { // MARKER: IdentityDecision
		assert := testarossa.For(t)

		_, err := mock.IdentityDecision(ctx, nil, true, true, true)
		assert.Contains(err.Error(), "not implemented")
		mock.MockIdentityDecision(func(ctx context.Context, flow *workflow.Flow, ssnVerified bool, addressVerified bool, phoneVerified bool) (identityVerified bool, err error) {
			return ssnVerified && addressVerified && phoneVerified, nil
		})
		identityVerified, err := mock.IdentityDecision(ctx, nil, true, true, true)
		assert.Expect(
			identityVerified, true,
			err, nil,
		)
	})

	t.Run("handle_credit_error", func(t *testing.T) { // MARKER: HandleCreditError
		assert := testarossa.For(t)

		_, err := mock.HandleCreditError(ctx, nil, nil)
		assert.Contains(err.Error(), "not implemented")
		mock.MockHandleCreditError(func(ctx context.Context, flow *workflow.Flow, onErr *errors.TracedError) (creditVerified bool, err error) {
			return false, nil
		})
		creditVerified, err := mock.HandleCreditError(ctx, nil, errors.Convert(errors.New("test error")))
		assert.Expect(
			creditVerified, false,
			err, nil,
		)
	})

	t.Run("decision", func(t *testing.T) { // MARKER: Decision
		assert := testarossa.For(t)

		_, err := mock.Decision(ctx, nil, true, 0, true)
		assert.Contains(err.Error(), "not implemented")
		mock.MockDecision(func(ctx context.Context, flow *workflow.Flow, creditVerified bool, employmentFailures int, identityVerified bool) (approved bool, err error) {
			return true, nil
		})
		approved, err := mock.Decision(ctx, nil, true, 0, true)
		assert.Expect(
			approved, true,
			err, nil,
		)
	})

	t.Run("request_more_info", func(t *testing.T) { // MARKER: RequestMoreInfo
		assert := testarossa.For(t)

		_, err := mock.RequestMoreInfo(ctx, nil, 0)
		assert.Contains(err.Error(), "not implemented")
		mock.MockRequestMoreInfo(func(ctx context.Context, flow *workflow.Flow, reviewAttempts int) (reviewAttemptsOut int, err error) {
			return reviewAttempts + 1, nil
		})
		reviewAttemptsOut, err := mock.RequestMoreInfo(ctx, nil, 0)
		assert.Expect(
			reviewAttemptsOut, 1,
			err, nil,
		)
	})

	t.Run("review_credit", func(t *testing.T) { // MARKER: ReviewCredit
		assert := testarossa.For(t)

		_, err := mock.ReviewCredit(ctx, nil, 580, false, 0, "")
		assert.Contains(err.Error(), "not implemented")
		mock.MockReviewCredit(func(ctx context.Context, flow *workflow.Flow, creditScore int, creditVerified bool, reviewAttempts int, faultInjection string) (creditVerifiedOut bool, err error) {
			return true, nil
		})
		creditVerifiedOut, err := mock.ReviewCredit(ctx, nil, 580, false, 0, "")
		assert.Expect(
			creditVerifiedOut, true,
			err, nil,
		)
	})

	t.Run("credit_approval", func(t *testing.T) { // MARKER: CreditApproval
		assert := testarossa.For(t)

		// Before mocking, graph endpoint returns "not implemented"
		_, err := mock.CreditApproval(ctx)
		assert.Contains(err.Error(), "not implemented")

		// Mock the workflow behavior
		mock.MockCreditApproval(func(ctx context.Context, flow *workflow.Flow, applicant creditflowapi.Applicant, faultInjection string) (approved bool, creditVerified bool, employmentFailures int, identityVerified bool, err error) {
			return true, true, 0, true, nil
		})
		// Graph endpoint should now return a valid graph
		graph, err := mock.CreditApproval(ctx)
		if assert.NoError(err) {
			assert.NotNil(graph)
		}

		// Clear the mock
		mock.MockCreditApproval(nil)
		_, err = mock.CreditApproval(ctx)
		assert.Contains(err.Error(), "not implemented")
	})

	t.Run("identity_verification", func(t *testing.T) { // MARKER: IdentityVerification
		assert := testarossa.For(t)

		// Before mocking, graph endpoint returns "not implemented"
		_, err := mock.IdentityVerification(ctx)
		assert.Contains(err.Error(), "not implemented")

		// Mock the workflow behavior
		mock.MockIdentityVerification(func(ctx context.Context, flow *workflow.Flow, applicantName string, ssn string, address string, phone string) (identityVerified bool, err error) {
			return true, nil
		})
		// Graph endpoint should now return a valid graph
		graph, err := mock.IdentityVerification(ctx)
		if assert.NoError(err) {
			assert.NotNil(graph)
		}

		// Clear the mock
		mock.MockIdentityVerification(nil)
		_, err = mock.IdentityVerification(ctx)
		assert.Contains(err.Error(), "not implemented")
	})

	t.Run("demo", func(t *testing.T) { // MARKER: Demo
		assert := testarossa.For(t)

		w := httpx.NewResponseRecorder()
		r := httpx.MustNewRequest("GET", "/", nil)

		err := mock.Demo(w, r)
		assert.Contains(err.Error(), "not implemented")
		mock.MockDemo(func(w http.ResponseWriter, r *http.Request) (err error) {
			w.WriteHeader(http.StatusOK)
			return nil
		})
		err = mock.Demo(w, r)
		assert.NoError(err)
	})
}

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

		creditVerified, err := exec.VerifyCredit(ctx, 750, "")
		if assert.NoError(err) {
			assert.Expect(creditVerified, true)
		}
	})

	t.Run("bad_score", func(t *testing.T) {
		assert := testarossa.For(t)

		creditVerified, err := exec.VerifyCredit(ctx, 540, "")
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
			employmentFailures, err := exec.WithOutputFlow(&outFlow).VerifyEmployment(ctx, applicantName, employerName)
			if assert.NoError(err) {
				assert.Expect(employmentFailures, 0)
			}
		})
	*/

	t.Run("employed", func(t *testing.T) {
		assert := testarossa.For(t)

		employmentFailures, err := exec.VerifyEmployment(ctx, "Alice", "Acme Corp")
		if assert.NoError(err) {
			assert.Expect(employmentFailures, 0)
		}
	})

	t.Run("empty_name", func(t *testing.T) {
		assert := testarossa.For(t)

		employmentFailures, err := exec.VerifyEmployment(ctx, "Alice", "")
		if assert.NoError(err) {
			assert.Expect(employmentFailures, 1)
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
			ssnVerified, err := exec.WithOutputFlow(&outFlow).VerifySSN(ctx, ssn, faultInjection)
			if assert.NoError(err) {
				assert.Expect(ssnVerified, true)
			}
		})
	*/

	t.Run("valid", func(t *testing.T) {
		assert := testarossa.For(t)
		ssnVerified, err := exec.VerifySSN(ctx, "123-45-6789", "")
		if assert.NoError(err) {
			assert.Expect(ssnVerified, true)
		}
	})

	t.Run("empty_ssn", func(t *testing.T) {
		assert := testarossa.For(t)
		ssnVerified, err := exec.VerifySSN(ctx, "", "")
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
			phoneVerified, err := exec.WithOutputFlow(&outFlow).VerifyPhoneNumber(ctx, phone, faultInjection)
			if assert.NoError(err) {
				assert.Expect(phoneVerified, true)
			}
		})
	*/

	t.Run("valid", func(t *testing.T) {
		assert := testarossa.For(t)
		phoneVerified, err := exec.VerifyPhoneNumber(ctx, "555-123-4567", "")
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

		creditVerifiedOut, err := exec.ReviewCredit(ctx, 580, false, 0, "")
		if assert.NoError(err) {
			assert.Expect(creditVerifiedOut, true)
		}
	})

	t.Run("too_low_rejected", func(t *testing.T) {
		assert := testarossa.For(t)

		creditVerifiedOut, err := exec.ReviewCredit(ctx, 400, false, 0, "")
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
			approved, err := exec.WithOutputFlow(&outFlow).Decision(ctx, creditVerified, employmentFailures, identityVerified)
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

			approved, creditVerified, employmentFailures, identityVerified, status, err := exec.CreditApproval(ctx, creditflowapi.Applicant{...}, "")
			assert.Expect(
				err, nil,
				status, foremanapi.StatusCompleted,
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
		}, "")
		if assert.NoError(err) {
			assert.Expect(
				status, foremanapi.StatusCompleted,
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
		}, "")
		if assert.NoError(err) {
			assert.Expect(
				status, foremanapi.StatusCompleted,
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
		}, "")
		if assert.NoError(err) {
			assert.Expect(
				status, foremanapi.StatusCompleted,
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
		}, "")
		if assert.NoError(err) {
			assert.Expect(
				status, foremanapi.StatusCompleted,
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
		}, "")
		if assert.NoError(err) {
			assert.Expect(
				status, foremanapi.StatusCompleted,
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
		}, "")
		if assert.NoError(err) {
			assert.Expect(
				status, foremanapi.StatusCompleted,
				approved, false,
				identityVerified, false,
			)
		}
	})

	t.Run("time_budget_exceeded", func(t *testing.T) {
		assert := testarossa.For(t)

		// VerifyPhoneNumber has a 1s time budget; "Delay" fault injection triggers a 1.5s sleep, causing failure
		_, _, _, _, status, err := exec.CreditApproval(ctx, creditflowapi.Applicant{
			ApplicantName: "Frank",
			SSN:           "123-45-6789",
			Address:       "123 Main St",
			Phone:         "555-123-4567",
			Employers:     []string{"Acme Corp"},
			CreditScore:   750,
		}, "Delay")
		if assert.NoError(err) {
			assert.Expect(status, foremanapi.StatusFailed)
		}
	})

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
		})
		assert.NoError(err)

		// Set a breakpoint on the ReviewCredit task (runs after fan-in, before Decision)
		err = foremanClient.BreakBefore(ctx, flowKey, creditflowapi.ReviewCredit.URL(), true)
		assert.NoError(err)

		// Start the flow
		err = foremanClient.Start(ctx, flowKey)
		assert.NoError(err)

		// Wait for the flow to hit the breakpoint (interrupted status)
		status, state, err := foremanClient.Await(ctx, flowKey)
		assert.Expect(
			err, nil,
			status, foremanapi.StatusInterrupted,
			state["creditVerified"], true, // The flow should be paused before Decision, so creditVerified should be set from prior steps
		)

		// Resume the flow past the breakpoint
		err = foremanClient.Resume(ctx, flowKey, nil)
		assert.NoError(err)

		// Wait for completion
		status, state, err = foremanClient.Await(ctx, flowKey)
		assert.Expect(
			err, nil,
			status, foremanapi.StatusCompleted,
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
		})
		assert.NoError(err)
		err = foremanClient.BreakBefore(ctx, flowKey, creditflowapi.IdentityDecision.URL(), true)
		assert.NoError(err)

		// Start the parent flow
		err = foremanClient.Start(ctx, flowKey)
		assert.NoError(err)

		// The subgraph breakpoint propagates up - the parent flow becomes interrupted
		status, _, err := foremanClient.Await(ctx, flowKey)
		if !assert.Expect(err, nil, status, foremanapi.StatusInterrupted) {
			return
		}

		// Cancel the parent flow - cancellation cascades to the subgraph
		err = foremanClient.Cancel(ctx, flowKey)
		assert.NoError(err)

		// Verify the parent is cancelled
		parentStatus, _, err := foremanClient.Await(ctx, flowKey)
		assert.Expect(
			err, nil,
			parentStatus, foremanapi.StatusCancelled,
		)
	})

	t.Run("subgraph_interrupt_resume", func(t *testing.T) {
		assert := testarossa.For(t)

		foremanClient := foremanapi.NewClient(tester)

		// Create the parent flow with MissingSSN fault injection - VerifySSN will interrupt to request it
		flowKey, err := foremanClient.Create(ctx, creditflowapi.CreditApproval.URL(), creditflowapi.CreditApprovalIn{
			Applicant: creditflowapi.Applicant{
				ApplicantName: "Ivy",
				SSN:           "123-45-6789",
				Address:       "123 Main St",
				Phone:         "555-123-4567",
				Employers:     []string{"Acme Corp"},
				CreditScore:   750,
			},
			FaultInjection: "MissingSSN",
		})
		assert.NoError(err)

		// Start the flow
		err = foremanClient.Start(ctx, flowKey)
		assert.NoError(err)

		// VerifySSN interrupts because of MissingSSN fault injection - propagates up to the parent
		status, state, err := foremanClient.Await(ctx, flowKey)
		if !assert.Expect(err, nil, status, foremanapi.StatusInterrupted) {
			return
		}
		// The interrupt payload should be visible from the parent's State
		assert.Expect(state["request"], "ssn")

		// Resume with the SSN provided and clear the fault injection
		err = foremanClient.Resume(ctx, flowKey, map[string]any{"ssn": "123-45-6789", "faultInjection": ""})
		assert.NoError(err)

		// Wait for completion
		status, state, err = foremanClient.Await(ctx, flowKey)
		assert.Expect(
			err, nil,
			status, foremanapi.StatusCompleted,
			state["approved"], true,
		)
	})

	t.Run("retry", func(t *testing.T) {
		assert := testarossa.For(t)

		// "Retry" fault injection triggers VerifyCredit to retry 3 times, incrementing retryCount each time
		approved, creditVerified, _, _, status, err := exec.CreditApproval(ctx, creditflowapi.Applicant{
			ApplicantName: "Kate",
			SSN:           "123-45-6789",
			Address:       "123 Main St",
			Phone:         "555-123-4567",
			Employers:     []string{"Acme Corp"},
			CreditScore:   750,
		}, "Retry")
		if assert.NoError(err) {
			assert.Expect(
				status, foremanapi.StatusCompleted,
				approved, true,
				creditVerified, true,
			)
		}
	})

	t.Run("sleep", func(t *testing.T) {
		assert := testarossa.For(t)

		// "Sleep" fault injection triggers ReviewCredit to sleep for 200ms before approving.
		// ReviewCredit is after fan-in, so the sleep delays the Decision step.
		t0 := time.Now()
		approved, _, _, _, status, err := exec.CreditApproval(ctx, creditflowapi.Applicant{
			ApplicantName: "Leo",
			SSN:           "123-45-6789",
			Address:       "123 Main St",
			Phone:         "555-123-4567",
			Employers:     []string{"Acme Corp"},
			CreditScore:   750,
		}, "Sleep")
		elapsed := time.Since(t0)
		if assert.NoError(err) {
			assert.Expect(
				status, foremanapi.StatusCompleted,
				approved, true,
				elapsed >= 200*time.Millisecond, true,
			)
		}
	})

	t.Run("bad_goto", func(t *testing.T) {
		assert := testarossa.For(t)

		// "BadGoto" fault injection triggers ReviewCredit to call flow.Goto with a target
		// that has no WithGoto transition, which should fail the flow.
		_, _, _, _, status, err := exec.CreditApproval(ctx, creditflowapi.Applicant{
			ApplicantName: "Max",
			SSN:           "123-45-6789",
			Address:       "123 Main St",
			Phone:         "555-123-4567",
			Employers:     []string{"Acme Corp"},
			CreditScore:   750,
		}, "BadGoto")
		if assert.NoError(err) {
			assert.Expect(status, foremanapi.StatusFailed)
		}
	})

	t.Run("error_transition", func(t *testing.T) {
		assert := testarossa.For(t)

		// "Error" fault injection causes VerifyCredit to return an error.
		// The error transition routes to HandleCreditError which sets creditVerified=false.
		// The flow completes (not fails) with approved=false since credit was not verified.
		approved, creditVerified, _, _, status, err := exec.CreditApproval(ctx, creditflowapi.Applicant{
			ApplicantName: "Olivia",
			SSN:           "123-45-6789",
			Address:       "123 Main St",
			Phone:         "555-123-4567",
			Employers:     []string{"Acme Corp"},
			CreditScore:   750,
		}, "Error")
		if assert.NoError(err) {
			assert.Expect(
				status, foremanapi.StatusCompleted,
				approved, false,
				creditVerified, false,
			)
		}
	})

}

func TestCreditFlow_DynamicSubgraph(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	svc := NewService()
	tester := connector.New("tester.client")
	foremanClient := foremanapi.NewClient(tester)

	app := application.New()
	app.Add(svc, foreman.NewService(), tester)
	app.RunInTest(t)

	t.Run("verify_credit_with_subgraph", func(t *testing.T) {
		assert := testarossa.For(t)

		// "Subgraph" fault injection triggers VerifyCredit to dynamically run the
		// IdentityVerification workflow as a child subgraph. On re-entry after the
		// child completes, VerifyCredit reads identityVerified from state and
		// combines it with the credit score check.
		//
		// We use CreateTask to run VerifyCredit as a standalone task with a workflow
		// runner, so it doesn't interfere with the static IdentityVerification
		// subgraph in the CreditApproval workflow.
		initialState := map[string]any{
			"creditScore":    750,
			"faultInjection": "Subgraph",
			"applicantName":  "Nina",
			"ssn":            "123-45-6789",
			"address":        "123 Main St",
			"phone":          "555-123-4567",
		}
		flowKey, err := foremanClient.CreateTask(ctx, creditflowapi.VerifyCredit.URL(), initialState)
		if !assert.NoError(err) {
			return
		}
		err = foremanClient.Start(ctx, flowKey)
		if !assert.NoError(err) {
			return
		}
		status, state, err := foremanClient.Await(ctx, flowKey)
		if assert.NoError(err) {
			assert.Expect(
				status, foremanapi.StatusCompleted,
				state["creditVerified"], true,
				state["identityVerified"], true,
			)
		}
	})
}

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
		func(ctx context.Context, flowKey string, status string, snapshot map[string]any) (err error) {
			notifyCh <- notification{flowKey: flowKey, status: status, snapshot: snapshot}
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
		})
		assert.NoError(err)
		err = foremanClient.StartNotify(ctx, flowKey, tester.Hostname())
		assert.NoError(err)

		n := waitForNotification(assert, flowKey, foremanapi.StatusCompleted)
		assert.Expect(n.status, foremanapi.StatusCompleted)
		assert.NotNil(n.snapshot)
	})

	t.Run("interrupted", func(t *testing.T) {
		assert := testarossa.For(t)

		// MissingSSN fault causes VerifySSN to interrupt the flow
		flowKey, err := foremanClient.Create(ctx, creditflowapi.CreditApproval.URL(), creditflowapi.CreditApprovalIn{
			Applicant:      goodApplicant,
			FaultInjection: "MissingSSN",
		})
		assert.NoError(err)
		err = foremanClient.StartNotify(ctx, flowKey, tester.Hostname())
		assert.NoError(err)

		n := waitForNotification(assert, flowKey, foremanapi.StatusInterrupted)
		assert.Expect(n.status, foremanapi.StatusInterrupted)
	})

	t.Run("cancelled", func(t *testing.T) {
		assert := testarossa.For(t)

		// Use a breakpoint to pause the flow, then cancel it
		flowKey, err := foremanClient.Create(ctx, creditflowapi.CreditApproval.URL(), creditflowapi.CreditApprovalIn{
			Applicant: goodApplicant,
		})
		assert.NoError(err)
		err = foremanClient.BreakBefore(ctx, flowKey, creditflowapi.ReviewCredit.URL(), true)
		assert.NoError(err)

		err = foremanClient.StartNotify(ctx, flowKey, tester.Hostname())
		assert.NoError(err)

		// Wait for breakpoint interrupt notification
		waitForNotification(assert, flowKey, foremanapi.StatusInterrupted)

		// Cancel the flow
		err = foremanClient.Cancel(ctx, flowKey)
		assert.NoError(err)

		n := waitForNotification(assert, flowKey, foremanapi.StatusCancelled)
		assert.Expect(n.status, foremanapi.StatusCancelled)
		assert.NotNil(n.snapshot)
	})

	t.Run("failed", func(t *testing.T) {
		assert := testarossa.For(t)

		// Delay fault causes VerifyPhoneNumber to exceed its time budget, failing the flow
		flowKey, err := foremanClient.Create(ctx, creditflowapi.CreditApproval.URL(), creditflowapi.CreditApprovalIn{
			Applicant:      goodApplicant,
			FaultInjection: "Delay",
		})
		assert.NoError(err)
		err = foremanClient.StartNotify(ctx, flowKey, tester.Hostname())
		assert.NoError(err)

		n := waitForNotification(assert, flowKey, foremanapi.StatusFailed)
		assert.Expect(n.status, foremanapi.StatusFailed)
		assert.NotNil(n.snapshot)
	})
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
				assert.Contains(body, "submit-credit-application")
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
		_, _, err := foremanClient.Run(ctx, creditflowapi.CreditApproval.URL(), applicant)
		if err != nil {
			b.Fatal(err)
		}
	}

	/*
		MySQL: approx 15 workflows/sec or 165 tasks/sec

		goos: darwin
		goarch: arm64
		pkg: github.com/microbus-io/fabric/examples/creditflow
		cpu: Apple M1 Pro
		BenchmarkCreditFlow_CreditApproval-10    	      16	  67107253 ns/op	 1392209 B/op	   18969 allocs/op
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
			_, _, err := foremanClient.Run(ctx, creditflowapi.CreditApproval.URL(), applicant)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	/*
		MySQL: approx 333 workflows/sec or 3660 tasks/sec

		goos: darwin
		goarch: arm64
		pkg: github.com/microbus-io/fabric/examples/creditflow
		cpu: Apple M1 Pro
		BenchmarkCreditFlow_CreditApprovalParallel-10    	     37	  29589952 ns/op	 1208531 B/op	   17993 allocs/op
	*/
}

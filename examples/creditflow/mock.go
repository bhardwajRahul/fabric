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
	"encoding/json"
	"net/http"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/fabric/utils"
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/examples/creditflow/creditflowapi"
)

var (
	_ http.Request
	_ json.Encoder
	_ errors.TracedError
	_ httpx.BodyReader
	_ = utils.RandomIdentifier
	_ *workflow.Flow
	_ creditflowapi.Client
)

// Mock is a mockable version of the microservice, allowing functions, event sinks and web handlers to be mocked.
type Mock struct {
	*Intermediate
	mockSubmitCreditApplication   func(ctx context.Context, flow *workflow.Flow, applicant creditflowapi.Applicant) (applicantName string, ssn string, address string, phone string, employers []string, creditScore int, err error) // MARKER: SubmitCreditApplication
	mockVerifyCredit              func(ctx context.Context, flow *workflow.Flow, creditScore int, faultInjection string) (creditVerified bool, err error)                                                                            // MARKER: VerifyCredit
	mockVerifyEmployment          func(ctx context.Context, flow *workflow.Flow, applicantName string, employerName string) (employmentFailures int, err error)                                                                      // MARKER: VerifyEmployment
	mockInitIdentityVerification  func(ctx context.Context, flow *workflow.Flow, applicantName string, ssn string, address string, phone string) (err error)                                                                         // MARKER: InitIdentityVerification
	mockVerifySSN                 func(ctx context.Context, flow *workflow.Flow, ssn string, faultInjection string) (ssnVerified bool, err error)                                                                                    // MARKER: VerifySSN
	mockVerifyAddress             func(ctx context.Context, flow *workflow.Flow, address string) (addressVerified bool, err error)                                                                                                   // MARKER: VerifyAddress
	mockVerifyPhoneNumber         func(ctx context.Context, flow *workflow.Flow, phone string, faultInjection string) (phoneVerified bool, err error)                                                                                // MARKER: VerifyPhoneNumber
	mockIdentityDecision          func(ctx context.Context, flow *workflow.Flow, ssnVerified bool, addressVerified bool, phoneVerified bool) (identityVerified bool, err error)                                                      // MARKER: IdentityDecision
	mockIdentityVerificationGraph func(ctx context.Context) (graph *workflow.Graph, err error)                                                                                                                                       // MARKER: IdentityVerification
	unsubMockIdentityVerification func() error                                                                                                                                                                                       // MARKER: IdentityVerification
	mockRequestMoreInfo           func(ctx context.Context, flow *workflow.Flow, reviewAttempts int) (reviewAttemptsOut int, err error)                                                                                              // MARKER: RequestMoreInfo
	mockReviewCredit              func(ctx context.Context, flow *workflow.Flow, creditScore int, creditVerified bool, reviewAttempts int, faultInjection string) (creditVerifiedOut bool, err error)                                // MARKER: ReviewCredit
	mockHandleCreditError         func(ctx context.Context, flow *workflow.Flow, onErr *errors.TracedError) (creditVerified bool, err error)                                                                                         // MARKER: HandleCreditError
	mockDecision                  func(ctx context.Context, flow *workflow.Flow, creditVerified bool, employmentFailures int, identityVerified bool) (approved bool, err error)                                                      // MARKER: Decision
	mockCreditApprovalGraph       func(ctx context.Context) (graph *workflow.Graph, err error)                                                                                                                                       // MARKER: CreditApproval
	unsubMockCreditApproval       func() error                                                                                                                                                                                       // MARKER: CreditApproval
	mockDemo                      func(w http.ResponseWriter, r *http.Request) (err error)                                                                                                                                           // MARKER: Demo
}

// NewMock creates a new mockable version of the microservice.
func NewMock() *Mock {
	svc := &Mock{}
	svc.Intermediate = NewIntermediate(svc)
	svc.SetVersion(7357) // Stands for TEST
	return svc
}

// OnStartup is called when the microservice is started up.
func (svc *Mock) OnStartup(ctx context.Context) (err error) {
	if svc.Deployment() != connector.LOCAL && svc.Deployment() != connector.TESTING {
		return errors.New("mocking disallowed in %s deployment", svc.Deployment())
	}
	return nil
}

// OnShutdown is called when the microservice is shut down.
func (svc *Mock) OnShutdown(ctx context.Context) (err error) {
	return nil
}

// MockSubmitCreditApplication sets up a mock handler for SubmitCreditApplication.
func (svc *Mock) MockSubmitCreditApplication(handler func(ctx context.Context, flow *workflow.Flow, applicant creditflowapi.Applicant) (applicantName string, ssn string, address string, phone string, employers []string, creditScore int, err error)) *Mock { // MARKER: SubmitCreditApplication
	svc.mockSubmitCreditApplication = handler
	return svc
}

// SubmitCreditApplication executes the mock handler.
func (svc *Mock) SubmitCreditApplication(ctx context.Context, flow *workflow.Flow, applicant creditflowapi.Applicant) (applicantName string, ssn string, address string, phone string, employers []string, creditScore int, err error) { // MARKER: SubmitCreditApplication
	if svc.mockSubmitCreditApplication == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	applicantName, ssn, address, phone, employers, creditScore, err = svc.mockSubmitCreditApplication(ctx, flow, applicant)
	return applicantName, ssn, address, phone, employers, creditScore, errors.Trace(err)
}

// MockVerifyCredit sets up a mock handler for VerifyCredit.
func (svc *Mock) MockVerifyCredit(handler func(ctx context.Context, flow *workflow.Flow, creditScore int, faultInjection string) (creditVerified bool, err error)) *Mock { // MARKER: VerifyCredit
	svc.mockVerifyCredit = handler
	return svc
}

// VerifyCredit executes the mock handler.
func (svc *Mock) VerifyCredit(ctx context.Context, flow *workflow.Flow, creditScore int, faultInjection string) (creditVerified bool, err error) { // MARKER: VerifyCredit
	if svc.mockVerifyCredit == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	creditVerified, err = svc.mockVerifyCredit(ctx, flow, creditScore, faultInjection)
	return creditVerified, errors.Trace(err)
}

// MockVerifyEmployment sets up a mock handler for VerifyEmployment.
func (svc *Mock) MockVerifyEmployment(handler func(ctx context.Context, flow *workflow.Flow, applicantName string, employerName string) (employmentFailures int, err error)) *Mock { // MARKER: VerifyEmployment
	svc.mockVerifyEmployment = handler
	return svc
}

// VerifyEmployment executes the mock handler.
func (svc *Mock) VerifyEmployment(ctx context.Context, flow *workflow.Flow, applicantName string, employerName string) (employmentFailures int, err error) { // MARKER: VerifyEmployment
	if svc.mockVerifyEmployment == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	employmentFailures, err = svc.mockVerifyEmployment(ctx, flow, applicantName, employerName)
	return employmentFailures, errors.Trace(err)
}

// MockInitIdentityVerification sets up a mock handler for InitIdentityVerification.
func (svc *Mock) MockInitIdentityVerification(handler func(ctx context.Context, flow *workflow.Flow, applicantName string, ssn string, address string, phone string) (err error)) *Mock { // MARKER: InitIdentityVerification
	svc.mockInitIdentityVerification = handler
	return svc
}

// InitIdentityVerification executes the mock handler.
func (svc *Mock) InitIdentityVerification(ctx context.Context, flow *workflow.Flow, applicantName string, ssn string, address string, phone string) (err error) { // MARKER: InitIdentityVerification
	if svc.mockInitIdentityVerification == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	err = svc.mockInitIdentityVerification(ctx, flow, applicantName, ssn, address, phone)
	return errors.Trace(err)
}

// MockVerifySSN sets up a mock handler for VerifySSN.
func (svc *Mock) MockVerifySSN(handler func(ctx context.Context, flow *workflow.Flow, ssn string, faultInjection string) (ssnVerified bool, err error)) *Mock { // MARKER: VerifySSN
	svc.mockVerifySSN = handler
	return svc
}

// VerifySSN executes the mock handler.
func (svc *Mock) VerifySSN(ctx context.Context, flow *workflow.Flow, ssn string, faultInjection string) (ssnVerified bool, err error) { // MARKER: VerifySSN
	if svc.mockVerifySSN == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	ssnVerified, err = svc.mockVerifySSN(ctx, flow, ssn, faultInjection)
	return ssnVerified, errors.Trace(err)
}

// MockVerifyAddress sets up a mock handler for VerifyAddress.
func (svc *Mock) MockVerifyAddress(handler func(ctx context.Context, flow *workflow.Flow, address string) (addressVerified bool, err error)) *Mock { // MARKER: VerifyAddress
	svc.mockVerifyAddress = handler
	return svc
}

// VerifyAddress executes the mock handler.
func (svc *Mock) VerifyAddress(ctx context.Context, flow *workflow.Flow, address string) (addressVerified bool, err error) { // MARKER: VerifyAddress
	if svc.mockVerifyAddress == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	addressVerified, err = svc.mockVerifyAddress(ctx, flow, address)
	return addressVerified, errors.Trace(err)
}

// MockVerifyPhoneNumber sets up a mock handler for VerifyPhoneNumber.
func (svc *Mock) MockVerifyPhoneNumber(handler func(ctx context.Context, flow *workflow.Flow, phone string, faultInjection string) (phoneVerified bool, err error)) *Mock { // MARKER: VerifyPhoneNumber
	svc.mockVerifyPhoneNumber = handler
	return svc
}

// VerifyPhoneNumber executes the mock handler.
func (svc *Mock) VerifyPhoneNumber(ctx context.Context, flow *workflow.Flow, phone string, faultInjection string) (phoneVerified bool, err error) { // MARKER: VerifyPhoneNumber
	if svc.mockVerifyPhoneNumber == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	phoneVerified, err = svc.mockVerifyPhoneNumber(ctx, flow, phone, faultInjection)
	return phoneVerified, errors.Trace(err)
}

// MockIdentityDecision sets up a mock handler for IdentityDecision.
func (svc *Mock) MockIdentityDecision(handler func(ctx context.Context, flow *workflow.Flow, ssnVerified bool, addressVerified bool, phoneVerified bool) (identityVerified bool, err error)) *Mock { // MARKER: IdentityDecision
	svc.mockIdentityDecision = handler
	return svc
}

// IdentityDecision executes the mock handler.
func (svc *Mock) IdentityDecision(ctx context.Context, flow *workflow.Flow, ssnVerified bool, addressVerified bool, phoneVerified bool) (identityVerified bool, err error) { // MARKER: IdentityDecision
	if svc.mockIdentityDecision == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	identityVerified, err = svc.mockIdentityDecision(ctx, flow, ssnVerified, addressVerified, phoneVerified)
	return identityVerified, errors.Trace(err)
}

// MockIdentityVerification sets up a mock handler for the IdentityVerification workflow.
// The handler receives typed inputs from the workflow's state and returns typed outputs.
// A nil handler clears the mock.
func (svc *Mock) MockIdentityVerification(handler func(ctx context.Context, flow *workflow.Flow, applicantName string, ssn string, address string, phone string) (identityVerified bool, err error)) *Mock { // MARKER: IdentityVerification
	if svc.unsubMockIdentityVerification != nil {
		svc.unsubMockIdentityVerification()
		svc.unsubMockIdentityVerification = nil
	}
	if handler == nil {
		svc.mockIdentityVerificationGraph = nil
		return svc
	}
	mockRoute := ":428/mock-wf-" + utils.RandomIdentifier(8)
	mockTaskURL := httpx.JoinHostAndPath(svc.Hostname(), mockRoute)
	svc.mockIdentityVerificationGraph = func(ctx context.Context) (graph *workflow.Graph, err error) {
		g := workflow.NewGraph(creditflowapi.IdentityVerification.URL())
		g.AddTransition(mockTaskURL, workflow.END)
		return g, nil
	}
	unsub, _ := svc.Subscribe("POST", mockRoute, func(w http.ResponseWriter, r *http.Request) error {
		var f workflow.Flow
		err := json.NewDecoder(r.Body).Decode(&f)
		if err != nil {
			return errors.Trace(err)
		}
		snap := f.Snapshot()
		var in creditflowapi.IdentityVerificationIn
		f.ParseState(&in)
		var out creditflowapi.IdentityVerificationOut
		out.IdentityVerified, err = handler(r.Context(), &f, in.ApplicantName, in.SSN, in.Address, in.Phone)
		if err != nil {
			return err // No trace
		}
		f.SetChanges(out, snap)
		w.Header().Set("Content-Type", "application/json")
		return json.NewEncoder(w).Encode(&f)
	})
	svc.unsubMockIdentityVerification = unsub
	return svc
}

// IdentityVerification returns the workflow graph, or a mocked graph if MockIdentityVerification was called.
func (svc *Mock) IdentityVerification(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: IdentityVerification
	if svc.mockIdentityVerificationGraph == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	graph, err = svc.mockIdentityVerificationGraph(ctx)
	return graph, errors.Trace(err)
}

// MockRequestMoreInfo sets up a mock handler for RequestMoreInfo.
func (svc *Mock) MockRequestMoreInfo(handler func(ctx context.Context, flow *workflow.Flow, reviewAttempts int) (reviewAttemptsOut int, err error)) *Mock { // MARKER: RequestMoreInfo
	svc.mockRequestMoreInfo = handler
	return svc
}

// RequestMoreInfo executes the mock handler.
func (svc *Mock) RequestMoreInfo(ctx context.Context, flow *workflow.Flow, reviewAttempts int) (reviewAttemptsOut int, err error) { // MARKER: RequestMoreInfo
	if svc.mockRequestMoreInfo == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	reviewAttemptsOut, err = svc.mockRequestMoreInfo(ctx, flow, reviewAttempts)
	return reviewAttemptsOut, errors.Trace(err)
}

// MockReviewCredit sets up a mock handler for ReviewCredit.
func (svc *Mock) MockReviewCredit(handler func(ctx context.Context, flow *workflow.Flow, creditScore int, creditVerified bool, reviewAttempts int, faultInjection string) (creditVerifiedOut bool, err error)) *Mock { // MARKER: ReviewCredit
	svc.mockReviewCredit = handler
	return svc
}

// ReviewCredit executes the mock handler.
func (svc *Mock) ReviewCredit(ctx context.Context, flow *workflow.Flow, creditScore int, creditVerified bool, reviewAttempts int, faultInjection string) (creditVerifiedOut bool, err error) { // MARKER: ReviewCredit
	if svc.mockReviewCredit == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	creditVerifiedOut, err = svc.mockReviewCredit(ctx, flow, creditScore, creditVerified, reviewAttempts, faultInjection)
	return creditVerifiedOut, errors.Trace(err)
}

// MockHandleCreditError sets up a mock handler for HandleCreditError.
func (svc *Mock) MockHandleCreditError(handler func(ctx context.Context, flow *workflow.Flow, onErr *errors.TracedError) (creditVerified bool, err error)) *Mock { // MARKER: HandleCreditError
	svc.mockHandleCreditError = handler
	return svc
}

// HandleCreditError executes the mock handler.
func (svc *Mock) HandleCreditError(ctx context.Context, flow *workflow.Flow, onErr *errors.TracedError) (creditVerified bool, err error) { // MARKER: HandleCreditError
	if svc.mockHandleCreditError == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	creditVerified, err = svc.mockHandleCreditError(ctx, flow, onErr)
	return creditVerified, errors.Trace(err)
}

// MockDecision sets up a mock handler for Decision.
func (svc *Mock) MockDecision(handler func(ctx context.Context, flow *workflow.Flow, creditVerified bool, employmentFailures int, identityVerified bool) (approved bool, err error)) *Mock { // MARKER: Decision
	svc.mockDecision = handler
	return svc
}

// Decision executes the mock handler.
func (svc *Mock) Decision(ctx context.Context, flow *workflow.Flow, creditVerified bool, employmentFailures int, identityVerified bool) (approved bool, err error) { // MARKER: Decision
	if svc.mockDecision == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	approved, err = svc.mockDecision(ctx, flow, creditVerified, employmentFailures, identityVerified)
	return approved, errors.Trace(err)
}

// MockCreditApproval sets up a mock handler for the CreditApproval workflow.
// The handler receives typed inputs from the workflow's state and returns typed outputs.
// A nil handler clears the mock.
func (svc *Mock) MockCreditApproval(handler func(ctx context.Context, flow *workflow.Flow, applicant creditflowapi.Applicant, faultInjection string) (approved bool, creditVerified bool, employmentFailures int, identityVerified bool, err error)) *Mock { // MARKER: CreditApproval
	if svc.unsubMockCreditApproval != nil {
		svc.unsubMockCreditApproval()
		svc.unsubMockCreditApproval = nil
	}
	if handler == nil {
		svc.mockCreditApprovalGraph = nil
		return svc
	}
	mockRoute := ":428/mock-wf-" + utils.RandomIdentifier(8)
	mockTaskURL := httpx.JoinHostAndPath(svc.Hostname(), mockRoute)
	svc.mockCreditApprovalGraph = func(ctx context.Context) (graph *workflow.Graph, err error) {
		g := workflow.NewGraph(creditflowapi.CreditApproval.URL())
		g.AddTransition(mockTaskURL, workflow.END)
		return g, nil
	}
	unsub, _ := svc.Subscribe("POST", mockRoute, func(w http.ResponseWriter, r *http.Request) error {
		var f workflow.Flow
		err := json.NewDecoder(r.Body).Decode(&f)
		if err != nil {
			return errors.Trace(err)
		}
		snap := f.Snapshot()
		var in creditflowapi.CreditApprovalIn
		f.ParseState(&in)
		var out creditflowapi.CreditApprovalOut
		out.Approved, out.CreditVerified, out.EmploymentFailures, out.IdentityVerified, err = handler(r.Context(), &f, in.Applicant, in.FaultInjection)
		if err != nil {
			return err // No trace
		}
		f.SetChanges(out, snap)
		w.Header().Set("Content-Type", "application/json")
		return json.NewEncoder(w).Encode(&f)
	})
	svc.unsubMockCreditApproval = unsub
	return svc
}

// CreditApproval returns the workflow graph, or a mocked graph if MockCreditApproval was called.
func (svc *Mock) CreditApproval(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: CreditApproval
	if svc.mockCreditApprovalGraph == nil {
		err = errors.New("mock not implemented", http.StatusNotImplemented)
		return
	}
	graph, err = svc.mockCreditApprovalGraph(ctx)
	return graph, errors.Trace(err)
}

// MockDemo sets up a mock handler for Demo.
func (svc *Mock) MockDemo(handler func(w http.ResponseWriter, r *http.Request) (err error)) *Mock { // MARKER: Demo
	svc.mockDemo = handler
	return svc
}

// Demo executes the mock handler.
func (svc *Mock) Demo(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Demo
	if svc.mockDemo == nil {
		return errors.New("mock not implemented", http.StatusNotImplemented)
	}
	err = svc.mockDemo(w, r)
	return errors.Trace(err)
}

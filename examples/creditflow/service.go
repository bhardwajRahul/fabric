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
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/pub"
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/coreservices/foreman/foremanapi"
	"github.com/microbus-io/fabric/examples/creditflow/creditflowapi"
)

var (
	_ context.Context
	_ http.Request
	_ errors.TracedError
	_ *workflow.Flow
	_ creditflowapi.Client
	_ foremanapi.Client
	_ pub.Option
	_ fmt.Stringer
	_ io.Reader
	_ json.Encoder
	_ strconv.NumError
)

/*
Service implements the creditflow example microservice.
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
SubmitCreditApplication receives a credit application and unpacks the applicant into individual state fields.
*/
func (svc *Service) SubmitCreditApplication(ctx context.Context, flow *workflow.Flow, applicant creditflowapi.Applicant) (applicantName string, ssn string, address string, phone string, employers []string, creditScore int, err error) { // MARKER: SubmitCreditApplication
	return applicant.ApplicantName, applicant.SSN, applicant.Address, applicant.Phone, applicant.Employers, applicant.CreditScore, nil
}

/*
VerifyCredit checks the applicant's credit score.
*/
func (svc *Service) VerifyCredit(ctx context.Context, flow *workflow.Flow, creditScore int, faultInjection string) (creditVerified bool, err error) { // MARKER: VerifyCredit
	// Fault: return an error to test error transitions
	if strings.Contains(faultInjection, "Error") {
		return false, errors.New("credit bureau unavailable", http.StatusServiceUnavailable)
	}
	// Fault: retry up to 3 times
	if strings.Contains(faultInjection, "Retry") {
		if flow.Retry(3, 0, 0, 0) {
			return false, nil
		}
		return true, nil
	}
	// Fault: dynamic subgraph - run IdentityVerification as a child workflow.
	// On re-entry after the child completes, the child's output (identityVerified)
	// is in state. This tests the flow.Subgraph re-run model.
	if strings.Contains(faultInjection, "Subgraph") {
		if !flow.GetBool("subgraphPending") {
			// First run: signal subgraph and return
			flow.SetBool("subgraphPending", true)
			flow.Subgraph(creditflowapi.IdentityVerification.URL(), map[string]any{
				"applicantName": flow.GetString("applicantName"),
				"ssn":           flow.GetString("ssn"),
				"address":       flow.GetString("address"),
				"phone":         flow.GetString("phone"),
			})
			return false, nil
		}
		// Re-entry: child completed, identityVerified is in state
		creditVerified = creditScore >= 550 && flow.GetBool("identityVerified")
		return creditVerified, nil
	}
	creditVerified = creditScore >= 550
	return creditVerified, nil
}

/*
HandleCreditError handles a credit verification error by setting creditVerified to false.
The error is received via the onErr state field from an error transition.
*/
func (svc *Service) HandleCreditError(ctx context.Context, flow *workflow.Flow, onErr *errors.TracedError) (creditVerified bool, err error) { // MARKER: HandleCreditError
	svc.LogWarn(ctx, "Credit verification failed, defaulting to not verified", "error", onErr)
	return false, nil
}

/*
VerifyEmployment checks the applicant's employment status.
*/
func (svc *Service) VerifyEmployment(ctx context.Context, flow *workflow.Flow, applicantName string, employerName string) (employmentFailures int, err error) { // MARKER: VerifyEmployment
	if applicantName == "" || employerName == "" {
		return 1, nil
	}
	return 0, nil
}

/*
InitIdentityVerification is the entry point for the identity verification subgraph. It passes through the applicant name.
*/
func (svc *Service) InitIdentityVerification(ctx context.Context, flow *workflow.Flow, applicantName string, ssn string, address string, phone string) (err error) { // MARKER: InitIdentityVerification
	// Pass-through: the input arguments are already set in the workflow state
	return nil
}

/*
VerifySSN checks the applicant's SSN.
*/
func (svc *Service) VerifySSN(ctx context.Context, flow *workflow.Flow, ssn string, faultInjection string) (ssnVerified bool, err error) { // MARKER: VerifySSN
	// Fault: interrupt to request the SSN from the caller
	if strings.Contains(faultInjection, "MissingSSN") {
		flow.Interrupt(map[string]any{"request": "ssn"})
		return false, nil
	}
	// Verify pattern XXX-XX-XXXX and reject if last 4 digits are 0000
	matched, _ := regexp.MatchString(`^\d{3}-\d{2}-\d{4}$`, ssn)
	ssnVerified = matched && !strings.HasSuffix(ssn, "0000")
	return ssnVerified, nil
}

/*
VerifyAddress checks the applicant's address.
*/
func (svc *Service) VerifyAddress(ctx context.Context, flow *workflow.Flow, address string) (addressVerified bool, err error) { // MARKER: VerifyAddress
	addressVerified = address != "" && !strings.Contains(address, "Nowhere")
	return addressVerified, nil
}

/*
VerifyPhoneNumber checks the applicant's phone number.
*/
func (svc *Service) VerifyPhoneNumber(ctx context.Context, flow *workflow.Flow, phone string, faultInjection string) (phoneVerified bool, err error) { // MARKER: VerifyPhoneNumber
	// Fault: sleep to exceed the time budget
	if strings.Contains(faultInjection, "Delay") {
		time.Sleep(1500 * time.Millisecond)
	}
	// Verify pattern XXX-XXX-XXXX or (XXX) XXX-XXXX
	phoneVerified, _ = regexp.MatchString(`^(\d{3}-\d{3}-\d{4}|\(\d{3}\) \d{3}-\d{4})$`, phone)
	return phoneVerified, nil
}

/*
IdentityDecision determines whether the applicant's identity is verified based on SSN, address, and phone checks.
*/
func (svc *Service) IdentityDecision(ctx context.Context, flow *workflow.Flow, ssnVerified bool, addressVerified bool, phoneVerified bool) (identityVerified bool, err error) { // MARKER: IdentityDecision
	identityVerified = ssnVerified && addressVerified && phoneVerified
	return identityVerified, nil
}

/*
IdentityVerification defines the workflow graph for the identity verification process.
*/
func (svc *Service) IdentityVerification(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: IdentityVerification
	initIdentityVerification := creditflowapi.InitIdentityVerification.URL()
	verifySSN := creditflowapi.VerifySSN.URL()
	verifyAddress := creditflowapi.VerifyAddress.URL()
	verifyPhoneNumber := creditflowapi.VerifyPhoneNumber.URL()
	identityDecision := creditflowapi.IdentityDecision.URL()

	graph = workflow.NewGraph(creditflowapi.IdentityVerification.URL())
	graph.DeclareInputs("applicantName", "ssn", "address", "phone", "faultInjection")
	graph.DeclareOutputs("identityVerified")
	// Init fans out to SSN, address, and phone verification
	graph.AddTransition(initIdentityVerification, verifySSN)
	graph.AddTransition(initIdentityVerification, verifyAddress)
	graph.AddTransition(initIdentityVerification, verifyPhoneNumber)
	// All verifications fan in to the identity decision
	graph.AddTransition(verifySSN, identityDecision)
	graph.AddTransition(verifyAddress, identityDecision)
	graph.AddTransition(verifyPhoneNumber, identityDecision)
	// Decision terminates the subgraph
	graph.AddTransition(identityDecision, workflow.END)
	// VerifyPhoneNumber has a tight time budget to test timeout enforcement
	graph.SetTimeBudget(verifyPhoneNumber, 1*time.Second)
	return graph, nil
}

/*
RequestMoreInfo requests additional information for the credit review and increments the review attempt counter.
*/
func (svc *Service) RequestMoreInfo(ctx context.Context, flow *workflow.Flow, reviewAttempts int) (reviewAttemptsOut int, err error) { // MARKER: RequestMoreInfo
	return reviewAttempts + 1, nil
}

/*
ReviewCredit performs a manual review of borderline credit scores.
For very borderline scores (550-579), it requests more info up to 2 times before deciding.
Scores of 580+ are approved. Below 550 are rejected.
*/
func (svc *Service) ReviewCredit(ctx context.Context, flow *workflow.Flow, creditScore int, creditVerified bool, reviewAttempts int, faultInjection string) (creditVerifiedOut bool, err error) { // MARKER: ReviewCredit
	// Fault: request goto to a non-existent target
	if strings.Contains(faultInjection, "BadGoto") {
		flow.Goto("https://credit.flow.example:428/non-existent-task")
		return creditVerified, nil
	}
	// Fault: sleep for 200ms before approving
	if strings.Contains(faultInjection, "Sleep") {
		flow.Sleep(200 * time.Millisecond)
		return true, nil
	}
	// Good scores (650+): pass through without review
	if creditScore >= 650 {
		return creditVerified, nil
	}
	// Scores 580-649 are approved after review
	if creditScore >= 580 {
		return true, nil
	}
	// Very borderline scores (550-579): request more info up to 2 times
	if creditScore >= 550 && reviewAttempts < 2 {
		flow.Goto(creditflowapi.RequestMoreInfo.URL())
		return creditVerified, nil
	}
	// After 2 attempts with more info, approve borderline scores
	if creditScore >= 550 {
		return true, nil
	}
	// Below 550: reject
	return creditVerified, nil
}

/*
Decision determines whether to approve the credit application based on verification results.
*/
func (svc *Service) Decision(ctx context.Context, flow *workflow.Flow, creditVerified bool, employmentFailures int, identityVerified bool) (approved bool, err error) { // MARKER: Decision
	approved = creditVerified && employmentFailures == 0 && identityVerified
	return approved, nil
}

// demoStep holds the data for a single step row in the demo template.
type demoStep struct {
	TaskName string
	Status   string
	Changes  string
	Indent   bool
}

// flattenSteps flattens the step history into a list of demoStep structs, indenting subgraph steps.
func flattenSteps(steps []foremanapi.FlowStep, indent bool) []demoStep {
	var result []demoStep
	for _, s := range steps {
		changes := ""
		if len(s.Changes) > 0 {
			b, _ := json.Marshal(s.Changes)
			changes = string(b)
		}
		taskName := s.TaskName
		if i := strings.LastIndex(taskName, "/"); i >= 0 {
			taskName = taskName[i+1:]
		}
		result = append(result, demoStep{
			TaskName: taskName,
			Status:   s.Status,
			Changes:  changes,
			Indent:   indent,
		})
		if len(s.SubHistory) > 0 {
			result = append(result, flattenSteps(s.SubHistory, true)...)
		}
	}
	return result
}

// demoResult holds the result of running the credit approval workflow.
type demoResult struct {
	status  string
	out     creditflowapi.CreditApprovalOut
	steps   []demoStep
	mermaid string
}

// runWorkflow creates, starts, awaits, and fetches the history of a credit approval workflow.
func (svc *Service) runWorkflow(ctx context.Context, foremanClient foremanapi.Client, initialState creditflowapi.CreditApprovalIn) (flowID string, result demoResult, err error) {
	flowID, err = foremanClient.Create(ctx, creditflowapi.CreditApproval.URL(), initialState)
	if err != nil {
		return "", result, errors.Trace(err)
	}
	err = foremanClient.Start(ctx, flowID)
	if err != nil {
		return flowID, result, errors.Trace(err)
	}
	result.status, err = foremanClient.AwaitAndParse(ctx, flowID, &result.out)
	if err != nil {
		return flowID, result, errors.Trace(err)
	}

	// Fetch history and Mermaid diagram in parallel
	svc.Parallel(
		func() error {
			steps, err := foremanClient.History(ctx, flowID)
			if err == nil {
				result.steps = flattenSteps(steps, false)
			}
			return nil
		},
		func() error {
			res, err := foremanClient.HistoryMermaid(ctx, "?flowKey="+flowID+"&format=raw")
			if err == nil {
				defer res.Body.Close()
				b, _ := io.ReadAll(res.Body)
				result.mermaid = string(b)
			}
			return nil
		},
	)
	return flowID, result, nil
}

/*
Demo serves the demo page for the credit approval workflow.
*/
func (svc *Service) Demo(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Demo
	ctx := r.Context()
	err = r.ParseForm()
	if err != nil {
		return errors.Trace(err, http.StatusBadRequest)
	}

	data := struct {
		Name               string
		SSN                string
		Address            string
		Phone              string
		Employers          string
		Score              string
		Fault              string
		Submitted          bool
		Error              string
		Status             string
		Approved           bool
		CreditVerified     bool
		IdentityVerified   bool
		EmploymentFailures int
		Steps              []demoStep
		MermaidDiagram     string
	}{
		Name:      r.FormValue("name"),
		SSN:       r.FormValue("ssn"),
		Address:   r.FormValue("address"),
		Phone:     r.FormValue("phone"),
		Employers: r.FormValue("employers"),
		Score:     r.FormValue("score"),
		Fault:     r.FormValue("fault"),
	}

	// Default values for GET with no params
	if r.Method == "GET" && data.Name == "" {
		data.Name = "Alice"
		data.SSN = "123-45-6789"
		data.Address = "123 Main St"
		data.Phone = "555-123-4567"
		data.Employers = "Acme Corp"
		data.Score = "750"
	}

	if r.Method == "POST" {
		data.Submitted = true

		// Build the applicant from form fields
		score, _ := strconv.Atoi(data.Score)
		var employers []string
		for _, e := range strings.Split(data.Employers, ",") {
			e = strings.TrimSpace(e)
			if e != "" {
				employers = append(employers, e)
			}
		}
		applicant := creditflowapi.Applicant{
			ApplicantName: data.Name,
			SSN:           data.SSN,
			Address:       data.Address,
			Phone:         data.Phone,
			Employers:     employers,
			CreditScore:   score,
		}

		// Create, start, await, and fetch history
		foremanClient := foremanapi.NewClient(svc)
		initialState := creditflowapi.CreditApprovalIn{
			Applicant:      applicant,
			FaultInjection: data.Fault,
		}
		_, result, runErr := svc.runWorkflow(ctx, foremanClient, initialState)
		if runErr != nil {
			data.Error = fmt.Sprintf("%+v", runErr)
		} else {
			data.Status = result.status
			data.Approved = result.out.Approved
			data.CreditVerified = result.out.CreditVerified
			data.IdentityVerified = result.out.IdentityVerified
			data.EmploymentFailures = result.out.EmploymentFailures
			data.Steps = result.steps
			data.MermaidDiagram = result.mermaid
		}
	}

	w.Header().Set("Content-Type", "text/html")
	err = svc.WriteResTemplate(w, "demo.html", data)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

/*
CreditApproval defines the workflow graph for the credit approval process.
*/
func (svc *Service) CreditApproval(ctx context.Context) (graph *workflow.Graph, err error) { // MARKER: CreditApproval
	submitCreditApplication := creditflowapi.SubmitCreditApplication.URL()
	verifyCredit := creditflowapi.VerifyCredit.URL()
	verifyEmployment := creditflowapi.VerifyEmployment.URL()
	identityVerification := creditflowapi.IdentityVerification.URL()
	reviewCredit := creditflowapi.ReviewCredit.URL()
	requestMoreInfo := creditflowapi.RequestMoreInfo.URL()
	decision := creditflowapi.Decision.URL()

	graph = workflow.NewGraph(creditflowapi.CreditApproval.URL())
	graph.DeclareInputs("applicant", "faultInjection")
	graph.DeclareOutputs("approved", "creditVerified", "employmentFailures", "identityVerified")
	// Identity verification is a subgraph with its own internal steps
	graph.AddSubgraph(identityVerification)
	handleCreditError := creditflowapi.HandleCreditError.URL()
	// Submit fans out to credit verification, identity verification (subgraph), and forEach employer verification
	graph.AddTransition(submitCreditApplication, verifyCredit)
	// If credit verification fails with an error, route to the error handler instead of failing the flow
	graph.AddErrorTransition(verifyCredit, handleCreditError)
	graph.AddTransition(handleCreditError, reviewCredit)
	graph.AddTransitionForEach(submitCreditApplication, verifyEmployment, "employers", "employerName")
	graph.AddTransition(submitCreditApplication, identityVerification)
	// Employment failure counts are summed across all employer verifications
	graph.SetReducer("employmentFailures", workflow.ReducerAdd)
	// All verifications fan in to review (which passes through for good scores)
	graph.AddTransition(verifyCredit, reviewCredit)
	graph.AddTransition(verifyEmployment, reviewCredit)
	graph.AddTransition(identityVerification, reviewCredit)
	// Review can goto RequestMoreInfo for borderline scores; RequestMoreInfo loops back to review
	graph.AddTransitionGoto(reviewCredit, requestMoreInfo)
	graph.AddTransition(requestMoreInfo, reviewCredit)
	// Review feeds into decision
	graph.AddTransition(reviewCredit, decision)
	// Decision terminates the workflow
	graph.AddTransition(decision, workflow.END)
	return graph, nil
}

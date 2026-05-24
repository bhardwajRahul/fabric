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

// outcomeStatus extracts the Status from a FlowOutcome, returning "" on nil.
func outcomeStatus(o *workflow.FlowOutcome) string {
	if o == nil {
		return ""
	}
	return o.Status
}

// outcomeState extracts the State from a FlowOutcome, returning nil on nil.
func outcomeState(o *workflow.FlowOutcome) map[string]any {
	if o == nil {
		return nil
	}
	return o.State
}

// outcomeStatusState extracts the Status and State from a FlowOutcome.
func outcomeStatusState(o *workflow.FlowOutcome) (string, map[string]any) {
	if o == nil {
		return "", nil
	}
	return o.Status, o.State
}

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
func (svc *Service) VerifyCredit(ctx context.Context, flow *workflow.Flow, creditScore int) (creditVerified bool, err error) { // MARKER: VerifyCredit
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
func (svc *Service) VerifyEmployment(ctx context.Context, flow *workflow.Flow, applicantName string, employerName string) (sumEmploymentFailuresOut int, err error) { // MARKER: VerifyEmployment
	if applicantName == "" || employerName == "" {
		return 1, nil
	}
	return 0, nil
}

/*
InitIdentityVerification is the entry point for the identity verification subgraph. It passes through the applicant name.
*/
func (svc *Service) InitIdentityVerification(ctx context.Context, flow *workflow.Flow, applicantName string, ssn string, address string, phone string) (err error) { // MARKER: InitIdentityVerification
	return nil
}

/*
VerifySSN checks the applicant's SSN.
*/
func (svc *Service) VerifySSN(ctx context.Context, flow *workflow.Flow, ssn string) (ssnVerified bool, err error) { // MARKER: VerifySSN
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
func (svc *Service) VerifyPhoneNumber(ctx context.Context, flow *workflow.Flow, phone string) (phoneVerified bool, err error) { // MARKER: VerifyPhoneNumber
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
	graph = workflow.NewGraph(creditflowapi.IdentityVerification.URL())
	graph.AddTask("initIdentityVerification", creditflowapi.InitIdentityVerification.URL())
	graph.AddTask("verifySSN", creditflowapi.VerifySSN.URL())
	graph.AddTask("verifyAddress", creditflowapi.VerifyAddress.URL())
	graph.AddTask("verifyPhoneNumber", creditflowapi.VerifyPhoneNumber.URL())
	graph.AddTask("identityDecision", creditflowapi.IdentityDecision.URL())
	graph.SetFanIn("identityDecision")
	// Init fans out to SSN, address, and phone verification
	graph.AddTransition("initIdentityVerification", "verifySSN")
	graph.AddTransition("initIdentityVerification", "verifyAddress")
	graph.AddTransition("initIdentityVerification", "verifyPhoneNumber")
	// All verifications fan in to the identity decision
	graph.AddTransition("verifySSN", "identityDecision")
	graph.AddTransition("verifyAddress", "identityDecision")
	graph.AddTransition("verifyPhoneNumber", "identityDecision")
	// Decision terminates the subgraph
	graph.AddTransition("identityDecision", workflow.END)
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
func (svc *Service) ReviewCredit(ctx context.Context, flow *workflow.Flow, creditScore int, creditVerified bool, reviewAttempts int) (creditVerifiedOut bool, err error) { // MARKER: ReviewCredit
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
func (svc *Service) Decision(ctx context.Context, flow *workflow.Flow, creditVerified bool, sumEmploymentFailures int, identityVerified bool) (approved bool, err error) { // MARKER: Decision
	approved = creditVerified && sumEmploymentFailures == 0 && identityVerified
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
func (svc *Service) runWorkflow(ctx context.Context, foremanClient foremanapi.Client, initialState creditflowapi.CreditApprovalIn) (flowKey string, result demoResult, err error) {
	flowKey, err = foremanClient.Create(ctx, creditflowapi.CreditApproval.URL(), initialState, nil)
	if err != nil {
		return "", result, errors.Trace(err)
	}
	err = foremanClient.Start(ctx, flowKey)
	if err != nil {
		return flowKey, result, errors.Trace(err)
	}
	outcome, err := foremanClient.AwaitAndParse(ctx, flowKey, &result.out)

	result.status = outcomeStatus(outcome)
	if err != nil {
		return flowKey, result, errors.Trace(err)
	}

	// Fetch history and Mermaid diagram in parallel
	svc.Parallel(
		func() error {
			steps, err := foremanClient.History(ctx, flowKey)
			if err == nil {
				result.steps = flattenSteps(steps, false)
			}
			return nil
		},
		func() error {
			res, err := foremanClient.HistoryMermaid(ctx, "?flowKey="+flowKey+"&format=raw")
			if err == nil {
				defer res.Body.Close()
				b, _ := io.ReadAll(res.Body)
				result.mermaid = string(b)
			}
			return nil
		},
	)
	return flowKey, result, nil
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
		Submitted          bool
		Error              string
		Status             string
		Approved           bool
		CreditVerified     bool
		IdentityVerified   bool
		SumEmploymentFailures int
		Steps                 []demoStep
		MermaidDiagram        string
	}{
		Name:      r.FormValue("name"),
		SSN:       r.FormValue("ssn"),
		Address:   r.FormValue("address"),
		Phone:     r.FormValue("phone"),
		Employers: r.FormValue("employers"),
		Score:     r.FormValue("score"),
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
			Applicant: applicant,
		}
		_, result, runErr := svc.runWorkflow(ctx, foremanClient, initialState)
		if runErr != nil {
			data.Error = fmt.Sprintf("%+v", runErr)
		} else {
			data.Status = result.status
			data.Approved = result.out.Approved
			data.CreditVerified = result.out.CreditVerified
			data.IdentityVerified = result.out.IdentityVerified
			data.SumEmploymentFailures = result.out.SumEmploymentFailures
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
	graph = workflow.NewGraph(creditflowapi.CreditApproval.URL())
	graph.AddTask("submitCreditApplication", creditflowapi.SubmitCreditApplication.URL())
	graph.AddTask("verifyCredit", creditflowapi.VerifyCredit.URL())
	graph.AddTask("verifyEmployment", creditflowapi.VerifyEmployment.URL())
	graph.AddSubgraph("identityVerification", creditflowapi.IdentityVerification.URL())
	graph.AddTask("handleCreditError", creditflowapi.HandleCreditError.URL())
	// reviewJoin and reviewCredit are two graph positions sharing the same task URL.
	// reviewJoin is the fan-in nexus for the submit cohort; reviewCredit is reached
	// sequentially from reviewJoin and hosts the goto loop with requestMoreInfo.
	// Splitting them lets the lineage validator close the cohort frame at reviewJoin
	// without conflicting with the goto re-entry into reviewCredit.
	graph.AddTask("reviewJoin", creditflowapi.ReviewCredit.URL())
	graph.AddTask("reviewCredit", creditflowapi.ReviewCredit.URL())
	graph.AddTask("requestMoreInfo", creditflowapi.RequestMoreInfo.URL())
	graph.AddTask("decision", creditflowapi.Decision.URL())
	graph.SetFanIn("reviewJoin")
	// Submit fans out to credit verification, identity verification (subgraph), and forEach employer verification
	graph.AddTransition("submitCreditApplication", "verifyCredit")
	// If credit verification fails with an error, route to the error handler instead of failing the flow
	graph.AddTransitionOnError("verifyCredit", "handleCreditError")
	graph.AddTransition("handleCreditError", "reviewJoin")
	graph.AddTransitionForEach("submitCreditApplication", "verifyEmployment", "employers", "employerName")
	graph.AddTransition("submitCreditApplication", "identityVerification")
	// Employment failure counts are summed across all employer verifications via the
	// sum* prefix convention; no explicit reducer is needed.
	// All verifications fan in to reviewJoin (the fan-in nexus).
	graph.AddTransition("verifyCredit", "reviewJoin")
	graph.AddTransition("verifyEmployment", "reviewJoin")
	graph.AddTransition("identityVerification", "reviewJoin")
	// reviewJoin runs ReviewCredit's logic once on the merged state; if it gotos, the loop
	// runs through requestMoreInfo and reviewCredit.
	graph.AddTransitionGoto("reviewJoin", "requestMoreInfo")
	graph.AddTransition("reviewJoin", "reviewCredit")
	// reviewCredit may itself goto requestMoreInfo for borderline scores; requestMoreInfo
	// loops back to reviewCredit (not reviewJoin — the cohort frame has already been closed).
	graph.AddTransitionGoto("reviewCredit", "requestMoreInfo")
	graph.AddTransition("requestMoreInfo", "reviewCredit")
	// Review feeds into decision
	graph.AddTransition("reviewCredit", "decision")
	// Decision terminates the workflow
	graph.AddTransition("decision", workflow.END)
	return graph, nil
}

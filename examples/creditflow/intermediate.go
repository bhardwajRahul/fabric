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
	"regexp"
	"strconv"
	"time"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/cfg"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/fabric/openapi"
	"github.com/microbus-io/fabric/sub"
	"github.com/microbus-io/fabric/utils"
	"github.com/microbus-io/fabric/workflow"

	"github.com/microbus-io/fabric/examples/creditflow/creditflowapi"
	"github.com/microbus-io/fabric/examples/creditflow/resources"
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
	_ creditflowapi.Client
	_ *workflow.Flow
)

const (
	Hostname = creditflowapi.Hostname
	Version  = 5
)

// ToDo is implemented by the service or mock.
// The intermediate delegates handling to this interface.
type ToDo interface {
	OnStartup(ctx context.Context) (err error)
	OnShutdown(ctx context.Context) (err error)
	SubmitCreditApplication(ctx context.Context, flow *workflow.Flow, applicant creditflowapi.Applicant) (applicantName string, ssn string, address string, phone string, employers []string, creditScore int, err error) // MARKER: SubmitCreditApplication
	VerifyCredit(ctx context.Context, flow *workflow.Flow, creditScore int, faultInjection string) (creditVerified bool, err error)                                                                                       // MARKER: VerifyCredit
	VerifyEmployment(ctx context.Context, flow *workflow.Flow, applicantName string, employerName string) (employmentFailures int, err error)                                                                             // MARKER: VerifyEmployment
	InitIdentityVerification(ctx context.Context, flow *workflow.Flow, applicantName string, ssn string, address string, phone string) (err error)                                                                        // MARKER: InitIdentityVerification
	VerifySSN(ctx context.Context, flow *workflow.Flow, ssn string, faultInjection string) (ssnVerified bool, err error)                                                                                                  // MARKER: VerifySSN
	VerifyAddress(ctx context.Context, flow *workflow.Flow, address string) (addressVerified bool, err error)                                                                                                             // MARKER: VerifyAddress
	VerifyPhoneNumber(ctx context.Context, flow *workflow.Flow, phone string, faultInjection string) (phoneVerified bool, err error)                                                                                      // MARKER: VerifyPhoneNumber
	IdentityDecision(ctx context.Context, flow *workflow.Flow, ssnVerified bool, addressVerified bool, phoneVerified bool) (identityVerified bool, err error)                                                             // MARKER: IdentityDecision
	IdentityVerification(ctx context.Context) (graph *workflow.Graph, err error)                                                                                                                                          // MARKER: IdentityVerification
	RequestMoreInfo(ctx context.Context, flow *workflow.Flow, reviewAttempts int) (reviewAttemptsOut int, err error)                                                                                                      // MARKER: RequestMoreInfo
	ReviewCredit(ctx context.Context, flow *workflow.Flow, creditScore int, creditVerified bool, reviewAttempts int, faultInjection string) (creditVerifiedOut bool, err error)                                           // MARKER: ReviewCredit
	HandleCreditError(ctx context.Context, flow *workflow.Flow, onErr *errors.TracedError) (creditVerified bool, err error)                                                                                               // MARKER: HandleCreditError
	Decision(ctx context.Context, flow *workflow.Flow, creditVerified bool, employmentFailures int, identityVerified bool) (approved bool, err error)                                                                     // MARKER: Decision
	CreditApproval(ctx context.Context) (graph *workflow.Graph, err error)                                                                                                                                                // MARKER: CreditApproval
	Demo(w http.ResponseWriter, r *http.Request) (err error)                                                                                                                                                              // MARKER: Demo
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
	svc.SetDescription(`CreditFlow is an example microservice that demonstrates agentic workflow features.`)
	svc.SetOnStartup(svc.OnStartup)
	svc.SetOnShutdown(svc.OnShutdown)
	svc.Subscribe("GET", ":0/openapi.json", svc.doOpenAPI)
	svc.SetResFS(resources.FS)
	svc.SetOnObserveMetrics(svc.doOnObserveMetrics)
	svc.SetOnConfigChanged(svc.doOnConfigChanged)

	// HINT: Add functional endpoints here

	// HINT: Add web endpoints here
	svc.Subscribe(creditflowapi.Demo.Method, creditflowapi.Demo.Route, svc.Demo) // MARKER: Demo

	// HINT: Add metrics here

	// HINT: Add tickers here

	// HINT: Add configs here

	// HINT: Add inbound event sinks here

	// HINT: Add task endpoints here
	svc.Subscribe(creditflowapi.SubmitCreditApplication.Method, creditflowapi.SubmitCreditApplication.Route, svc.doSubmitCreditApplication)    // MARKER: SubmitCreditApplication
	svc.Subscribe(creditflowapi.VerifyCredit.Method, creditflowapi.VerifyCredit.Route, svc.doVerifyCredit)                                     // MARKER: VerifyCredit
	svc.Subscribe(creditflowapi.VerifyEmployment.Method, creditflowapi.VerifyEmployment.Route, svc.doVerifyEmployment)                         // MARKER: VerifyEmployment
	svc.Subscribe(creditflowapi.InitIdentityVerification.Method, creditflowapi.InitIdentityVerification.Route, svc.doInitIdentityVerification) // MARKER: InitIdentityVerification
	svc.Subscribe(creditflowapi.VerifySSN.Method, creditflowapi.VerifySSN.Route, svc.doVerifySSN)                                              // MARKER: VerifySSN
	svc.Subscribe(creditflowapi.VerifyAddress.Method, creditflowapi.VerifyAddress.Route, svc.doVerifyAddress)                                  // MARKER: VerifyAddress
	svc.Subscribe(creditflowapi.VerifyPhoneNumber.Method, creditflowapi.VerifyPhoneNumber.Route, svc.doVerifyPhoneNumber)                      // MARKER: VerifyPhoneNumber
	svc.Subscribe(creditflowapi.IdentityDecision.Method, creditflowapi.IdentityDecision.Route, svc.doIdentityDecision)                         // MARKER: IdentityDecision
	svc.Subscribe(creditflowapi.RequestMoreInfo.Method, creditflowapi.RequestMoreInfo.Route, svc.doRequestMoreInfo)                            // MARKER: RequestMoreInfo
	svc.Subscribe(creditflowapi.ReviewCredit.Method, creditflowapi.ReviewCredit.Route, svc.doReviewCredit)                                     // MARKER: ReviewCredit
	svc.Subscribe(creditflowapi.HandleCreditError.Method, creditflowapi.HandleCreditError.Route, svc.doHandleCreditError)                      // MARKER: HandleCreditError
	svc.Subscribe(creditflowapi.Decision.Method, creditflowapi.Decision.Route, svc.doDecision)                                                 // MARKER: Decision

	// HINT: Add graph endpoints here
	svc.Subscribe(creditflowapi.IdentityVerification.Method, creditflowapi.IdentityVerification.Route, svc.doIdentityVerification) // MARKER: IdentityVerification
	svc.Subscribe(creditflowapi.CreditApproval.Method, creditflowapi.CreditApproval.Route, svc.doCreditApproval)                   // MARKER: CreditApproval

	_ = marshalFunction
	return svc
}

// doOpenAPI renders the OpenAPI document of the microservice.
func (svc *Intermediate) doOpenAPI(w http.ResponseWriter, r *http.Request) (err error) {
	oapiSvc := openapi.Service{
		ServiceName: svc.Hostname(),
		Description: svc.Description(),
		Version:     svc.Version(),
		Endpoints:   []*openapi.Endpoint{},
		RemoteURI:   frame.Of(r).XForwardedFullURL(),
	}

	endpoints := []*openapi.Endpoint{
		// HINT: Register web handlers and functional endpoints by adding them here
		{ // MARKER: Demo
			Type:        "web",
			Name:        "Demo",
			Method:      creditflowapi.Demo.Method,
			Route:       creditflowapi.Demo.Route,
			Summary:     "Demo()",
			Description: `Demo serves the demo page for the credit approval workflow.`,
		},
		{ // MARKER: IdentityVerification
			Type:        "workflow",
			Name:        "IdentityVerification",
			Method:      "POST",
			Route:       creditflowapi.IdentityVerification.Route,
			Summary:     "IdentityVerification(applicantName string, ssn string, address string, phone string) (identityVerified bool)",
			Description: `IdentityVerification defines the workflow graph for the identity verification process.`,
			InputArgs:   creditflowapi.IdentityVerificationIn{},
			OutputArgs:  creditflowapi.IdentityVerificationOut{},
		},
		{ // MARKER: CreditApproval
			Type:        "workflow",
			Name:        "CreditApproval",
			Method:      "POST",
			Route:       creditflowapi.CreditApproval.Route,
			Summary:     "CreditApproval(applicant Applicant, faultInjection string) (approved bool, creditVerified bool, employmentFailures int, identityVerified bool)",
			Description: `CreditApproval defines the workflow graph for the full credit approval process.`,
			InputArgs:   creditflowapi.CreditApprovalIn{},
			OutputArgs:  creditflowapi.CreditApprovalOut{},
		},
	}

	// Filter by the port of the request
	rePort := regexp.MustCompile(`:(` + regexp.QuoteMeta(r.URL.Port()) + `|0)(/|$)`)
	reAnyPort := regexp.MustCompile(`:[0-9]+(/|$)`)
	for _, ep := range endpoints {
		if rePort.MatchString(ep.Route) || r.URL.Port() == "443" && !reAnyPort.MatchString(ep.Route) {
			oapiSvc.Endpoints = append(oapiSvc.Endpoints, ep)
		}
	}
	if len(oapiSvc.Endpoints) == 0 {
		w.WriteHeader(http.StatusNotFound)
		return nil
	}
	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	if svc.Deployment() == connector.LOCAL {
		encoder.SetIndent("", "  ")
	}
	err = encoder.Encode(&oapiSvc)
	return errors.Trace(err)
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

// doSubmitCreditApplication handles marshaling for SubmitCreditApplication.
func (svc *Intermediate) doSubmitCreditApplication(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: SubmitCreditApplication
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in creditflowapi.SubmitCreditApplicationIn
	flow.ParseState(&in)
	var out creditflowapi.SubmitCreditApplicationOut
	out.ApplicantName, out.SSN, out.Address, out.Phone, out.Employers, out.CreditScore, err = svc.SubmitCreditApplication(r.Context(), &flow, in.Applicant)
	if err != nil {
		return err // No trace
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// doVerifyCredit handles marshaling for VerifyCredit.
func (svc *Intermediate) doVerifyCredit(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: VerifyCredit
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in creditflowapi.VerifyCreditIn
	flow.ParseState(&in)
	var out creditflowapi.VerifyCreditOut
	out.CreditVerified, err = svc.VerifyCredit(r.Context(), &flow, in.CreditScore, in.FaultInjection)
	if err != nil {
		return err // No trace
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// doVerifyEmployment handles marshaling for VerifyEmployment.
func (svc *Intermediate) doVerifyEmployment(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: VerifyEmployment
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in creditflowapi.VerifyEmploymentIn
	flow.ParseState(&in)
	var out creditflowapi.VerifyEmploymentOut
	out.EmploymentFailures, err = svc.VerifyEmployment(r.Context(), &flow, in.ApplicantName, in.EmployerName)
	if err != nil {
		return err // No trace
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// doInitIdentityVerification handles marshaling for InitIdentityVerification.
func (svc *Intermediate) doInitIdentityVerification(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: InitIdentityVerification
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in creditflowapi.InitIdentityVerificationIn
	flow.ParseState(&in)
	var out creditflowapi.InitIdentityVerificationOut
	err = svc.InitIdentityVerification(r.Context(), &flow, in.ApplicantName, in.SSN, in.Address, in.Phone)
	if err != nil {
		return err // No trace
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// doVerifySSN handles marshaling for VerifySSN.
func (svc *Intermediate) doVerifySSN(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: VerifySSN
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in creditflowapi.VerifySSNIn
	flow.ParseState(&in)
	var out creditflowapi.VerifySSNOut
	out.SsnVerified, err = svc.VerifySSN(r.Context(), &flow, in.SSN, in.FaultInjection)
	if err != nil {
		return err // No trace
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// doVerifyAddress handles marshaling for VerifyAddress.
func (svc *Intermediate) doVerifyAddress(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: VerifyAddress
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in creditflowapi.VerifyAddressIn
	flow.ParseState(&in)
	var out creditflowapi.VerifyAddressOut
	out.AddressVerified, err = svc.VerifyAddress(r.Context(), &flow, in.Address)
	if err != nil {
		return err // No trace
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// doVerifyPhoneNumber handles marshaling for VerifyPhoneNumber.
func (svc *Intermediate) doVerifyPhoneNumber(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: VerifyPhoneNumber
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in creditflowapi.VerifyPhoneNumberIn
	flow.ParseState(&in)
	var out creditflowapi.VerifyPhoneNumberOut
	out.PhoneVerified, err = svc.VerifyPhoneNumber(r.Context(), &flow, in.Phone, in.FaultInjection)
	if err != nil {
		return err // No trace
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// doIdentityDecision handles marshaling for IdentityDecision.
func (svc *Intermediate) doIdentityDecision(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: IdentityDecision
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in creditflowapi.IdentityDecisionIn
	flow.ParseState(&in)
	var out creditflowapi.IdentityDecisionOut
	out.IdentityVerified, err = svc.IdentityDecision(r.Context(), &flow, in.SsnVerified, in.AddressVerified, in.PhoneVerified)
	if err != nil {
		return err // No trace
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// doIdentityVerification handles marshaling for IdentityVerification.
func (svc *Intermediate) doIdentityVerification(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: IdentityVerification
	graph, err := svc.IdentityVerification(r.Context())
	if err != nil {
		return err // No trace
	}
	err = graph.Validate()
	if err != nil {
		return errors.Trace(err)
	}
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(struct {
		Graph *workflow.Graph `json:"graph"`
	}{Graph: graph})
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// doRequestMoreInfo handles marshaling for RequestMoreInfo.
func (svc *Intermediate) doRequestMoreInfo(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: RequestMoreInfo
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in creditflowapi.RequestMoreInfoIn
	flow.ParseState(&in)
	var out creditflowapi.RequestMoreInfoOut
	out.ReviewAttemptsOut, err = svc.RequestMoreInfo(r.Context(), &flow, in.ReviewAttempts)
	if err != nil {
		return err // No trace
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// doReviewCredit handles marshaling for ReviewCredit.
func (svc *Intermediate) doReviewCredit(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: ReviewCredit
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in creditflowapi.ReviewCreditIn
	flow.ParseState(&in)
	var out creditflowapi.ReviewCreditOut
	out.CreditVerifiedOut, err = svc.ReviewCredit(r.Context(), &flow, in.CreditScore, in.CreditVerified, in.ReviewAttempts, in.FaultInjection)
	if err != nil {
		return err // No trace
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// doHandleCreditError handles marshaling for HandleCreditError.
func (svc *Intermediate) doHandleCreditError(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: HandleCreditError
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in creditflowapi.HandleCreditErrorIn
	flow.ParseState(&in)
	var out creditflowapi.HandleCreditErrorOut
	out.CreditVerified, err = svc.HandleCreditError(r.Context(), &flow, in.OnErr)
	if err != nil {
		return err // No trace
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// doDecision handles marshaling for Decision.
func (svc *Intermediate) doDecision(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: Decision
	var flow workflow.Flow
	err = json.NewDecoder(r.Body).Decode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	snap := flow.Snapshot()
	var in creditflowapi.DecisionIn
	flow.ParseState(&in)
	var out creditflowapi.DecisionOut
	out.Approved, err = svc.Decision(r.Context(), &flow, in.CreditVerified, in.EmploymentFailures, in.IdentityVerified)
	if err != nil {
		return err // No trace
	}
	flow.SetChanges(out, snap)
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(&flow)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

// doCreditApproval handles marshaling for CreditApproval.
func (svc *Intermediate) doCreditApproval(w http.ResponseWriter, r *http.Request) (err error) { // MARKER: CreditApproval
	graph, err := svc.CreditApproval(r.Context())
	if err != nil {
		return err // No trace
	}
	err = graph.Validate()
	if err != nil {
		return errors.Trace(err)
	}
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(struct {
		Graph *workflow.Graph `json:"graph"`
	}{Graph: graph})
	if err != nil {
		return errors.Trace(err)
	}
	return nil
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

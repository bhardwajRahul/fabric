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

package creditflowapi

import (
	"context"
	"encoding/json"
	"iter"
	"net/http"
	"reflect"

	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/fabric/pub"
	"github.com/microbus-io/fabric/service"
	"github.com/microbus-io/fabric/sub"
	"github.com/microbus-io/fabric/workflow"
)

var (
	_ context.Context
	_ json.Encoder
	_ *http.Request
	_ *errors.TracedError
	_ *httpx.BodyReader
	_ = marshalRequest
	_ = marshalPublish
	_ = marshalFunction
	_ = marshalTask
	_ = marshalWorkflow
	_ workflow.Flow
)

// multicastResponse packs the response of a functional multicast.
type multicastResponse struct {
	data         any
	HTTPResponse *http.Response
	err          error
}

// Client is a lightweight proxy for making unicast calls to the microservice.
type Client struct {
	svc  service.Publisher
	host string
	opts []pub.Option
}

// NewClient creates a new unicast client proxy to the microservice.
func NewClient(caller service.Publisher) Client {
	return Client{svc: caller, host: Hostname}
}

// ForHost returns a copy of the client with a different hostname to be applied to requests.
func (_c Client) ForHost(host string) Client {
	return Client{
		svc:  _c.svc,
		host: host,
		opts: _c.opts,
	}
}

// WithOptions returns a copy of the client with options to be applied to requests.
func (_c Client) WithOptions(opts ...pub.Option) Client {
	return Client{svc: _c.svc, host: _c.host, opts: append(_c.opts, opts...)}
}

// MulticastClient is a lightweight proxy for making multicast calls to the microservice.
type MulticastClient struct {
	svc  service.Publisher
	host string
	opts []pub.Option
}

// NewMulticastClient creates a new multicast client proxy to the microservice.
func NewMulticastClient(caller service.Publisher) MulticastClient {
	return MulticastClient{svc: caller, host: Hostname}
}

// ForHost returns a copy of the client with a different hostname to be applied to requests.
func (_c MulticastClient) ForHost(host string) MulticastClient {
	return MulticastClient{svc: _c.svc, host: host, opts: _c.opts}
}

// WithOptions returns a copy of the client with options to be applied to requests.
func (_c MulticastClient) WithOptions(opts ...pub.Option) MulticastClient {
	return MulticastClient{svc: _c.svc, host: _c.host, opts: append(_c.opts, opts...)}
}

// WorkflowRunner executes a workflow by name with initial state, blocking until termination.
// foremanapi.Client satisfies this interface.
type WorkflowRunner interface {
	Run(ctx context.Context, workflowName string, initialState any) (status string, state map[string]any, err error)
}

// Executor runs tasks and workflows synchronously, blocking until termination.
// It is primarily intended for integration tests.
type Executor struct {
	svc     service.Publisher
	host    string
	opts    []pub.Option
	inFlow  *workflow.Flow
	outFlow *workflow.Flow
	runner  WorkflowRunner
}

// NewExecutor creates a new executor proxy to the microservice.
func NewExecutor(caller service.Publisher) Executor {
	return Executor{svc: caller, host: Hostname}
}

// ForHost returns a copy of the executor with a different hostname to be applied to requests.
func (_c Executor) ForHost(host string) Executor {
	return Executor{svc: _c.svc, host: host, opts: _c.opts, inFlow: _c.inFlow, outFlow: _c.outFlow, runner: _c.runner}
}

// WithOptions returns a copy of the executor with options to be applied to requests.
func (_c Executor) WithOptions(opts ...pub.Option) Executor {
	return Executor{svc: _c.svc, host: _c.host, opts: append(_c.opts, opts...), inFlow: _c.inFlow, outFlow: _c.outFlow, runner: _c.runner}
}

// WithInputFlow returns a copy of the executor with an input flow to use for task execution.
// The input flow's state is available to the task in addition to the typed input arguments.
func (_c Executor) WithInputFlow(flow *workflow.Flow) Executor {
	return Executor{svc: _c.svc, host: _c.host, opts: _c.opts, inFlow: flow, outFlow: _c.outFlow, runner: _c.runner}
}

// WithOutputFlow returns a copy of the executor with an output flow to populate after task execution.
// The output flow captures the full flow state including control signals (Goto, Retry, Interrupt, Sleep).
func (_c Executor) WithOutputFlow(flow *workflow.Flow) Executor {
	return Executor{svc: _c.svc, host: _c.host, opts: _c.opts, inFlow: _c.inFlow, outFlow: flow, runner: _c.runner}
}

// WithWorkflowRunner returns a copy of the executor with a workflow runner for executing workflows.
// foremanapi.NewClient(svc) satisfies the WorkflowRunner interface.
func (_c Executor) WithWorkflowRunner(runner WorkflowRunner) Executor {
	return Executor{svc: _c.svc, host: _c.host, opts: _c.opts, inFlow: _c.inFlow, outFlow: _c.outFlow, runner: runner}
}

// MulticastTrigger is a lightweight proxy for triggering the events of the microservice.
type MulticastTrigger struct {
	svc  service.Publisher
	host string
	opts []pub.Option
}

// NewMulticastTrigger creates a new multicast trigger of events of the microservice.
func NewMulticastTrigger(caller service.Publisher) MulticastTrigger {
	return MulticastTrigger{svc: caller, host: Hostname}
}

// ForHost returns a copy of the trigger with a different hostname to be applied to requests.
func (_c MulticastTrigger) ForHost(host string) MulticastTrigger {
	return MulticastTrigger{svc: _c.svc, host: host, opts: _c.opts}
}

// WithOptions returns a copy of the trigger with options to be applied to requests.
func (_c MulticastTrigger) WithOptions(opts ...pub.Option) MulticastTrigger {
	return MulticastTrigger{svc: _c.svc, host: _c.host, opts: append(_c.opts, opts...)}
}

// Hook assists in the subscription to the events of the microservice.
type Hook struct {
	svc  service.Subscriber
	host string
	opts []sub.Option
}

// NewHook creates a new hook to the events of the microservice.
func NewHook(listener service.Subscriber) Hook {
	return Hook{svc: listener, host: Hostname}
}

// ForHost returns a copy of the hook with a different hostname to be applied to the subscription.
func (c Hook) ForHost(host string) Hook {
	return Hook{svc: c.svc, host: host, opts: c.opts}
}

// WithOptions returns a copy of the hook with options to be applied to subscriptions.
func (c Hook) WithOptions(opts ...sub.Option) Hook {
	return Hook{svc: c.svc, host: c.host, opts: append(c.opts, opts...)}
}

/*
SubmitCreditApplication receives a credit application and sets up the workflow state.
*/
func (_c Executor) SubmitCreditApplication(ctx context.Context, applicant Applicant) (applicantName string, ssn string, address string, phone string, employers []string, creditScore int, err error) { // MARKER: SubmitCreditApplication
	var out SubmitCreditApplicationOut
	err = marshalTask(ctx, _c.svc, _c.opts, _c.host, SubmitCreditApplication.Method, SubmitCreditApplication.Route, SubmitCreditApplicationIn{
		Applicant: applicant,
	}, &out, _c.inFlow, _c.outFlow)
	return out.ApplicantName, out.SSN, out.Address, out.Phone, out.Employers, out.CreditScore, err // No trace
}

/*
VerifyCredit checks the applicant's credit score.
*/
func (_c Executor) VerifyCredit(ctx context.Context, creditScore int, faultInjection string) (creditVerified bool, err error) { // MARKER: VerifyCredit
	var out VerifyCreditOut
	err = marshalTask(ctx, _c.svc, _c.opts, _c.host, VerifyCredit.Method, VerifyCredit.Route, VerifyCreditIn{
		CreditScore:    creditScore,
		FaultInjection: faultInjection,
	}, &out, _c.inFlow, _c.outFlow)
	return out.CreditVerified, err // No trace
}

/*
VerifyEmployment checks the applicant's employment status.
*/
func (_c Executor) VerifyEmployment(ctx context.Context, applicantName string, employerName string) (employmentFailures int, err error) { // MARKER: VerifyEmployment
	var out VerifyEmploymentOut
	err = marshalTask(ctx, _c.svc, _c.opts, _c.host, VerifyEmployment.Method, VerifyEmployment.Route, VerifyEmploymentIn{
		ApplicantName: applicantName,
		EmployerName:  employerName,
	}, &out, _c.inFlow, _c.outFlow)
	return out.EmploymentFailures, err // No trace
}

/*
InitIdentityVerification is the entry point for the identity verification subgraph.
*/
func (_c Executor) InitIdentityVerification(ctx context.Context, applicantName string, ssn string, address string, phone string) (err error) { // MARKER: InitIdentityVerification
	err = marshalTask(ctx, _c.svc, _c.opts, _c.host, InitIdentityVerification.Method, InitIdentityVerification.Route, InitIdentityVerificationIn{
		ApplicantName: applicantName,
		SSN:           ssn,
		Address:       address,
		Phone:         phone,
	}, nil, _c.inFlow, _c.outFlow)
	return err // No trace
}

/*
VerifySSN checks the applicant's SSN.
*/
func (_c Executor) VerifySSN(ctx context.Context, ssn string, faultInjection string) (ssnVerified bool, err error) { // MARKER: VerifySSN
	var out VerifySSNOut
	err = marshalTask(ctx, _c.svc, _c.opts, _c.host, VerifySSN.Method, VerifySSN.Route, VerifySSNIn{
		SSN:            ssn,
		FaultInjection: faultInjection,
	}, &out, _c.inFlow, _c.outFlow)
	return out.SsnVerified, err // No trace
}

/*
VerifyAddress checks the applicant's address.
*/
func (_c Executor) VerifyAddress(ctx context.Context, address string) (addressVerified bool, err error) { // MARKER: VerifyAddress
	var out VerifyAddressOut
	err = marshalTask(ctx, _c.svc, _c.opts, _c.host, VerifyAddress.Method, VerifyAddress.Route, VerifyAddressIn{
		Address: address,
	}, &out, _c.inFlow, _c.outFlow)
	return out.AddressVerified, err // No trace
}

/*
VerifyPhoneNumber checks the applicant's phone number.
*/
func (_c Executor) VerifyPhoneNumber(ctx context.Context, phone string, faultInjection string) (phoneVerified bool, err error) { // MARKER: VerifyPhoneNumber
	var out VerifyPhoneNumberOut
	err = marshalTask(ctx, _c.svc, _c.opts, _c.host, VerifyPhoneNumber.Method, VerifyPhoneNumber.Route, VerifyPhoneNumberIn{
		Phone:          phone,
		FaultInjection: faultInjection,
	}, &out, _c.inFlow, _c.outFlow)
	return out.PhoneVerified, err // No trace
}

/*
IdentityDecision determines whether the applicant's identity is verified based on SSN, address, and phone checks.
*/
func (_c Executor) IdentityDecision(ctx context.Context, ssnVerified bool, addressVerified bool, phoneVerified bool) (identityVerified bool, err error) { // MARKER: IdentityDecision
	var out IdentityDecisionOut
	err = marshalTask(ctx, _c.svc, _c.opts, _c.host, IdentityDecision.Method, IdentityDecision.Route, IdentityDecisionIn{
		SsnVerified:     ssnVerified,
		AddressVerified: addressVerified,
		PhoneVerified:   phoneVerified,
	}, &out, _c.inFlow, _c.outFlow)
	return out.IdentityVerified, err // No trace
}

/*
RequestMoreInfo requests additional information for the credit review and increments the review attempt counter.
*/
func (_c Executor) RequestMoreInfo(ctx context.Context, reviewAttempts int) (reviewAttemptsOut int, err error) { // MARKER: RequestMoreInfo
	var out RequestMoreInfoOut
	err = marshalTask(ctx, _c.svc, _c.opts, _c.host, RequestMoreInfo.Method, RequestMoreInfo.Route, RequestMoreInfoIn{
		ReviewAttempts: reviewAttempts,
	}, &out, _c.inFlow, _c.outFlow)
	return out.ReviewAttemptsOut, err // No trace
}

/*
ReviewCredit performs a manual review of borderline credit scores.
*/
func (_c Executor) ReviewCredit(ctx context.Context, creditScore int, creditVerified bool, reviewAttempts int, faultInjection string) (creditVerifiedOut bool, err error) { // MARKER: ReviewCredit
	var out ReviewCreditOut
	err = marshalTask(ctx, _c.svc, _c.opts, _c.host, ReviewCredit.Method, ReviewCredit.Route, ReviewCreditIn{
		CreditScore:    creditScore,
		CreditVerified: creditVerified,
		ReviewAttempts: reviewAttempts,
		FaultInjection: faultInjection,
	}, &out, _c.inFlow, _c.outFlow)
	return out.CreditVerifiedOut, err // No trace
}

/*
HandleCreditError handles a credit verification error by setting creditVerified to false.
*/
func (_c Executor) HandleCreditError(ctx context.Context, onErr *errors.TracedError) (creditVerified bool, err error) { // MARKER: HandleCreditError
	var out HandleCreditErrorOut
	err = marshalTask(ctx, _c.svc, _c.opts, _c.host, HandleCreditError.Method, HandleCreditError.Route, HandleCreditErrorIn{
		OnErr: onErr,
	}, &out, _c.inFlow, _c.outFlow)
	return out.CreditVerified, err // No trace
}

/*
Decision determines whether to approve the credit application based on verification results.
*/
func (_c Executor) Decision(ctx context.Context, creditVerified bool, employmentFailures int, identityVerified bool) (approved bool, err error) { // MARKER: Decision
	var out DecisionOut
	err = marshalTask(ctx, _c.svc, _c.opts, _c.host, Decision.Method, Decision.Route, DecisionIn{
		CreditVerified:     creditVerified,
		EmploymentFailures: employmentFailures,
		IdentityVerified:   identityVerified,
	}, &out, _c.inFlow, _c.outFlow)
	return out.Approved, err // No trace
}

/*
IdentityVerification creates and runs the identity verification workflow, blocking until termination.
*/
func (_c Executor) IdentityVerification(ctx context.Context, applicantName string, ssn string, address string, phone string) (identityVerified bool, status string, err error) { // MARKER: IdentityVerification
	if _c.runner == nil {
		return false, "", errors.New("workflow runner not set, use WithWorkflowRunner")
	}
	var out IdentityVerificationOut
	status, err = marshalWorkflow(ctx, _c.runner, IdentityVerification.URL(), IdentityVerificationIn{
		ApplicantName: applicantName,
		SSN:           ssn,
		Address:       address,
		Phone:         phone,
	}, &out)
	return out.IdentityVerified, status, err
}

/*
CreditApproval creates and runs the credit approval workflow, blocking until termination.
*/
func (_c Executor) CreditApproval(ctx context.Context, applicant Applicant, faultInjection string) (approved bool, creditVerified bool, employmentFailures int, identityVerified bool, status string, err error) { // MARKER: CreditApproval
	if _c.runner == nil {
		return false, false, 0, false, "", errors.New("workflow runner not set, use WithWorkflowRunner")
	}
	var out CreditApprovalOut
	status, err = marshalWorkflow(ctx, _c.runner, CreditApproval.URL(), CreditApprovalIn{
		Applicant:      applicant,
		FaultInjection: faultInjection,
	}, &out)
	return out.Approved, out.CreditVerified, out.EmploymentFailures, out.IdentityVerified, status, err
}

// marshalTask supports task execution via the Executor.
func marshalTask(ctx context.Context, svc service.Publisher, opts []pub.Option, host string, method string, route string, in any, out any, inFlow *workflow.Flow, outFlow *workflow.Flow) (err error) {
	flow := inFlow
	if flow == nil {
		flow = workflow.NewFlow()
	}
	err = flow.SetState(in)
	if err != nil {
		return errors.Trace(err)
	}
	body, err := json.Marshal(flow)
	if err != nil {
		return errors.Trace(err)
	}
	u := httpx.JoinHostAndPath(host, route)
	httpRes, err := svc.Request(
		ctx,
		pub.Method(method),
		pub.URL(u),
		pub.Body(body),
		pub.ContentType("application/json"),
		pub.Options(opts...),
	)
	if err != nil {
		return err // No trace
	}
	err = json.NewDecoder(httpRes.Body).Decode(flow)
	if err != nil {
		return errors.Trace(err)
	}
	if outFlow != nil {
		*outFlow = *flow
	}
	if out != nil {
		err = flow.ParseState(out)
		return errors.Trace(err)
	}
	return nil
}

// marshalWorkflow supports workflow execution via the Executor.
func marshalWorkflow(ctx context.Context, runner WorkflowRunner, workflowURL string, in any, out any) (status string, err error) {
	status, state, err := runner.Run(ctx, workflowURL, in)
	if err != nil {
		return status, err // No trace
	}
	if out != nil && state != nil {
		data, err := json.Marshal(state)
		if err != nil {
			return status, errors.Trace(err)
		}
		err = json.Unmarshal(data, out)
		if err != nil {
			return status, errors.Trace(err)
		}
	}
	return status, nil
}

// marshalRequest supports functional endpoints.
func marshalRequest(ctx context.Context, svc service.Publisher, opts []pub.Option, host string, method string, route string, in any, out any) (err error) {
	if method == "ANY" {
		method = "POST"
	}
	u := httpx.JoinHostAndPath(host, route)
	query, body, err := httpx.WriteInputPayload(method, in)
	if err != nil {
		return err // No trace
	}
	httpRes, err := svc.Request(
		ctx,
		pub.Method(method),
		pub.URL(u),
		pub.Query(query),
		pub.Body(body),
		pub.Options(opts...),
	)
	if err != nil {
		return err // No trace
	}
	err = httpx.ReadOutputPayload(httpRes, out)
	return errors.Trace(err)
}

// marshalPublish supports multicast functional endpoints.
func marshalPublish(ctx context.Context, svc service.Publisher, opts []pub.Option, host string, method string, route string, in any, out any) iter.Seq[*multicastResponse] {
	if method == "ANY" {
		method = "POST"
	}
	u := httpx.JoinHostAndPath(host, route)
	query, body, err := httpx.WriteInputPayload(method, in)
	if err != nil {
		return func(yield func(*multicastResponse) bool) {
			yield(&multicastResponse{err: err})
		}
	}
	_queue := svc.Publish(
		ctx,
		pub.Method(method),
		pub.URL(u),
		pub.Query(query),
		pub.Body(body),
		pub.Options(opts...),
	)
	return func(yield func(*multicastResponse) bool) {
		for qi := range _queue {
			httpResp, err := qi.Get()
			if err == nil {
				reflect.ValueOf(out).Elem().SetZero()
				err = httpx.ReadOutputPayload(httpResp, out)
			}
			if err != nil {
				if !yield(&multicastResponse{err: err, HTTPResponse: httpResp}) {
					return
				}
			} else {
				if !yield(&multicastResponse{data: out, HTTPResponse: httpResp}) {
					return
				}
			}
		}
	}
}

/*
Demo serves the demo page for the credit approval workflow.

If a URL is provided, it is resolved relative to the URL of the endpoint.
If the body is of type io.Reader, []byte or string, it is serialized in binary form.
If it is of type url.Values, it is serialized as form data. All other types are serialized as JSON.
*/
func (_c Client) Demo(ctx context.Context, method string, relativeURL string, body any) (res *http.Response, err error) { // MARKER: Demo
	if method == "" {
		method = Demo.Method
	}
	if method == "ANY" {
		method = "POST"
	}
	return _c.svc.Request(
		ctx,
		pub.Method(method),
		pub.URL(httpx.JoinHostAndPath(_c.host, Demo.Route)),
		pub.RelativeURL(relativeURL),
		pub.Body(body),
		pub.Options(_c.opts...),
	)
}

/*
Demo serves the demo page for the credit approval workflow.

If a URL is provided, it is resolved relative to the URL of the endpoint.
If the body is of type io.Reader, []byte or string, it is serialized in binary form.
If it is of type url.Values, it is serialized as form data. All other types are serialized as JSON.
*/
func (_c MulticastClient) Demo(ctx context.Context, method string, relativeURL string, body any) iter.Seq[*pub.Response] { // MARKER: Demo
	if method == "" {
		method = Demo.Method
	}
	if method == "ANY" {
		method = "POST"
	}
	return _c.svc.Publish(
		ctx,
		pub.Method(method),
		pub.URL(httpx.JoinHostAndPath(_c.host, Demo.Route)),
		pub.RelativeURL(relativeURL),
		pub.Body(body),
		pub.Options(_c.opts...),
	)
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

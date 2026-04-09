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
	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/httpx"
)

// Hostname is the default hostname of the microservice.
const Hostname = "creditflow.example"

// Def is the routing identity of an endpoint exposed by this microservice.
type Def struct {
	Method string
	Route  string
}

// URL is the full URL of the endpoint, joined with the package-level Hostname.
func (d Def) URL() string {
	return httpx.JoinHostAndPath(Hostname, d.Route)
}

// SubmitCreditApplicationIn are the input arguments of SubmitCreditApplication.
type SubmitCreditApplicationIn struct { // MARKER: SubmitCreditApplication
	Applicant Applicant `json:"applicant,omitzero"`
}

// SubmitCreditApplicationOut are the output arguments of SubmitCreditApplication.
type SubmitCreditApplicationOut struct { // MARKER: SubmitCreditApplication
	ApplicantName string   `json:"applicantName,omitzero"`
	SSN           string   `json:"ssn,omitzero"`
	Address       string   `json:"address,omitzero"`
	Phone         string   `json:"phone,omitzero"`
	Employers     []string `json:"employers,omitzero"`
	CreditScore   int      `json:"creditScore,omitzero"`
}

// VerifyCreditIn are the input arguments of VerifyCredit.
type VerifyCreditIn struct { // MARKER: VerifyCredit
	CreditScore    int    `json:"creditScore,omitzero"`
	FaultInjection string `json:"faultInjection,omitzero"`
}

// VerifyCreditOut are the output arguments of VerifyCredit.
type VerifyCreditOut struct { // MARKER: VerifyCredit
	CreditVerified bool `json:"creditVerified,omitzero"`
}

// VerifyEmploymentIn are the input arguments of VerifyEmployment.
type VerifyEmploymentIn struct { // MARKER: VerifyEmployment
	ApplicantName string `json:"applicantName,omitzero"`
	EmployerName  string `json:"employerName,omitzero"`
}

// VerifyEmploymentOut are the output arguments of VerifyEmployment.
type VerifyEmploymentOut struct { // MARKER: VerifyEmployment
	EmploymentFailures int `json:"employmentFailures,omitzero"`
}

// InitIdentityVerificationIn are the input arguments of InitIdentityVerification.
type InitIdentityVerificationIn struct { // MARKER: InitIdentityVerification
	ApplicantName string `json:"applicantName,omitzero"`
	SSN           string `json:"ssn,omitzero"`
	Address       string `json:"address,omitzero"`
	Phone         string `json:"phone,omitzero"`
}

// InitIdentityVerificationOut are the output arguments of InitIdentityVerification.
type InitIdentityVerificationOut struct { // MARKER: InitIdentityVerification
}

// VerifySSNIn are the input arguments of VerifySSN.
type VerifySSNIn struct { // MARKER: VerifySSN
	SSN            string `json:"ssn,omitzero"`
	FaultInjection string `json:"faultInjection,omitzero"`
}

// VerifySSNOut are the output arguments of VerifySSN.
type VerifySSNOut struct { // MARKER: VerifySSN
	SsnVerified bool `json:"ssnVerified,omitzero"`
}

// VerifyAddressIn are the input arguments of VerifyAddress.
type VerifyAddressIn struct { // MARKER: VerifyAddress
	Address string `json:"address,omitzero"`
}

// VerifyAddressOut are the output arguments of VerifyAddress.
type VerifyAddressOut struct { // MARKER: VerifyAddress
	AddressVerified bool `json:"addressVerified,omitzero"`
}

// VerifyPhoneNumberIn are the input arguments of VerifyPhoneNumber.
type VerifyPhoneNumberIn struct { // MARKER: VerifyPhoneNumber
	Phone          string `json:"phone,omitzero"`
	FaultInjection string `json:"faultInjection,omitzero"`
}

// VerifyPhoneNumberOut are the output arguments of VerifyPhoneNumber.
type VerifyPhoneNumberOut struct { // MARKER: VerifyPhoneNumber
	PhoneVerified bool `json:"phoneVerified,omitzero"`
}

// IdentityDecisionIn are the input arguments of IdentityDecision.
type IdentityDecisionIn struct { // MARKER: IdentityDecision
	SsnVerified     bool `json:"ssnVerified,omitzero"`
	AddressVerified bool `json:"addressVerified,omitzero"`
	PhoneVerified   bool `json:"phoneVerified,omitzero"`
}

// IdentityDecisionOut are the output arguments of IdentityDecision.
type IdentityDecisionOut struct { // MARKER: IdentityDecision
	IdentityVerified bool `json:"identityVerified,omitzero"`
}

// RequestMoreInfoIn are the input arguments of RequestMoreInfo.
type RequestMoreInfoIn struct { // MARKER: RequestMoreInfo
	ReviewAttempts int `json:"reviewAttempts,omitzero"`
}

// RequestMoreInfoOut are the output arguments of RequestMoreInfo.
type RequestMoreInfoOut struct { // MARKER: RequestMoreInfo
	ReviewAttemptsOut int `json:"reviewAttempts,omitzero"`
}

// ReviewCreditIn are the input arguments of ReviewCredit.
type ReviewCreditIn struct { // MARKER: ReviewCredit
	CreditScore    int    `json:"creditScore,omitzero"`
	CreditVerified bool   `json:"creditVerified,omitzero"`
	ReviewAttempts int    `json:"reviewAttempts,omitzero"`
	FaultInjection string `json:"faultInjection,omitzero"`
}

// ReviewCreditOut are the output arguments of ReviewCredit.
type ReviewCreditOut struct { // MARKER: ReviewCredit
	CreditVerifiedOut bool `json:"creditVerified,omitzero"`
}

// HandleCreditErrorIn are the input arguments of HandleCreditError.
type HandleCreditErrorIn struct { // MARKER: HandleCreditError
	OnErr *errors.TracedError `json:"onErr,omitzero"`
}

// HandleCreditErrorOut are the output arguments of HandleCreditError.
type HandleCreditErrorOut struct { // MARKER: HandleCreditError
	CreditVerified bool `json:"creditVerified,omitzero"`
}

// DecisionIn are the input arguments of Decision.
type DecisionIn struct { // MARKER: Decision
	CreditVerified     bool `json:"creditVerified,omitzero"`
	EmploymentFailures int  `json:"employmentFailures,omitzero"`
	IdentityVerified   bool `json:"identityVerified,omitzero"`
}

// DecisionOut are the output arguments of Decision.
type DecisionOut struct { // MARKER: Decision
	Approved bool `json:"approved,omitzero"`
}

// IdentityVerificationIn are the input arguments of IdentityVerification.
type IdentityVerificationIn struct { // MARKER: IdentityVerification
	ApplicantName string `json:"applicantName,omitzero"`
	SSN           string `json:"ssn,omitzero"`
	Address       string `json:"address,omitzero"`
	Phone         string `json:"phone,omitzero"`
}

// IdentityVerificationOut are the output arguments of IdentityVerification.
type IdentityVerificationOut struct { // MARKER: IdentityVerification
	IdentityVerified bool `json:"identityVerified,omitzero"`
}

// CreditApprovalIn are the input arguments of CreditApproval.
type CreditApprovalIn struct { // MARKER: CreditApproval
	Applicant      Applicant `json:"applicant,omitzero"`
	FaultInjection string    `json:"faultInjection,omitzero"`
}

// CreditApprovalOut are the output arguments of CreditApproval.
type CreditApprovalOut struct { // MARKER: CreditApproval
	Approved           bool `json:"approved,omitzero"`
	CreditVerified     bool `json:"creditVerified,omitzero"`
	EmploymentFailures int  `json:"employmentFailures,omitzero"`
	IdentityVerified   bool `json:"identityVerified,omitzero"`
}

var (
	// HINT: Insert endpoint definitions here
	SubmitCreditApplication  = Def{Method: "POST", Route: ":428/submit-credit-application"}  // MARKER: SubmitCreditApplication
	VerifyCredit             = Def{Method: "POST", Route: ":428/verify-credit"}              // MARKER: VerifyCredit
	VerifyEmployment         = Def{Method: "POST", Route: ":428/verify-employment"}          // MARKER: VerifyEmployment
	InitIdentityVerification = Def{Method: "POST", Route: ":428/init-identity-verification"} // MARKER: InitIdentityVerification
	VerifySSN                = Def{Method: "POST", Route: ":428/verify-ssn"}                 // MARKER: VerifySSN
	VerifyAddress            = Def{Method: "POST", Route: ":428/verify-address"}             // MARKER: VerifyAddress
	VerifyPhoneNumber        = Def{Method: "POST", Route: ":428/verify-phone-number"}        // MARKER: VerifyPhoneNumber
	IdentityDecision         = Def{Method: "POST", Route: ":428/identity-decision"}          // MARKER: IdentityDecision
	RequestMoreInfo          = Def{Method: "POST", Route: ":428/request-more-info"}          // MARKER: RequestMoreInfo
	ReviewCredit             = Def{Method: "POST", Route: ":428/review-credit"}              // MARKER: ReviewCredit
	HandleCreditError        = Def{Method: "POST", Route: ":428/handle-credit-error"}        // MARKER: HandleCreditError
	Decision                 = Def{Method: "POST", Route: ":428/decision"}                   // MARKER: Decision
	IdentityVerification     = Def{Method: "GET", Route: ":428/identity-verification"}       // MARKER: IdentityVerification
	CreditApproval           = Def{Method: "GET", Route: ":428/credit-approval"}             // MARKER: CreditApproval
	Demo                     = Def{Method: "ANY", Route: "/demo"}                            // MARKER: Demo
)

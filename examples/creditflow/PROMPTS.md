## Create CreditFlow microservice

Create a new example microservice named CreditFlow with hostname "creditflow.example" in the examples directory. This microservice demonstrates agentic workflow features of the Microbus framework.

## Add credit approval workflow

Add a workflow with 7 task endpoints and 1 workflow graph endpoint:

- **SubmitCreditApplication** - entry point task that receives an applicant name, list of employers, and credit score
- **VerifyCredit** - verifies the applicant's credit score (passes if score >= 550)
- **VerifyEmployment** - verifies employment for a single employer (uses forEach dynamic fan-out)
- **VerifyIdentity** - verifies the applicant's identity (passes if name is non-empty)
- **ReviewCredit** - fan-in point for all verifications; passes through for good scores (650+), approves borderline (580-649), uses goto loop to RequestMoreInfo for very borderline (550-579)
- **RequestMoreInfo** - increments review attempt counter, loops back to ReviewCredit
- **Decision** - determines approval based on creditVerified, employmentFailures, identityVerified

The **CreditApproval** workflow graph exercises multiple patterns:
- Fan-out from SubmitCreditApplication to VerifyCredit, VerifyEmployment (forEach), VerifyIdentity
- Dynamic fan-out (forEach) on employers list for VerifyEmployment
- Reducer (ReducerAdd) on employmentFailures to sum across employer verifications
- Fan-in from all verifications to ReviewCredit
- Goto transition from ReviewCredit to RequestMoreInfo with loop back
- Sequential chain from ReviewCredit to Decision to END

## Add end-to-end workflow tests

Add the foreman service to TestCreditFlow_CreditApproval and add 6 end-to-end test cases that start the workflow with various initial states, wait for completion, and verify results:
- good_score_approved (score 750, 2 employers)
- low_score_rejected (score 400)
- borderline_with_review_approved (score 580)
- very_borderline_goto_loop (score 560, verifies reviewAttempts=2)
- employment_failure (one empty employer name)
- empty_applicant_rejected (empty applicant name fails identity)

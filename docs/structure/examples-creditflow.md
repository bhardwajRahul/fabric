# Package `examples/creditflow`

The creditflow example microservice demonstrates the agentic workflow features of the framework. It can only be started in the `TESTING` deployment environment and is not intended for production use.

CreditFlow implements a mock credit approval workflow that exercises the key capabilities of the workflow engine: fan-out to parallel branches, fan-in with a reducer, conditional transitions, and goto loops. The foreman's integration tests use creditflow to verify that all of these mechanisms work correctly end-to-end.

The workflow starts by accepting a credit application, then fans out to verify the applicant's credit, identity, and employment in parallel. The results converge at a review step that either approves, requests more information (looping back via goto), or proceeds to a final decision. This structure is simple enough to reason about but rich enough to cover the workflow patterns that matter.

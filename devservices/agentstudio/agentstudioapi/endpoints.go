package agentstudioapi

import (
	"github.com/microbus-io/fabric/httpx"
)

// Hostname is the default hostname of the microservice.
const Hostname = "agentstudio.dev"

// Def is the routing identity of an endpoint exposed by this microservice.
type Def struct {
	Method string
	Route  string
}

// URL is the full URL of the endpoint, joined with the package-level Hostname.
func (d Def) URL() string {
	return httpx.JoinHostAndPath(Hostname, d.Route)
}

var (
	// HINT: Insert endpoint definitions here
	ListFlows       = Def{Method: "ANY", Route: "/flows"}                       // MARKER: ListFlows
	FlowDetail      = Def{Method: "ANY", Route: "/flows/{flowKey}"}             // MARKER: FlowDetail
	StepDetail      = Def{Method: "ANY", Route: "/steps/{stepKey}"}             // MARKER: StepDetail
	ListWorkflows   = Def{Method: "ANY", Route: "/workflows"}                   // MARKER: ListWorkflows
	WorkflowDetail  = Def{Method: "ANY", Route: "/workflows/{workflowURL...}"}  // MARKER: WorkflowDetail
	RunWorkflow     = Def{Method: "ANY", Route: "/run-workflow"}                // MARKER: RunWorkflow
	ContinueFlow    = Def{Method: "ANY", Route: "/continue-flow"}               // MARKER: ContinueFlow
	ResumeFlow      = Def{Method: "ANY", Route: "/resume-flow"}                 // MARKER: ResumeFlow
	RestartFlow     = Def{Method: "ANY", Route: "/restart-flow"}                // MARKER: RestartFlow
	RestartFromStep = Def{Method: "ANY", Route: "/restart-from-step"}           // MARKER: RestartFromStep
	PollFlow        = Def{Method: "GET", Route: "/poll-flow"}                   // MARKER: PollFlow
	TaskDetail      = Def{Method: "ANY", Route: "/task-detail"}                 // MARKER: TaskDetail
	Dashboard       = Def{Method: "ANY", Route: "/dashboard"}                   // MARKER: Dashboard
	Assets          = Def{Method: "GET", Route: "//bespa/{path...}"}            // MARKER: Assets
)

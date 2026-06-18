package agentstudioapi

import (
	"github.com/microbus-io/fabric/define"
)

// Hostname is the default hostname of the microservice.
const Hostname = "agentstudio.dev"

// Name is the decorative PascalCase name of the microservice.
const Name = ""

// Version is the major version of the microservice's public API.
const Version = 7

// Description is the human-readable summary of the microservice, surfaced in OpenAPI and discovery.
const Description = `AgentStudio is a developer console for inspecting flows running under the Foreman.`

// ListFlows renders an HTML page with a paginated, sortable table of flows.
var ListFlows = define.Web{
	Host: Hostname, Method: "ANY", Route: "/flows",
}

// FlowDetail renders an HTML page with the details, DAG diagram, and step log of a flow.
var FlowDetail = define.Web{
	Host: Hostname, Method: "ANY", Route: "/flows/{flowKey}",
}

// StepDetail renders an HTML page with the details of one execution step.
var StepDetail = define.Web{
	Host: Hostname, Method: "ANY", Route: "/steps/{stepKey}",
}

// ListWorkflows renders an HTML page listing the workflows available in the system.
var ListWorkflows = define.Web{
	Host: Hostname, Method: "ANY", Route: "/workflows",
}

// WorkflowDetail renders an HTML page with the structure and definition of a single workflow graph.
var WorkflowDetail = define.Web{
	Host: Hostname, Method: "ANY", Route: "/workflows/{workflowURL...}",
}

// RunWorkflow renders a form to create and start a workflow with an initial state and FlowOptions.
var RunWorkflow = define.Web{
	Host: Hostname, Method: "ANY", Route: "/run-workflow",
}

// ContinueFlow renders a form to continue a completed flow's thread with additional state.
var ContinueFlow = define.Web{
	Host: Hostname, Method: "ANY", Route: "/continue-flow",
}

// ResumeFlow renders a form to resume an interrupted flow with a resume payload.
var ResumeFlow = define.Web{
	Host: Hostname, Method: "ANY", Route: "/resume-flow",
}

// RestartFlow renders a form to restart a terminated flow from its entry step with optional state overrides.
var RestartFlow = define.Web{
	Host: Hostname, Method: "ANY", Route: "/restart-flow",
}

// RestartFromStep renders a form to restart a flow from a specific step with optional state overrides.
var RestartFromStep = define.Web{
	Host: Hostname, Method: "ANY", Route: "/restart-from-step",
}

// PollFlow returns a JSON status payload driving the FlowDetail live-update progress bar.
var PollFlow = define.Web{
	Host: Hostname, Method: "GET", Route: "/poll-flow",
}

// TaskDetail renders an HTML page with the metadata of a single task in a workflow graph.
var TaskDetail = define.Web{
	Host: Hostname, Method: "ANY", Route: "/task-detail",
}

// Dashboard renders an HTML page with operator dashboards for flows and workflows.
var Dashboard = define.Web{
	Host: Hostname, Method: "ANY", Route: "/dashboard",
}

// Assets serves the bespa CSS and JavaScript assets at /bespa/.
var Assets = define.Web{
	Host: Hostname, Method: "GET", Route: "//bespa/{path...}",
}

package interruptflowapi

import (
	"github.com/microbus-io/fabric/httpx"
)

// Hostname is the default hostname of the microservice.
const Hostname = "interruptflow.verify"

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
	TaskA       = Def{Method: "POST", Route: ":428/task-a"}      // MARKER: TaskA
	AwaitInput  = Def{Method: "POST", Route: ":428/await-input"} // MARKER: AwaitInput
	Compose     = Def{Method: "POST", Route: ":428/compose"}     // MARKER: Compose
	Interruptor = Def{Method: "GET", Route: ":428/interruptor"}  // MARKER: Interruptor
)

// TaskAIn are the input arguments of TaskA.
type TaskAIn struct { // MARKER: TaskA
	Prompt string `json:"prompt,omitzero"`
}

// TaskAOut are the output arguments of TaskA.
type TaskAOut struct { // MARKER: TaskA
	PromptOut string `json:"prompt,omitzero"`
}

// AwaitInputIn are the input arguments of AwaitInput.
type AwaitInputIn struct { // MARKER: AwaitInput
	UserInput string `json:"userInput,omitzero"`
}

// AwaitInputOut are the output arguments of AwaitInput.
type AwaitInputOut struct { // MARKER: AwaitInput
	UserInputOut string `json:"userInput,omitzero"`
}

// ComposeIn are the input arguments of Compose.
type ComposeIn struct { // MARKER: Compose
	Prompt    string `json:"prompt,omitzero"`
	UserInput string `json:"userInput,omitzero"`
}

// ComposeOut are the output arguments of Compose.
type ComposeOut struct { // MARKER: Compose
	Result string `json:"result,omitzero"`
}

// InterruptorIn are the input arguments of Interruptor.
type InterruptorIn struct { // MARKER: Interruptor
	Prompt string `json:"prompt,omitzero"`
}

// InterruptorOut are the output arguments of Interruptor.
type InterruptorOut struct { // MARKER: Interruptor
	Result string `json:"result,omitzero"`
}

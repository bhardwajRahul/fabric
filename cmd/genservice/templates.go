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

package main

import _ "embed"

// Fixed client.go pieces, kept as Go source in templates/*.txt and embedded here. emitClient writes
// each one selectively, based on the service's feature mix.

//go:embed templates/multicastresponse.txt
var pcMulticastResponse string

//go:embed templates/client.txt
var pcClient string

//go:embed templates/multicastclient.txt
var pcMulticastClient string

//go:embed templates/workflowrunner.txt
var pcWorkflowRunner string

//go:embed templates/executor.txt
var pcExecutor string

//go:embed templates/subflow.txt
var pcSubflow string

//go:embed templates/multicasttrigger.txt
var pcMulticastTrigger string

//go:embed templates/hook.txt
var pcHook string

//go:embed templates/marshalrequest.txt
var hMarshalRequest string

//go:embed templates/marshalpublish.txt
var hMarshalPublish string

//go:embed templates/marshalfunction.txt
var hMarshalFunction string

//go:embed templates/marshaltask.txt
var hMarshalTask string

//go:embed templates/marshalworkflow.txt
var hMarshalWorkflow string

//go:embed templates/marshalsubflow.txt
var hMarshalSubflow string

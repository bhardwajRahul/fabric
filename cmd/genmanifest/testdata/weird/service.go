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

package weird

import (
	"context"

	"github.com/microbus-io/errors"

	"github.com/microbus-io/fabric/cmd/genmanifest/testdata/weird/weirdapi"
)

var (
	_ context.Context
	_ errors.TracedError
	_ weirdapi.Client
)

/*
Service implements weird, a fixture for genmanifest exercising every Def route shape.
*/
type Service struct {
	*Intermediate // IMPORTANT: Do not remove
}

// OnStartup is called when the microservice is started up.
func (svc *Service) OnStartup(ctx context.Context) (err error) {
	return nil
}

// OnShutdown is called when the microservice is shut down.
func (svc *Service) OnShutdown(ctx context.Context) (err error) {
	return nil
}

// Plain handles the Plain endpoint.
func (svc *Service) Plain(ctx context.Context) (result string, err error) { // MARKER: Plain
	return "", nil
}

// PathArg handles the PathArg endpoint.
func (svc *Service) PathArg(ctx context.Context, id string) (err error) { // MARKER: PathArg
	return nil
}

// GreedyArg handles the GreedyArg endpoint.
func (svc *Service) GreedyArg(ctx context.Context, tail string) (err error) { // MARKER: GreedyArg
	return nil
}

// PeriodInPath handles the PeriodInPath endpoint.
func (svc *Service) PeriodInPath(ctx context.Context) (err error) { // MARKER: PeriodInPath
	return nil
}

// AnyMethod handles the AnyMethod endpoint.
func (svc *Service) AnyMethod(ctx context.Context) (err error) { // MARKER: AnyMethod
	return nil
}

// InternalPort handles the InternalPort endpoint.
func (svc *Service) InternalPort(ctx context.Context) (err error) { // MARKER: InternalPort
	return nil
}

// TrustRoot handles the trust-root :666 endpoint.
func (svc *Service) TrustRoot(ctx context.Context, cmd string) (err error) { // MARKER: TrustRoot
	return nil
}

// SlashHostRoot handles the //root endpoint.
func (svc *Service) SlashHostRoot(ctx context.Context) (err error) { // MARKER: SlashHostRoot
	return nil
}

// SlashHostPort handles the //alt.host:0/alt-path endpoint.
func (svc *Service) SlashHostPort(ctx context.Context) (err error) { // MARKER: SlashHostPort
	return nil
}

// SlashHostPathArg handles the //alt.host/items/{id} endpoint.
func (svc *Service) SlashHostPathArg(ctx context.Context, id string) (err error) { // MARKER: SlashHostPathArg
	return nil
}

// SpecialHostRoute handles the //my$.xml/lookup endpoint - exercises hostname encoding for URL specials.
func (svc *Service) SpecialHostRoute(ctx context.Context) (err error) { // MARKER: SpecialHostRoute
	return nil
}

// UpperCasePath handles the :443/UPPERCASE.xml endpoint - exercises case-preserving path encoding.
func (svc *Service) UpperCasePath(ctx context.Context) (err error) { // MARKER: UpperCasePath
	return nil
}

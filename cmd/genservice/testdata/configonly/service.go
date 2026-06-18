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

package configonly

import "context"

// Service is the hand-written half of the config-only fixture; intermediate.go is generated alongside it.
type Service struct {
	*Intermediate // IMPORTANT: Do not remove
}

// OnStartup is called when the microservice is started up.
func (svc *Service) OnStartup(ctx context.Context) (err error) { return nil }

// OnShutdown is called when the microservice is shut down.
func (svc *Service) OnShutdown(ctx context.Context) (err error) { return nil }

// OnChangedDenyList is the callback fired when the DenyList config changes.
func (svc *Service) OnChangedDenyList(ctx context.Context) (err error) { return nil }

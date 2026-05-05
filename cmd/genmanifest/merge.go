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

import (
	"fmt"
	"os"
	"time"

	"go.yaml.in/yaml/v3"
)

// existingManifest is a permissive shape used for reading the previous
// manifest.yaml to extract operator-curated fields. Unknown keys are tolerated.
type existingManifest struct {
	General struct {
		Name             string `yaml:"name,omitempty"`
		Hostname         string `yaml:"hostname,omitempty"`
		Description      string `yaml:"description,omitempty"`
		Package          string `yaml:"package,omitempty"`
		FrameworkVersion string `yaml:"frameworkVersion,omitempty"`
		DB               string `yaml:"db,omitempty"`
		Cloud            string `yaml:"cloud,omitempty"`
		ModifiedAt       string `yaml:"modifiedAt,omitempty"`
	} `yaml:"general,omitempty"`
}

// readExisting reads manifest.yaml if present. Returns the parsed permissive
// shape, the raw bytes (for byte-level comparisons during --check), and any
// fatal I/O error. Missing files are not an error.
func readExisting(path string) (*existingManifest, []byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &existingManifest{}, nil, nil
		}
		return nil, nil, err
	}
	var m existingManifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &m, data, nil
}

// merge combines the extracted data with the existing manifest's preserved
// fields and produces the final Manifest to be emitted.
func merge(x *extracted, existing *existingManifest, now time.Time) *Manifest {
	m := &Manifest{}
	m.General.Hostname = x.hostname
	m.General.Description = x.description
	m.General.Package = existing.General.Package
	// Preserve the operator-curated PascalCase decoration of the service name.
	m.General.Name = existing.General.Name
	m.General.FrameworkVersion = existing.General.FrameworkVersion
	m.General.ModifiedAt = now.Format(time.RFC3339)

	m.Configs = x.configs
	m.Metrics = x.metrics
	m.OutboundEvents = x.outboundEvents
	m.Functions = x.functions
	m.Webs = x.webs
	m.InboundEvents = x.inboundEvents
	m.Tasks = x.tasks
	m.Workflows = x.workflows
	m.Tickers = x.tickers

	return m
}

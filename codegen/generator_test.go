/*
Copyright (c) 2023-2025 Microbus LLC and various contributors

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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/microbus-io/fabric/rand"
	"github.com/microbus-io/testarossa"
)

func TestCodeGen_FullGeneration(t *testing.T) {
	// No parallel
	tt := testarossa.For(t)

	serviceYaml := `
general:
  host: test.full.generation
  description: Testing full generation.
configs:
  - signature: Config1() (b bool)
    description: Config1 is a bool.
    default: true
    validation: bool
    callback: true
  - signature: Config2() (dur Duration)
    description: Config2 is a duration.
    default: 2m
    validation: dur [1m,5m]
functions:
  - signature: Func1(x Type1, y Type1) (result float)
    description: Func1 returns the distance between two points.
    path: :1234/distance
  - signature: Func2(term string) (count int)
    description: Func2 is a distributed search.
    path: :1234/count-occurrences
    queue: none
    actor: roles.x || roles.y
events:
  - signature: OnEvent1(x Type1) (ok bool)
    description: OnEvent1 is fired before deletion.
    path: :1234/event1
  - signature: OnEvent2(x Type2)
    description: OnEvent2 is fired after deletion.
    path: :1234/event2
sinks:
  - signature: OnSink1(x Type1) (ok bool)
    description: OnSink1 handles an event.
    event: OnSink
    source: from/somewhere/else
    forHost: event.source
  - signature: OnSink2(x Type2)
    description: OnSink2 handles an event.
    source: from/somewhere/else
webs:
  - signature: Web1()
  - signature: Web2()
    queue: none
tickers:
  - signature: Ticker1()
    description: Ticker1 runs once a minute.
    interval: 1m
    timeBudget: 30s
  - signature: Ticker2()
    description: Ticker1 runs once an hour.
    interval: 1h
metrics:
  - signature: Metric1(measure time.Duration, label1 string, label2 int, label3 bool)
    description: Metric1 measures time [second]
    kind: counter
  - signature: Metric2(measure int, label1 string, label2 int, label3 bool)
    description: Metric2 measures bytes [bytes]
    kind: gauge
    callback: true
`

	// Create a temp directory with the service.yaml file
	dir := "testing" + rand.AlphaNum32(12)
	os.Mkdir(dir, os.ModePerm)
	defer os.RemoveAll(dir)
	err := os.WriteFile(filepath.Join(dir, "service.yaml"), []byte(serviceYaml), 0666)
	tt.NoError(err)

	// Generate
	gen := NewGenerator()
	gen.WorkDir = dir
	err = gen.Run()
	tt.NoError(err)

	// Validate
	fileContains := func(fileName string, terms ...string) {
		b, err := os.ReadFile(filepath.Join(dir, fileName))
		tt.NoError(err, "%s", fileName)
		body := string(b)
		for _, term := range terms {
			tt.Contains(body, term, "%s does not contain '%s'", fileName, term)
		}
	}

	fileContains(filepath.Join("app", dir, "main-gen.go"),
		"func main", dir+".NewService()",
	)
	fileContains(filepath.Join(dir+"api", "clients-gen.go"),
		"Func1(ctx", "Func2(ctx", "Web1(ctx", "Web2(ctx", "OnEvent1(ctx", "OnEvent2(ctx", "OnEvent1(handler", "OnEvent2(handler",
	)
	fileContains(filepath.Join(dir+"api", "type1.go"),
		"type Type1 struct",
	)
	fileContains(filepath.Join(dir+"api", "type2.go"),
		"type Type2 struct",
	)
	fileContains(filepath.Join("intermediate", "intermediate-gen.go"),
		"svc.Subscribe(",
		"svc.impl.Func1", "svc.impl.Func1",
		") doFunc1(w", ") doFunc2(w",
		"svc.impl.OnSink1", "svc.impl.OnSink2",
		"svc.impl.Web1", "svc.impl.Web2",
		"svc.StartTicker(",
		"svc.impl.Ticker1", "svc.impl.Ticker2",
		"svc.DefineConfig(",
		"svc.impl.OnChangedConfig1",
		") Config1() (b bool)", ") Config2() (dur time.Duration)",
		") SetConfig1(b bool)", ") SetConfig2(dur time.Duration)",
		"roles.x || roles.y",
		":1234/count-occurrences",
		":1234/distance",
		"svc.DescribeCounter",
		"svc.DescribeGauge",
		"fabric_"+strings.ToLower(dir)+"_metric_1",
		"fabric_"+strings.ToLower(dir)+"_metric_2",
		"OnObserveMetric2(ctx context.Context",
		"AddMetric1",
		"RecordMetric2",
	)
	fileContains(filepath.Join("resources", "embed-gen.go"),
		"go:embed",
	)
	fileContains("service.go",
		") Func1(ctx", ") Func2(ctx",
		") Web1(w", ") Web2(w",
		") Ticker1(ctx", ") Ticker2(ctx",
		") OnChangedConfig1(ctx",
		") OnObserveMetric2(ctx",
	)
	fileContains("version-gen.go",
		"Version", "SourceCodeSHA256",
	)
	fileContains("version-gen_test.go",
		"SourceCodeSHA256",
	)

	ver, err := gen.currentVersion()
	tt.NoError(err)
	tt.Equal(1, ver.Version)

	// Second run should be a no op
	err = gen.Run()
	tt.NoError(err)

	ver, err = gen.currentVersion()
	tt.NoError(err)
	tt.Equal(1, ver.Version)
}

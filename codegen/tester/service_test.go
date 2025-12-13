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

package tester

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/application"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/fabric/pub"
	"github.com/microbus-io/testarossa"

	"github.com/microbus-io/fabric/codegen/tester/testerapi"
)

// valueAtPath traverses the OpenAPI document and returns the value at the indicated path.
// The path uses the pipe as the separator character.
func valueAtPath(openAPI map[string]any, path string) any {
	if openAPI == nil {
		return nil
	}
	var at any
	at = openAPI
	parts := strings.Split(path, "|")
	for i := range parts {
		var next any
		if m, ok := at.(map[string]any); ok {
			next = m[parts[i]]
		}
		if a, ok := at.([]any); ok {
			i, _ := strconv.Atoi(parts[i])
			next = a[i]
		}
		if i == len(parts)-1 {
			return next
		}
		if next == nil {
			return nil
		}
		at = next
	}
	return nil
}

func TestTester_StringCut(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("tester.stringcut.tester")
	client := testerapi.NewClient(tester)

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	t.Run("test_cases", func(t *testing.T) {
		assert := testarossa.For(t)

		before, after, found, err := client.StringCut(ctx, "Hello World", " ")
		assert.Expect(
			before, "Hello",
			after, "World",
			found, true,
			err, nil,
		)
		before, after, found, err = client.StringCut(ctx, "Hello World", "X")
		assert.Expect(
			before, "Hello World",
			after, "",
			found, false,
			err, nil,
		)
	})

	t.Run("requests", func(t *testing.T) {
		assert := testarossa.For(t)

		res, err := tester.Request(ctx, pub.GET("https://"+Hostname+"/string-cut?s=Foo+Bar&Sep=+"))
		if assert.NoError(err) {
			var out testerapi.StringCutOut
			err = json.NewDecoder(res.Body).Decode(&out)
			assert.Expect(out.Before, "Foo", out.After, "Bar", out.Found, true, err, nil)
		}
	})

	t.Run("open_api", func(t *testing.T) {
		assert := testarossa.For(t)

		var openAPI map[string]any
		res, err := tester.Request(ctx, pub.GET("https://"+Hostname+"/openapi.json"))
		if assert.NoError(err) {
			err = json.NewDecoder(res.Body).Decode(&openAPI)
			assert.NoError(err)
		}

		basePath := "paths|/" + Hostname + ":443/string-cut|post|"
		// Input arguments
		schemaRef := valueAtPath(openAPI, basePath+"requestBody|content|application/json|schema|$ref").(string)
		schemaRef = strings.ReplaceAll(schemaRef, "/", "|")[2:] + "|"
		assert.Equal("string", valueAtPath(openAPI, schemaRef+"properties|s|type"))
		assert.Equal("string", valueAtPath(openAPI, schemaRef+"properties|sep|type"))
		// Output argument
		assert.NotNil(valueAtPath(openAPI, basePath+"responses|2XX"))
		schemaRef = valueAtPath(openAPI, basePath+"responses|2XX|content|application/json|schema|$ref").(string)
		schemaRef = strings.ReplaceAll(schemaRef, "/", "|")[2:] + "|"
		assert.Equal("object", valueAtPath(openAPI, schemaRef+"type"))
		assert.Equal("string", valueAtPath(openAPI, schemaRef+"properties|before|type"))
		assert.Equal("string", valueAtPath(openAPI, schemaRef+"properties|after|type"))
		assert.Equal("boolean", valueAtPath(openAPI, schemaRef+"properties|found|type"))
		// Error
		assert.NotNil(valueAtPath(openAPI, basePath+"responses|4XX"))
		schemaRef = valueAtPath(openAPI, basePath+"responses|4XX|content|application/json|schema|$ref").(string)
		schemaRef = strings.ReplaceAll(schemaRef, "/", "|")[2:] + "|"
		schemaRef = valueAtPath(openAPI, schemaRef+"properties|err|$ref").(string)
		schemaRef = strings.ReplaceAll(schemaRef, "/", "|")[2:] + "|"
		assert.Equal("string", valueAtPath(openAPI, schemaRef+"properties|error|type"))
		assert.Equal("integer", valueAtPath(openAPI, schemaRef+"properties|statusCode|type"))
		assert.Equal("array", valueAtPath(openAPI, schemaRef+"properties|stack|type"))
		assert.NotNil(valueAtPath(openAPI, basePath+"responses|5XX"))
		schemaRef = valueAtPath(openAPI, basePath+"responses|5XX|content|application/json|schema|$ref").(string)
		schemaRef = strings.ReplaceAll(schemaRef, "/", "|")[2:] + "|"
		schemaRef = valueAtPath(openAPI, schemaRef+"properties|err|$ref").(string)
		schemaRef = strings.ReplaceAll(schemaRef, "/", "|")[2:] + "|"
		assert.Equal("string", valueAtPath(openAPI, schemaRef+"properties|error|type"))
		assert.Equal("integer", valueAtPath(openAPI, schemaRef+"properties|statusCode|type"))
		assert.Equal("array", valueAtPath(openAPI, schemaRef+"properties|stack|type"))
	})
}

func TestTester_PointDistance(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("tester.pointdistance.tester")
	client := testerapi.NewClient(tester)

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	t.Run("test_cases", func(t *testing.T) {
		assert := testarossa.For(t)

		d, err := client.PointDistance(ctx, testerapi.XYCoord{X: 1, Y: 1}, &testerapi.XYCoord{X: 4, Y: 5})
		assert.Expect(
			d, 5.0,
			err, nil,
		)
		d, err = client.PointDistance(ctx, testerapi.XYCoord{X: 4, Y: 5}, &testerapi.XYCoord{X: 1, Y: 1})
		assert.Expect(
			d, 5.0,
			err, nil,
		)
		d, err = client.PointDistance(ctx, testerapi.XYCoord{X: 1.5, Y: 1.6}, &testerapi.XYCoord{X: 2.5, Y: 2.6})
		if assert.NoError(err) {
			assert.True(d >= 1.414-.01 && d <= 1.414+.01) // sqrt(2) ≈ 1.414
		}
		d, err = client.PointDistance(ctx, testerapi.XYCoord{X: 6.1, Y: 7.6}, &testerapi.XYCoord{X: 6.1, Y: 7.6})
		assert.Expect(
			d, 0.0,
			err, nil,
		)
	})

	t.Run("requests", func(t *testing.T) {
		assert := testarossa.For(t)

		res, err := tester.Request(ctx, pub.GET("https://"+Hostname+"/point-distance?p1.x=1&p1.y=1&p2.x=4&p2.y=5"))
		if assert.NoError(err) {
			var out testerapi.PointDistanceOut
			err = json.NewDecoder(res.Body).Decode(&out)
			assert.Expect(out.D, 5.0, err, nil)
		}
		_, err = tester.Request(ctx, pub.POST("https://"+Hostname+"/point-distance?p1.x=1&p1.y=1&p2.x=4&p2.y=5"))
		assert.Error(err)
	})

	t.Run("open_api", func(t *testing.T) {
		assert := testarossa.For(t)

		var openAPI map[string]any
		res, err := tester.Request(ctx, pub.GET("https://"+Hostname+"/openapi.json"))
		if assert.NoError(err) {
			err = json.NewDecoder(res.Body).Decode(&openAPI)
			assert.NoError(err)
		}

		basePath := "paths|/" + Hostname + ":443/point-distance|get|"
		// Input argument p1 refers to XYCoord with its own x and y
		assert.Equal("p1", valueAtPath(openAPI, basePath+"parameters|0|name"))
		assert.Equal("query", valueAtPath(openAPI, basePath+"parameters|0|in"))
		schemaRef := valueAtPath(openAPI, basePath+"parameters|0|schema|$ref").(string)
		schemaRef = strings.ReplaceAll(schemaRef, "/", "|")[2:] + "|"
		assert.Equal("object", valueAtPath(openAPI, schemaRef+"type"))
		assert.Equal("number", valueAtPath(openAPI, schemaRef+"properties|x|type"))
		assert.Equal("number", valueAtPath(openAPI, schemaRef+"properties|y|type"))
		// Input argument p2 refers to XYCoord with its own x and y
		assert.Equal("p2", valueAtPath(openAPI, basePath+"parameters|1|name"))
		assert.Equal("query", valueAtPath(openAPI, basePath+"parameters|1|in"))
		schemaRef = valueAtPath(openAPI, basePath+"parameters|1|schema|$ref").(string)
		schemaRef = strings.ReplaceAll(schemaRef, "/", "|")[2:] + "|"
		assert.Equal("object", valueAtPath(openAPI, schemaRef+"type"))
		assert.Equal("number", valueAtPath(openAPI, schemaRef+"properties|x|type"))
		assert.Equal("number", valueAtPath(openAPI, schemaRef+"properties|y|type"))
		// Output argument d is a number
		schemaRef = valueAtPath(openAPI, basePath+"responses|2XX|content|application/json|schema|$ref").(string)
		schemaRef = strings.ReplaceAll(schemaRef, "/", "|")[2:] + "|"
		assert.Equal("object", valueAtPath(openAPI, schemaRef+"type"))
		assert.Equal("number", valueAtPath(openAPI, schemaRef+"properties|d|type"))
	})
}

func TestTester_ShiftPoint(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("tester.shiftpoint.tester")
	client := testerapi.NewClient(tester)

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	t.Run("test_cases", func(t *testing.T) {
		assert := testarossa.For(t)

		shifted, err := client.ShiftPoint(ctx, &testerapi.XYCoord{X: 5, Y: 6}, 3, 4)
		assert.Expect(
			shifted, &testerapi.XYCoord{X: 8, Y: 10},
			err, nil,
		)
		shifted, err = client.ShiftPoint(ctx, &testerapi.XYCoord{X: 5, Y: 6}, -5, -6)
		assert.Expect(
			shifted, &testerapi.XYCoord{X: 0, Y: 0},
			err, nil,
		)
	})

	t.Run("requests", func(t *testing.T) {
		assert := testarossa.For(t)

		res, err := tester.Request(ctx,
			pub.POST("https://"+Hostname+"/shift-point?x=10&y=10"),
			pub.Body(testerapi.ShiftPointIn{
				P: &testerapi.XYCoord{
					X: 5,
					Y: 6,
				},
			}))
		if assert.NoError(err) {
			var out testerapi.ShiftPointOut
			err = json.NewDecoder(res.Body).Decode(&out)
			assert.Expect(out.Shifted.X, 15.0, out.Shifted.Y, 16.0, err, nil)
		}
		res, err = tester.Request(ctx, pub.GET("https://"+Hostname+"/shift-point?x=10&y=10&p.x=5&p.y=6"))
		if assert.NoError(err) {
			var out testerapi.ShiftPointOut
			err = json.NewDecoder(res.Body).Decode(&out)
			assert.Expect(out.Shifted.X, 15.0, out.Shifted.Y, 16.0, err, nil)
		}
	})

	t.Run("open_api", func(t *testing.T) {
		assert := testarossa.For(t)

		var openAPI map[string]any
		res, err := tester.Request(ctx, pub.GET("https://"+Hostname+"/openapi.json"))
		if assert.NoError(err) {
			err = json.NewDecoder(res.Body).Decode(&openAPI)
			assert.NoError(err)
		}

		basePath := "paths|/" + Hostname + ":443/shift-point|post|"
		// Input arguments x and y are numbers
		schemaRef := valueAtPath(openAPI, basePath+"requestBody|content|application/json|schema|$ref").(string)
		schemaRef = strings.ReplaceAll(schemaRef, "/", "|")[2:] + "|"
		assert.Equal("number", valueAtPath(openAPI, schemaRef+"properties|x|type"))
		assert.Equal("number", valueAtPath(openAPI, schemaRef+"properties|y|type"))
		// Input argument p refers to XYCoord with its own x and y
		schemaRef = valueAtPath(openAPI, schemaRef+"properties|p|$ref").(string)
		schemaRef = strings.ReplaceAll(schemaRef, "/", "|")[2:] + "|"
		assert.Equal("number", valueAtPath(openAPI, schemaRef+"properties|x|type"))
		assert.Equal("number", valueAtPath(openAPI, schemaRef+"properties|y|type"))
		// Output argument shifted also refers to XYCoord
		schemaRef = valueAtPath(openAPI, basePath+"responses|2XX|content|application/json|schema|$ref").(string)
		schemaRef = strings.ReplaceAll(schemaRef, "/", "|")[2:] + "|"
		schemaRef = valueAtPath(openAPI, schemaRef+"properties|shifted|$ref").(string)
		schemaRef = strings.ReplaceAll(schemaRef, "/", "|")[2:] + "|"
		assert.Equal("object", valueAtPath(openAPI, schemaRef+"type"))
		assert.Equal("number", valueAtPath(openAPI, schemaRef+"properties|x|type"))
		assert.Equal("number", valueAtPath(openAPI, schemaRef+"properties|y|type"))
	})
}

func TestTester_LinesIntersection(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("tester.linesintersection.tester")
	client := testerapi.NewClient(tester)

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	t.Run("test_cases", func(t *testing.T) {
		assert := testarossa.For(t)

		b, err := client.LinesIntersection(ctx,
			testerapi.XYLine{
				Start: testerapi.XYCoord{X: 1, Y: 1},
				End:   testerapi.XYCoord{X: 10, Y: 1},
			}, &testerapi.XYLine{
				Start: testerapi.XYCoord{X: 1, Y: 2},
				End:   testerapi.XYCoord{X: 10, Y: 2},
			})
		assert.Expect(b, false, err, nil)

		b, err = client.LinesIntersection(ctx,
			testerapi.XYLine{
				Start: testerapi.XYCoord{X: 10, Y: 1},
				End:   testerapi.XYCoord{X: 0, Y: 10},
			}, &testerapi.XYLine{
				Start: testerapi.XYCoord{X: 0, Y: 0},
				End:   testerapi.XYCoord{X: 10, Y: 10},
			})
		assert.Expect(b, true, err, nil)

		b, err = client.LinesIntersection(ctx,
			testerapi.XYLine{
				Start: testerapi.XYCoord{X: -5, Y: -5},
				End:   testerapi.XYCoord{X: 0, Y: 0},
			}, &testerapi.XYLine{
				Start: testerapi.XYCoord{X: 1, Y: 1},
				End:   testerapi.XYCoord{X: 10, Y: 10},
			})
		assert.Expect(b, false, err, nil)
	})

	t.Run("requests", func(t *testing.T) {
		assert := testarossa.For(t)

		res, err := tester.Request(ctx,
			pub.POST("https://"+Hostname+"/lines-intersection"),
			pub.Body(testerapi.LinesIntersectionIn{
				L1: testerapi.XYLine{
					Start: testerapi.XYCoord{X: 10, Y: 1},
					End:   testerapi.XYCoord{X: 0, Y: 10},
				},
				L2: &testerapi.XYLine{
					Start: testerapi.XYCoord{X: 0, Y: 0},
					End:   testerapi.XYCoord{X: 10, Y: 10},
				},
			}))
		if assert.NoError(err) {
			var out testerapi.LinesIntersectionOut
			err = json.NewDecoder(res.Body).Decode(&out)
			assert.Expect(out.B, true, err, nil)
		}
	})

	t.Run("open_api", func(t *testing.T) {
		assert := testarossa.For(t)

		var openAPI map[string]any
		res, err := tester.Request(ctx, pub.GET("https://"+Hostname+"/openapi.json"))
		if assert.NoError(err) {
			err = json.NewDecoder(res.Body).Decode(&openAPI)
			assert.NoError(err)
		}

		basePath := "paths|/" + Hostname + ":443/lines-intersection|post|"
		// Input arguments l1 and l2 are lines
		schemaRef := valueAtPath(openAPI, basePath+"requestBody|content|application/json|schema|$ref").(string)
		schemaRef = strings.ReplaceAll(schemaRef, "/", "|")[2:] + "|"
		l1SchemaRef := valueAtPath(openAPI, schemaRef+"properties|l1|$ref").(string)
		l1SchemaRef = strings.ReplaceAll(l1SchemaRef, "/", "|")[2:] + "|"
		startSchemaRef := valueAtPath(openAPI, l1SchemaRef+"properties|start|$ref").(string)
		startSchemaRef = strings.ReplaceAll(startSchemaRef, "/", "|")[2:] + "|"
		assert.Equal("number", valueAtPath(openAPI, startSchemaRef+"properties|x|type"))
		assert.Equal("number", valueAtPath(openAPI, startSchemaRef+"properties|y|type"))
		endSchemaRef := valueAtPath(openAPI, l1SchemaRef+"properties|end|$ref").(string)
		endSchemaRef = strings.ReplaceAll(endSchemaRef, "/", "|")[2:] + "|"
		assert.Equal("number", valueAtPath(openAPI, endSchemaRef+"properties|x|type"))
		assert.Equal("number", valueAtPath(openAPI, endSchemaRef+"properties|y|type"))

		l2SchemaRef := valueAtPath(openAPI, schemaRef+"properties|l2|$ref").(string)
		l2SchemaRef = strings.ReplaceAll(l2SchemaRef, "/", "|")[2:] + "|"
		startSchemaRef = valueAtPath(openAPI, l2SchemaRef+"properties|start|$ref").(string)
		startSchemaRef = strings.ReplaceAll(startSchemaRef, "/", "|")[2:] + "|"
		assert.Equal("number", valueAtPath(openAPI, startSchemaRef+"properties|x|type"))
		assert.Equal("number", valueAtPath(openAPI, startSchemaRef+"properties|y|type"))
		endSchemaRef = valueAtPath(openAPI, l2SchemaRef+"properties|end|$ref").(string)
		endSchemaRef = strings.ReplaceAll(endSchemaRef, "/", "|")[2:] + "|"
		assert.Equal("number", valueAtPath(openAPI, endSchemaRef+"properties|x|type"))
		assert.Equal("number", valueAtPath(openAPI, endSchemaRef+"properties|y|type"))

		// Output argument is a boolean
		schemaRef = valueAtPath(openAPI, basePath+"responses|2XX|content|application/json|schema|$ref").(string)
		schemaRef = strings.ReplaceAll(schemaRef, "/", "|")[2:] + "|"
		assert.Equal("boolean", valueAtPath(openAPI, schemaRef+"properties|b|type"))
	})
}

func TestTester_EchoAnything(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("tester.echoanything.tester")
	client := testerapi.NewClient(tester)

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	t.Run("test_cases", func(t *testing.T) {
		assert := testarossa.For(t)

		echoed, err := client.EchoAnything(ctx, "string")
		assert.Expect(echoed, "string", err, nil)

		echoed, err = client.EchoAnything(ctx, 5.0)
		assert.Expect(echoed, 5.0, err, nil)

		echoed, err = client.EchoAnything(ctx, nil)
		assert.Expect(echoed, nil, err, nil)

		echoed, err = client.EchoAnything(ctx, testerapi.XYCoord{X: 5, Y: 6})
		assert.Expect(echoed, map[string]any{"x": 5.0, "y": 6.0}, err, nil)
	})

	t.Run("requests", func(t *testing.T) {
		assert := testarossa.For(t)

		res, err := tester.Request(ctx, pub.POST("https://"+Hostname+"/echo-anything"), pub.Body(struct {
			Original testerapi.XYCoord `json:"original"`
		}{
			testerapi.XYCoord{X: 1, Y: 2},
		}))
		if assert.NoError(err) {
			var out struct {
				Echoed testerapi.XYCoord `json:"echoed"`
			}
			err = json.NewDecoder(res.Body).Decode(&out)
			assert.Expect(out.Echoed.X, 1.0, out.Echoed.Y, 2.0, err, nil)
		}
		res, err = tester.Request(ctx, pub.POST("https://"+Hostname+"/echo-anything?original=hello"))
		if assert.NoError(err) {
			var out struct {
				Echoed string `json:"echoed"`
			}
			err = json.NewDecoder(res.Body).Decode(&out)
			assert.Expect(out.Echoed, "hello", err, nil)
		}
	})

	t.Run("open_api", func(t *testing.T) {
		assert := testarossa.For(t)

		var openAPI map[string]any
		res, err := tester.Request(ctx, pub.GET("https://"+Hostname+"/openapi.json"))
		if assert.NoError(err) {
			err = json.NewDecoder(res.Body).Decode(&openAPI)
			assert.NoError(err)
		}

		basePath := "paths|/" + Hostname + ":443/echo-anything|post|"
		// Input argument should exist but have no type
		schemaRef := valueAtPath(openAPI, basePath+"requestBody|content|application/json|schema|$ref").(string)
		schemaRef = strings.ReplaceAll(schemaRef, "/", "|")[2:] + "|"
		assert.NotNil(valueAtPath(openAPI, schemaRef+"properties|original"))
		assert.Nil(valueAtPath(openAPI, schemaRef+"properties|original|type"))
		// Output argument should exist but have no type
		schemaRef = valueAtPath(openAPI, basePath+"responses|2XX|content|application/json|schema|$ref").(string)
		schemaRef = strings.ReplaceAll(schemaRef, "/", "|")[2:] + "|"
		assert.NotNil(valueAtPath(openAPI, schemaRef+"properties|echoed"))
		assert.Nil(valueAtPath(openAPI, schemaRef+"properties|echoed|type"))
	})
}

func TestTester_SubArrayRange(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("tester.subarrayrange.tester")
	client := testerapi.NewClient(tester)

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	t.Run("test_cases", func(t *testing.T) {
		assert := testarossa.For(t)

		sub, status, err := client.SubArrayRange(ctx, []int{1, 2, 3, 4, 5, 6}, 2, 4)
		assert.Expect(
			sub, []int{2, 3, 4},
			status, 202, // http.StatusAccepted
			err, nil,
		)
	})

	t.Run("open_api", func(t *testing.T) {
		assert := testarossa.For(t)

		var openAPI map[string]any
		res, err := tester.Request(ctx, pub.GET("https://"+Hostname+"/openapi.json"))
		if assert.NoError(err) {
			err = json.NewDecoder(res.Body).Decode(&openAPI)
			assert.NoError(err)
		}

		basePath := "paths|/" + Hostname + ":443/sub-array-range/{max}|post|"
		// Argument pushed to query because of httpRequestBody
		assert.Equal("min", valueAtPath(openAPI, basePath+"parameters|0|name"))
		assert.Equal("query", valueAtPath(openAPI, basePath+"parameters|0|in"))
		// Argument indicated in path
		assert.Equal("max", valueAtPath(openAPI, basePath+"parameters|1|name"))
		assert.Equal("path", valueAtPath(openAPI, basePath+"parameters|1|in"))
		// httpRequestBody should not be listed as an argument
		assert.Len(valueAtPath(openAPI, basePath+"parameters").([]any), 2)
		// Request schema is an array
		schemaRef := valueAtPath(openAPI, basePath+"requestBody|content|application/json|schema|$ref").(string)
		schemaRef = strings.ReplaceAll(schemaRef, "/", "|")[2:] + "|"
		assert.Equal("array", valueAtPath(openAPI, schemaRef+"type"))
		assert.Equal("integer", valueAtPath(openAPI, schemaRef+"items|type"))
		// Response schema is an array
		schemaRef = valueAtPath(openAPI, basePath+"responses|2XX|content|application/json|schema|$ref").(string)
		schemaRef = strings.ReplaceAll(schemaRef, "/", "|")[2:] + "|"
		assert.Equal("array", valueAtPath(openAPI, schemaRef+"type"))
		assert.Equal("integer", valueAtPath(openAPI, schemaRef+"items|type"))
	})
}

func TestTester_SumTwoIntegers(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("tester.sumtwointegers.tester")
	client := testerapi.NewClient(tester)

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	t.Run("test_cases", func(t *testing.T) {
		assert := testarossa.For(t)

		sum, httpStatusCode, err := client.SumTwoIntegers(ctx, 5, 6)
		assert.Expect(
			sum, 11,
			httpStatusCode, 202, // http.StatusAccepted
			err, nil,
		)
		sum, httpStatusCode, err = client.SumTwoIntegers(ctx, 5, -6)
		assert.Expect(
			sum, -1,
			httpStatusCode, 406, // http.StatusNotAcceptable
			err, nil,
		)
	})

	t.Run("requests", func(t *testing.T) {
		assert := testarossa.For(t)

		res, err := tester.Request(ctx, pub.GET("https://"+Hostname+"/sum-two-integers?x=73&y=83"))
		if assert.NoError(err) {
			// The status code is not returned in the body but only through the status code field of the response
			assert.Equal(202, res.StatusCode) // http.StatusAccepted
			body, _ := io.ReadAll(res.Body)
			assert.Contains(body, "156")
			assert.NotContains(string(body), "httpStatusCode")
			assert.NotContains(string(body), "202")
		}
	})
}

func TestTester_FunctionPathArguments(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("tester.functionpatharguments.tester")
	client := testerapi.NewClient(tester)

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	t.Run("test_cases", func(t *testing.T) {
		assert := testarossa.For(t)

		joined, err := client.FunctionPathArguments(ctx, "1", "2", "3/4")
		assert.Expect(joined, "1 2 3/4", err, nil)

		joined, err = client.FunctionPathArguments(ctx, "", "", "")
		assert.Expect(joined, "  ", err, nil)

		joined, err = client.FunctionPathArguments(ctx, "[a&b$c]", "[d&e$f]", "[g&h/i]")
		assert.Expect(joined, "[a&b$c] [d&e$f] [g&h/i]", err, nil)
	})

	t.Run("requests", func(t *testing.T) {
		assert := testarossa.For(t)

		res, err := tester.Request(ctx, pub.GET("https://"+Hostname+"/function-path-arguments/fixed/1/2/3/4"))
		if assert.NoError(err) {
			var out testerapi.FunctionPathArgumentsOut
			err = json.NewDecoder(res.Body).Decode(&out)
			assert.Expect(out.Joined, "1 2 3/4", err, nil)
		}
		res, err = tester.Request(ctx, pub.GET("https://"+Hostname+"/function-path-arguments/fixed///"))
		if assert.NoError(err) {
			var out testerapi.FunctionPathArgumentsOut
			err = json.NewDecoder(res.Body).Decode(&out)
			assert.Expect(out.Joined, "  ", err, nil)
		}
		res, err = tester.Request(ctx, pub.GET("https://"+Hostname+"/function-path-arguments/fixed/[a&b$c]/[d&e$f]/[g&h/i]"))
		if assert.NoError(err) {
			var out testerapi.FunctionPathArgumentsOut
			err = json.NewDecoder(res.Body).Decode(&out)
			assert.Expect(out.Joined, "[a&b$c] [d&e$f] [g&h/i]", err, nil)
		}
	})

	t.Run("open_api", func(t *testing.T) {
		assert := testarossa.For(t)

		var openAPI map[string]any
		res, err := tester.Request(ctx, pub.GET("https://"+Hostname+"/openapi.json"))
		if assert.NoError(err) {
			err = json.NewDecoder(res.Body).Decode(&openAPI)
			assert.NoError(err)
		}

		basePath := "paths|/" + Hostname + ":443/function-path-arguments/fixed/{named}/{path2}/{suffix+}|get|"
		// named
		assert.Equal("named", valueAtPath(openAPI, basePath+"parameters|0|name"))
		assert.Equal("path", valueAtPath(openAPI, basePath+"parameters|0|in"))
		assert.Equal("string", valueAtPath(openAPI, basePath+"parameters|0|schema|type"))
		// path2
		assert.Equal("path2", valueAtPath(openAPI, basePath+"parameters|1|name"))
		assert.Equal("path", valueAtPath(openAPI, basePath+"parameters|1|in"))
		assert.Equal("string", valueAtPath(openAPI, basePath+"parameters|1|schema|type"))
		// suffix
		assert.Equal("suffix+", valueAtPath(openAPI, basePath+"parameters|2|name"))
		assert.Equal("path", valueAtPath(openAPI, basePath+"parameters|2|in"))
		assert.Equal("string", valueAtPath(openAPI, basePath+"parameters|2|schema|type"))
		// Response
		schemaRef := valueAtPath(openAPI, basePath+"responses|2XX|content|application/json|schema|$ref").(string)
		schemaRef = strings.ReplaceAll(schemaRef, "/", "|")[2:] + "|"
		assert.Equal("string", valueAtPath(openAPI, schemaRef+"properties|joined|type"))
	})
}

func TestTester_NonStringPathArguments(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("tester.nonstringpatharguments.tester")
	client := testerapi.NewClient(tester)

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	t.Run("test_cases", func(t *testing.T) {
		assert := testarossa.For(t)

		joined, err := client.NonStringPathArguments(ctx, 1, true, 0.75)
		assert.Expect(joined, "1 true 0.75", err, nil)
	})

	t.Run("requests", func(t *testing.T) {
		assert := testarossa.For(t)

		_, err := tester.Request(ctx, pub.GET("https://"+Hostname+"/non-string-path-arguments/fixed/1.5/true/0.75"))
		assert.Contains(err, "json")
		_, err = tester.Request(ctx, pub.GET("https://"+Hostname+"/non-string-path-arguments/fixed/1/x/0.75"))
		assert.Contains(err, "invalid character")
		_, err = tester.Request(ctx, pub.GET("https://"+Hostname+"/non-string-path-arguments/fixed/1/true/x"))
		assert.Contains(err, "invalid character")
		_, err = tester.Request(ctx, pub.GET("https://"+Hostname+"/non-string-path-arguments/fixed/1/true/0.75"))
		assert.NoError(err)
	})

	t.Run("open_api", func(t *testing.T) {
		assert := testarossa.For(t)

		var openAPI map[string]any
		res, err := tester.Request(ctx, pub.GET("https://"+Hostname+"/openapi.json"))
		if assert.NoError(err) {
			err = json.NewDecoder(res.Body).Decode(&openAPI)
			assert.NoError(err)
		}

		basePath := "paths|/" + Hostname + ":443/non-string-path-arguments/fixed/{named}/{path2}/{suffix+}|get|"
		// named
		assert.Equal("named", valueAtPath(openAPI, basePath+"parameters|0|name"))
		assert.Equal("path", valueAtPath(openAPI, basePath+"parameters|0|in"))
		assert.Equal("integer", valueAtPath(openAPI, basePath+"parameters|0|schema|type"))
		// path2
		assert.Equal("path2", valueAtPath(openAPI, basePath+"parameters|1|name"))
		assert.Equal("path", valueAtPath(openAPI, basePath+"parameters|1|in"))
		assert.Equal("boolean", valueAtPath(openAPI, basePath+"parameters|1|schema|type"))
		// suffix
		assert.Equal("suffix+", valueAtPath(openAPI, basePath+"parameters|2|name"))
		assert.Equal("path", valueAtPath(openAPI, basePath+"parameters|2|in"))
		assert.Equal("number", valueAtPath(openAPI, basePath+"parameters|2|schema|type"))
		// Response
		schemaRef := valueAtPath(openAPI, basePath+"responses|2XX|content|application/json|schema|$ref").(string)
		schemaRef = strings.ReplaceAll(schemaRef, "/", "|")[2:] + "|"
		assert.Equal("string", valueAtPath(openAPI, schemaRef+"properties|joined|type"))
	})
}

func TestTester_UnnamedFunctionPathArguments(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("tester.unnamedfunctionpatharguments.tester")

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	t.Run("requests", func(t *testing.T) {
		assert := testarossa.For(t)

		res, err := tester.Request(ctx, pub.GET("https://"+Hostname+"/unnamed-function-path-arguments/x123/foo/y345/bar/z1/z2/z3"))
		assert.NoError(err)
		body, _ := io.ReadAll(res.Body)
		assert.Contains(body, "x123 y345 z1/z2/z3")
	})

	t.Run("open_api", func(t *testing.T) {
		assert := testarossa.For(t)

		var openAPI map[string]any
		res, err := tester.Request(ctx, pub.GET("https://"+Hostname+"/openapi.json"))
		if assert.NoError(err) {
			err = json.NewDecoder(res.Body).Decode(&openAPI)
			assert.NoError(err)
		}

		basePath := "paths|/" + Hostname + ":443/unnamed-function-path-arguments/{path1}/foo/{path2}/bar/{path3+}|get|"
		assert.Equal("path1", valueAtPath(openAPI, basePath+"parameters|0|name"))
		assert.Equal("path", valueAtPath(openAPI, basePath+"parameters|0|in"))
		assert.Equal("path2", valueAtPath(openAPI, basePath+"parameters|1|name"))
		assert.Equal("path", valueAtPath(openAPI, basePath+"parameters|1|in"))
		assert.Equal("path3+", valueAtPath(openAPI, basePath+"parameters|2|name"))
		assert.Equal("path", valueAtPath(openAPI, basePath+"parameters|2|in"))
	})
}

func TestTester_PathArgumentsPriority(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("tester.pathargumentspriority.tester")
	client := testerapi.NewClient(tester)

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	t.Run("test_cases", func(t *testing.T) {
		assert := testarossa.For(t)

		echo, err := client.PathArgumentsPriority(ctx, "BAR")
		assert.Expect(echo, "BAR", err, nil)

		echo, err = client.PathArgumentsPriority(ctx, "XYZ")
		assert.Expect(echo, "XYZ", err, nil)
	})

	t.Run("requests", func(t *testing.T) {
		assert := testarossa.For(t)

		// Argument in the query should take priority over that in the path
		res, err := tester.Request(ctx, pub.GET("https://"+Hostname+"/path-arguments-priority/BAR?foo=XYZ"))
		if assert.NoError(err) {
			b, _ := io.ReadAll(res.Body)
			assert.NotContains(string(b), "BAR")
			assert.Contains(string(b), "XYZ")
		}

		// If argument is not provided in the path, take from the query
		res, err = tester.Request(ctx, pub.GET("https://"+Hostname+"/path-arguments-priority/{foo}?foo=BAR"))
		if assert.NoError(err) {
			b, _ := io.ReadAll(res.Body)
			assert.Contains(string(b), "BAR")
		}

		// Argument in the body should take priority over that in the path
		res, err = tester.Request(ctx, pub.POST("https://"+Hostname+"/path-arguments-priority/BAR"), pub.Body(`{"foo":"XYZ"}`))
		if assert.NoError(err) {
			b, _ := io.ReadAll(res.Body)
			assert.NotContains(string(b), "BAR")
			assert.Contains(string(b), "XYZ")
		}

		// If argument is not provided in the path, take from the body
		res, err = tester.Request(ctx, pub.POST("https://"+Hostname+"/path-arguments-priority/{foo}"), pub.Body(`{"foo":"BAR"}`))
		if assert.NoError(err) {
			b, _ := io.ReadAll(res.Body)
			assert.Contains(string(b), "BAR")
		}

		// Argument in the query should take priority over that in the body
		res, err = tester.Request(ctx, pub.POST("https://"+Hostname+"/path-arguments-priority/ABC?foo=BAR"), pub.Body(`{"foo":"XYZ"}`))
		if assert.NoError(err) {
			b, _ := io.ReadAll(res.Body)
			assert.Contains(string(b), "BAR")
			assert.NotContains(string(b), "XYZ")
			assert.NotContains(string(b), "ABC")
		}
	})
}

func TestTester_WhatTimeIsIt(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("tester.whattimeisit.tester")
	client := testerapi.NewClient(tester)

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	t.Run("test_cases", func(t *testing.T) {
		assert := testarossa.For(t)
		realNow := time.Now()

		approxTime := func(actual, expected time.Time) {
			assert.True(!actual.Before(expected.Add(-time.Second)) && !actual.After(expected.Add(time.Second)))
		}

		// Test with no clock shift
		tm, err := client.WhatTimeIsIt(ctx)
		if assert.NoError(err) {
			approxTime(tm, realNow)
		}

		// Test with clock shift - create new context
		testCtx := frame.CloneContext(ctx)
		frame.Of(testCtx).SetClockShift(time.Hour)
		tm, err = client.WhatTimeIsIt(testCtx)
		if assert.NoError(err) {
			approxTime(tm, realNow.Add(time.Hour))
		}
	})

	t.Run("requests", func(t *testing.T) {
		assert := testarossa.For(t)
		realNow := time.Now()

		approxTime := func(actual, expected time.Time) {
			assert.True(!actual.Before(expected.Add(-time.Second)) && !actual.After(expected.Add(time.Second)))
		}

		res, err := tester.Request(ctx, pub.GET("https://"+Hostname+"/what-time-is-it"))
		if assert.NoError(err) {
			var out testerapi.WhatTimeIsItOut
			err = json.NewDecoder(res.Body).Decode(&out)
			if assert.NoError(err) {
				approxTime(out.T, realNow)
			}
		}

		shiftedCtx := frame.CloneContext(ctx)
		frame.Of(shiftedCtx).SetClockShift(time.Hour)
		res, err = tester.Request(shiftedCtx, pub.GET("https://"+Hostname+"/what-time-is-it"))
		if assert.NoError(err) {
			var out testerapi.WhatTimeIsItOut
			err = json.NewDecoder(res.Body).Decode(&out)
			if assert.NoError(err) {
				approxTime(out.T, realNow.Add(time.Hour))
			}
		}
	})

	t.Run("open_api", func(t *testing.T) {
		assert := testarossa.For(t)

		var openAPI map[string]any
		res, err := tester.Request(ctx, pub.GET("https://"+Hostname+"/openapi.json"))
		if assert.NoError(err) {
			err = json.NewDecoder(res.Body).Decode(&openAPI)
			assert.NoError(err)
		}

		basePath := "paths|/" + Hostname + ":443/what-time-is-it|post|"
		// Output argument
		schemaRef := valueAtPath(openAPI, basePath+"responses|2XX|content|application/json|schema|$ref").(string)
		schemaRef = strings.ReplaceAll(schemaRef, "/", "|")[2:] + "|"
		assert.Equal("object", valueAtPath(openAPI, schemaRef+"type"))
		assert.Equal("string", valueAtPath(openAPI, schemaRef+"properties|t|type"))
		assert.Equal("date-time", valueAtPath(openAPI, schemaRef+"properties|t|format"))
	})
}

func TestTester_AuthzRequired(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("tester.authzrequired.tester")
	client := testerapi.NewClient(tester)

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	t.Run("test_cases", func(t *testing.T) {
		assert := testarossa.For(t)

		// Without auth token
		err := client.AuthzRequired(ctx)
		if assert.Error(err) {
			assert.Equal(http.StatusUnauthorized, errors.StatusCode(err))
		}

		// With insufficient role
		err = client.WithOptions(pub.Actor(jwt.MapClaims{"roles": "d"})).AuthzRequired(ctx)
		if assert.Error(err) {
			assert.Equal(http.StatusForbidden, errors.StatusCode(err))
		}

		// With sufficient roles
		err = client.WithOptions(pub.Actor(jwt.MapClaims{"roles": "ac"})).AuthzRequired(ctx)
		assert.NoError(err)

		err = client.WithOptions(pub.Actor(jwt.MapClaims{"scopes": "r"})).AuthzRequired(ctx)
		assert.NoError(err)
	})

	t.Run("open_api", func(t *testing.T) {
		assert := testarossa.For(t)

		var openAPI map[string]any
		res, err := tester.Request(ctx, pub.GET("https://"+Hostname+"/openapi.json"))
		if assert.NoError(err) {
			err = json.NewDecoder(res.Body).Decode(&openAPI)
			assert.NoError(err)
		}

		basePath := "paths|/" + Hostname + ":443/authz-required|post|"
		assert.Equal([]any{}, valueAtPath(openAPI, basePath+"security|0|http_bearer_jwt"))
		assert.Contains(valueAtPath(openAPI, basePath+"responses|403|description").(string), `roles=~"(a|b|c)" || scopes=~"r"`)
		securitySchemePath := "components|securitySchemes|http_bearer_jwt|"
		assert.Equal("http", valueAtPath(openAPI, securitySchemePath+"type"))
		assert.Equal("bearer", valueAtPath(openAPI, securitySchemePath+"scheme"))
		assert.Equal("JWT", valueAtPath(openAPI, securitySchemePath+"bearerFormat"))
	})
}

func TestTester_OnDiscovered(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("tester.ondiscovered.tester")
	trigger := testerapi.NewMulticastTrigger(tester)
	hook := testerapi.NewHook(tester)

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	t.Run("test_cases", func(t *testing.T) {
		assert := testarossa.For(t)

		hook.OnDiscovered(func(ctx context.Context, p testerapi.XYCoord, n int) (q testerapi.XYCoord, m int, err error) {
			assert.Expect(
				p, testerapi.XYCoord{X: 5, Y: -6},
				n, 3,
			)
			return testerapi.XYCoord{X: 5, Y: -6}, 4, nil
		})
		defer hook.OnDiscovered(nil)

		for e := range trigger.OnDiscovered(ctx, testerapi.XYCoord{X: 5, Y: -6}, 3) {
			q, m, err := e.Get()
			assert.Expect(
				q, testerapi.XYCoord{X: 5, Y: -6},
				m, 4,
				err, nil,
			)
		}
	})

	t.Run("requests", func(t *testing.T) {
		assert := testarossa.For(t)

		hook.OnDiscovered(func(ctx context.Context, p testerapi.XYCoord, n int) (q testerapi.XYCoord, m int, err error) {
			assert.Expect(
				p, testerapi.XYCoord{X: 5, Y: -6},
				n, -3,
			)
			return testerapi.XYCoord{X: -5, Y: 6}, -2, nil
		})
		defer hook.OnDiscovered(nil)

		res := <-tester.Publish(ctx, pub.PATCH("https://"+Hostname+":417/on-discovered"), pub.Body(&testerapi.OnDiscoveredIn{
			P: testerapi.XYCoord{X: 5, Y: -6},
			N: -3,
		}))
		assert.Nil(res) // Wrong HTTP method

		res = <-tester.Publish(ctx, pub.POST("https://"+Hostname+":417/on-discovered"), pub.Body(&testerapi.OnDiscoveredIn{
			P: testerapi.XYCoord{X: 5, Y: -6},
			N: -3,
		}))
		httpRes, err := res.Get()
		if assert.NoError(err) {
			var out testerapi.OnDiscoveredOut
			err = json.NewDecoder(httpRes.Body).Decode(&out)
			if assert.NoError(err) {
				assert.Equal(testerapi.XYCoord{X: -5, Y: 6}, out.Q)
				assert.Equal(-2, out.M)
			}
		}
	})
}

func TestTester_OnDiscoveredSink(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("tester.ondiscoveredsink.tester")
	trigger := testerapi.NewMulticastTrigger(tester)
	_ = trigger

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	t.Run("test_cases", func(t *testing.T) {
		assert := testarossa.For(t)

		for e := range trigger.OnDiscovered(ctx, testerapi.XYCoord{X: 5, Y: -6}, -2) {
			q, m, err := e.Get()
			if frame.Of(e.HTTPResponse).FromHost() == svc.Hostname() {
				assert.Expect(
					q, testerapi.XYCoord{X: -5, Y: 6},
					m, -1,
					err, nil,
				)
			}
		}

		for e := range trigger.OnDiscovered(ctx, testerapi.XYCoord{X: 5, Y: -6}, 3) {
			q, m, err := e.Get()
			if frame.Of(e.HTTPResponse).FromHost() == svc.Hostname() {
				assert.Expect(
					q, testerapi.XYCoord{X: 5, Y: -6},
					m, 4,
					err, nil,
				)
			}
		}

		for e := range trigger.OnDiscovered(ctx, testerapi.XYCoord{X: 5, Y: -6}, 0) {
			_, _, err := e.Get()
			if frame.Of(e.HTTPResponse).FromHost() == svc.Hostname() {
				assert.Contains(err, "zero")
			}
		}
	})
}

func TestTester_Echo(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("tester.echo.tester")
	client := testerapi.NewClient(tester)

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	t.Run("requests", func(t *testing.T) {
		assert := testarossa.For(t)

		res, err := tester.Request(ctx,
			pub.PATCH("https://"+Hostname+"/echo?alpha=111&beta=222"),
			pub.Body("HEAVY PAYLOAD"),
			pub.ContentType("text/plain"),
		)
		if assert.NoError(err) {
			body, err := io.ReadAll(res.Body)
			if assert.NoError(err) {
				assert.Contains(body, "PATCH /")
				assert.Contains(body, "alpha=111&beta=222")
				assert.Contains(body, "Content-Type: text/plain")
				assert.Contains(body, "HEAVY PAYLOAD")
			}
		}

		res, err = client.
			WithOptions(
				pub.Header("Foo", "Bar"),
			).
			Echo(ctx, "PATCH", "https://"+Hostname+"/echo", "", strings.NewReader("Lorem ipsum"))
		if assert.NoError(err) {
			body, _ := io.ReadAll(res.Body)
			assert.Contains(body, "PATCH /")
			assert.Contains(body, "Lorem ipsum")
			assert.Contains(body, "Foo: Bar")
		}

		res, err = client.
			WithOptions(
				pub.Method("POST"),
				pub.Body("Dolor sit amet"),
				pub.Header("Foo", "Baz"),
			).
			Echo(ctx, "PATCH", "https://"+Hostname+"/echo", "", strings.NewReader("Lorem ipsum"))
		if assert.NoError(err) {
			body, _ := io.ReadAll(res.Body)
			assert.NotContains(string(body), "PATCH /")
			assert.NotContains(string(body), "Lorem ipsum")
			assert.NotContains(string(body), "Foo: Bar")
			assert.Contains(body, "POST /")
			assert.Contains(body, "Dolor sit amet")
			assert.Contains(body, "Foo: Baz")
		}
	})

	t.Run("open_api", func(t *testing.T) {
		assert := testarossa.For(t)

		var openAPI map[string]any
		res, err := tester.Request(ctx, pub.GET("https://"+Hostname+"/openapi.json"))
		if assert.NoError(err) {
			err = json.NewDecoder(res.Body).Decode(&openAPI)
			assert.NoError(err)
		}

		basePath := "paths|/" + Hostname + ":443/echo|get|"
		assert.NotNil(valueAtPath(openAPI, basePath+"responses|2XX"))
		assert.NotNil(valueAtPath(openAPI, basePath+"responses|4XX"))
		assert.NotNil(valueAtPath(openAPI, basePath+"responses|5XX"))
	})
}

func TestTester_MultiValueHeaders(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("tester.multivalueheaders.tester")
	client := testerapi.NewClient(tester).WithOptions(
		pub.AddHeader("Multi-In", "In1"),
		pub.AddHeader("Multi-In", "In2"),
	)

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	t.Run("test_cases", func(t *testing.T) {
		assert := testarossa.For(t)

		res, err := client.MultiValueHeaders(ctx, "GET", "", "", nil)
		if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
			assert.Len(res.Header["Multi-Out"], 2)
		}

		res, err = client.MultiValueHeaders(ctx, "POST", "", "text/plain", "Payload")
		if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
			assert.Len(res.Header["Multi-Out"], 2)
		}
	})
}

func TestTester_WebPathArguments(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("tester.webpatharguments.tester")
	client := testerapi.NewClient(tester)

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	t.Run("test_cases", func(t *testing.T) {
		assert := testarossa.For(t)

		res, err := client.WebPathArguments(ctx, "GET", "?named=1&path2=2&suffix=3/4", "", nil)
		if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if assert.NoError(err) {
				assert.Contains(body, "/fixed/1/2/3/4$")
				assert.NotContains(body, "?")
				assert.NotContains(body, "{")
				assert.NotContains(body, "}")
			}
		}

		res, err = client.WebPathArguments(ctx, "GET", "?named=1&path2=2&suffix=3/4&q=5", "", nil)
		if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if assert.NoError(err) {
				assert.Contains(body, "/fixed/1/2/3/4?q=5$")
				assert.NotContains(body, "&")
				assert.NotContains(body, "{")
				assert.NotContains(body, "}")
			}
		}

		res, err = client.WebPathArguments(ctx, "GET", "", "", nil)
		if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if assert.NoError(err) {
				assert.Contains(body, "/fixed///$")
				assert.NotContains(body, "?")
				assert.NotContains(body, "&")
				assert.NotContains(body, "{")
				assert.NotContains(body, "}")
			}
		}

		res, err = client.WebPathArguments(ctx, "GET", "?named="+url.QueryEscape("[a&b/c]")+"&path2="+url.QueryEscape("[d&e/f]")+"&suffix="+url.QueryEscape("[g&h/i]")+"&q="+url.QueryEscape("[j&k/l]"), "", nil)
		if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			if assert.NoError(err) {
				assert.Contains(body, "/fixed/"+url.PathEscape("[a&b/c]")+"/"+url.PathEscape("[d&e/f]")+"/"+url.PathEscape("[g&h")+"/"+url.PathEscape("i]")+"?q="+url.QueryEscape("[j&k/l]"))
				assert.NotContains(body, "{")
				assert.NotContains(body, "}")
			}
		}
	})

	t.Run("open_api", func(t *testing.T) {
		assert := testarossa.For(t)

		var openAPI map[string]any
		res, err := tester.Request(ctx, pub.GET("https://"+Hostname+"/openapi.json"))
		if assert.NoError(err) {
			err = json.NewDecoder(res.Body).Decode(&openAPI)
			assert.NoError(err)
		}

		basePath := "paths|/" + Hostname + ":443/web-path-arguments/fixed/{named}/{path2}/{suffix+}|get|"
		// named
		assert.Equal("named", valueAtPath(openAPI, basePath+"parameters|0|name"))
		assert.Equal("path", valueAtPath(openAPI, basePath+"parameters|0|in"))
		assert.Equal("string", valueAtPath(openAPI, basePath+"parameters|0|schema|type"))
		// path2
		assert.Equal("path2", valueAtPath(openAPI, basePath+"parameters|1|name"))
		assert.Equal("path", valueAtPath(openAPI, basePath+"parameters|1|in"))
		assert.Equal("string", valueAtPath(openAPI, basePath+"parameters|1|schema|type"))
		// suffix
		assert.Equal("suffix+", valueAtPath(openAPI, basePath+"parameters|2|name"))
		assert.Equal("path", valueAtPath(openAPI, basePath+"parameters|2|in"))
		assert.Equal("string", valueAtPath(openAPI, basePath+"parameters|2|schema|type"))
	})
}

func TestTester_UnnamedWebPathArguments(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("tester.unnamedwebpatharguments.tester")

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	t.Run("requests", func(t *testing.T) {
		assert := testarossa.For(t)

		res, err := tester.Request(ctx, pub.GET("https://"+Hostname+"/unnamed-web-path-arguments/x123/foo/y345/bar/z1/z2/z3"))
		if assert.NoError(err) {
			body, err := io.ReadAll(res.Body)
			if assert.NoError(err) {
				assert.Contains(body, "x123 y345 z1/z2/z3")
			}
		}
	})

	t.Run("open_api", func(t *testing.T) {
		assert := testarossa.For(t)

		var openAPI map[string]any
		res, err := tester.Request(ctx, pub.GET("https://"+Hostname+"/openapi.json"))
		if assert.NoError(err) {
			err = json.NewDecoder(res.Body).Decode(&openAPI)
			assert.NoError(err)
		}

		basePath := "paths|/" + Hostname + ":443/unnamed-web-path-arguments/{path1}/foo/{path2}/bar/{path3+}|get|"
		assert.Equal("path1", valueAtPath(openAPI, basePath+"parameters|0|name"))
		assert.Equal("path", valueAtPath(openAPI, basePath+"parameters|0|in"))
		assert.Equal("path2", valueAtPath(openAPI, basePath+"parameters|1|name"))
		assert.Equal("path", valueAtPath(openAPI, basePath+"parameters|1|in"))
		assert.Equal("path3+", valueAtPath(openAPI, basePath+"parameters|2|name"))
		assert.Equal("path", valueAtPath(openAPI, basePath+"parameters|2|in"))
	})
}

func TestTester_DirectoryServer(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("tester.directoryserver.tester")
	client := testerapi.NewClient(tester)

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	t.Run("test_cases", func(t *testing.T) {
		assert := testarossa.For(t)

		// Test accessing files with different path formats
		res, err := client.DirectoryServer(ctx, "1.txt")
		if assert.NoError(err) {
			b, err := io.ReadAll(res.Body)
			if assert.NoError(err) {
				assert.Contains(b, "111")
			}
		}

		res, err = client.DirectoryServer(ctx, "sub/2.txt")
		if assert.NoError(err) {
			b, err := io.ReadAll(res.Body)
			if assert.NoError(err) {
				assert.Contains(b, "222")
			}
		}

		// Test path traversal protection
		_, err = client.DirectoryServer(ctx, "../3.txt")
		assert.Error(err)
	})

	t.Run("requests", func(t *testing.T) {
		assert := testarossa.For(t)

		// Test various file paths
		res, err := tester.Request(ctx, pub.GET("https://"+Hostname+"/directory-server/1.txt"))
		if assert.NoError(err) {
			b, err := io.ReadAll(res.Body)
			if assert.NoError(err) {
				assert.Contains(b, "111")
			}
		}

		res, err = tester.Request(ctx, pub.GET("https://"+Hostname+"/directory-server/sub/2.txt"))
		if assert.NoError(err) {
			b, err := io.ReadAll(res.Body)
			if assert.NoError(err) {
				assert.Contains(b, "222")
			}
		}

		// Test path traversal protection
		_, err = tester.Request(ctx, pub.GET("https://"+Hostname+"/directory-server/../3.txt"))
		assert.Error(err)

		// Test wrong HTTP method
		_, err = tester.Request(ctx, pub.POST("https://"+Hostname+"/directory-server/1.txt"))
		assert.Error(err)
	})

	t.Run("open_api", func(t *testing.T) {
		assert := testarossa.For(t)

		var openAPI map[string]any
		res, err := tester.Request(ctx, pub.GET("https://"+Hostname+"/openapi.json"))
		if assert.NoError(err) {
			err = json.NewDecoder(res.Body).Decode(&openAPI)
			assert.NoError(err)
		}

		basePath := "paths|/" + Hostname + ":443/directory-server/{filename+}|get|"
		assert.Equal("filename+", valueAtPath(openAPI, basePath+"parameters|0|name"))
		assert.Equal("path", valueAtPath(openAPI, basePath+"parameters|0|in"))
	})
}

func TestTester_Hello(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("tester.hello.tester")
	client := testerapi.NewClient(tester)

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	t.Run("set_language_with_option", func(t *testing.T) {
		assert := testarossa.For(t)

		res, err := client.WithOptions(pub.Languages("en")).Hello(ctx, "GET", "", "", nil)
		if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			assert.Expect(string(body), "Hello", err, nil)
		}

		res, err = client.WithOptions(pub.Languages("en-NZ")).Hello(ctx, "GET", "", "", nil)
		if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			assert.Expect(string(body), "Hello", err, nil)
		}

		res, err = client.WithOptions(pub.Languages("it")).Hello(ctx, "GET", "", "", nil)
		if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			assert.Expect(string(body), "Salve", err, nil)
		}
	})

	t.Run("set_language_with_ctx", func(t *testing.T) {
		assert := testarossa.For(t)

		ctx := frame.CloneContext(ctx)

		res, err := client.Hello(ctx, "GET", "", "", nil)
		if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			assert.Expect(string(body), "Hello", err, nil)
		}

		frame.Of(ctx).SetLanguages("it")
		res, err = client.Hello(ctx, "GET", "", "", nil)
		if assert.NoError(err) && assert.Expect(res.StatusCode, http.StatusOK) {
			body, err := io.ReadAll(res.Body)
			assert.Expect(string(body), "Salve", err, nil)
		}
	})

	t.Run("requests", func(t *testing.T) {
		assert := testarossa.For(t)

		ctx := frame.CloneContext(ctx)

		res, err := tester.Request(ctx, pub.GET("https://"+Hostname+"/hello"))
		if assert.NoError(err) {
			body, err := io.ReadAll(res.Body)
			assert.Expect(string(body), "Hello", err, nil)
		}

		res, err = tester.Request(ctx, pub.GET("https://"+Hostname+"/hello"), pub.Header("Accept-Language", "it"))
		if assert.NoError(err) {
			body, err := io.ReadAll(res.Body)
			assert.Expect(string(body), "Salve", err, nil)
		}

		frame.Of(ctx).SetLanguages("it")
		res, err = tester.Request(ctx, pub.GET("https://"+Hostname+"/hello"))
		if assert.NoError(err) {
			body, err := io.ReadAll(res.Body)
			assert.Expect(string(body), "Salve", err, nil)
		}
	})
}

func TestTester_OnceAMinute(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	assert := testarossa.For(t)

	// Initialize the microservice under test
	svc := NewService()

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
	)
	app.RunInTest(t)

	err := svc.OnceAMinute(ctx)
	assert.NoError(err)
}

func TestTester_OnObserveMemoryAvailable(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	assert := testarossa.For(t)

	// Initialize the microservice under test
	svc := NewService()

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
	)
	app.RunInTest(t)

	err := svc.OnObserveMemoryAvailable(ctx)
	assert.NoError(err)
}

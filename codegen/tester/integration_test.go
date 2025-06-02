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

package tester

import (
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/microbus-io/testarossa"

	"github.com/microbus-io/fabric/errors"
	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/fabric/pub"

	"github.com/microbus-io/fabric/codegen/tester/testerapi"
)

var (
	openAPI map[string]any
)

// Initialize starts up the testing app.
func Initialize() (err error) {
	// Add microservices to the testing app
	err = App.AddAndStartup(
		Svc,
	)
	if err != nil {
		return err
	}

	ctx := Context()
	res, err := Svc.Request(ctx, pub.GET("https://"+Hostname+"/openapi.json"))
	if err != nil {
		return err
	}
	err = json.NewDecoder(res.Body).Decode(&openAPI)
	if err != nil {
		return err
	}
	return nil
}

// Terminate gets called after the testing app shut down.
func Terminate() (err error) {
	return nil
}

// openAPIValue traverses the OpenAPI document and returns the value at the indicated path.
// The path uses the pipe as the separator character.
func openAPIValue(path string) any {
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
	tt := testarossa.For(t)
	/*
		ctx := Context()
		StringCut(t, ctx, s, sep).
			Expect(before, after, found)
	*/

	ctx := Context()

	// --- Test cases ---
	StringCut(t, ctx, "Hello World", " ").
		Expect("Hello", "World", true)
	StringCut(t, ctx, "Hello World", "X").
		Expect("Hello World", "", false)

	// --- Requests ---
	res, err := Svc.Request(ctx, pub.GET("https://"+Hostname+"/string-cut?s=Foo+Bar&Sep=+"))
	if tt.NoError(err) {
		var out testerapi.StringCutOut
		json.NewDecoder(res.Body).Decode(&out)
		tt.Equal("Foo", out.Before)
		tt.Equal("Bar", out.After)
		tt.Equal(true, out.Found)
	}

	// --- OpenAPI ---
	basePath := "paths|/" + Hostname + ":443/string-cut|post|"
	// Input arguments
	schemaRef := openAPIValue(basePath + "requestBody|content|application/json|schema|$ref").(string)
	schemaRef = strings.ReplaceAll(schemaRef, "/", "|")[2:] + "|"
	tt.Equal("string", openAPIValue(schemaRef+"properties|s|type"))
	tt.Equal("string", openAPIValue(schemaRef+"properties|sep|type"))
	// Output argument
	tt.NotNil(openAPIValue(basePath + "responses|2XX"))
	tt.NotNil(openAPIValue(basePath + "responses|4XX"))
	tt.NotNil(openAPIValue(basePath + "responses|5XX"))
	schemaRef = openAPIValue(basePath + "responses|2XX|content|application/json|schema|$ref").(string)
	schemaRef = strings.ReplaceAll(schemaRef, "/", "|")[2:] + "|"
	tt.Equal("object", openAPIValue(schemaRef+"type"))
	tt.Equal("string", openAPIValue(schemaRef+"properties|before|type"))
	tt.Equal("string", openAPIValue(schemaRef+"properties|after|type"))
	tt.Equal("boolean", openAPIValue(schemaRef+"properties|found|type"))
}

func TestTester_PointDistance(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)
	/*
		ctx := Context()
		PointDistance(t, ctx, p1, p2).
			Expect(d)
	*/

	ctx := Context()

	// --- Test cases ---
	PointDistance(t, ctx, testerapi.XYCoord{X: 1, Y: 1}, &testerapi.XYCoord{X: 4, Y: 5}).
		Expect(5)
	PointDistance(t, ctx, testerapi.XYCoord{X: 4, Y: 5}, &testerapi.XYCoord{X: 1, Y: 1}).
		Expect(5)
	PointDistance(t, ctx, testerapi.XYCoord{X: 1.5, Y: 1.6}, &testerapi.XYCoord{X: 2.5, Y: 2.6}).
		Assert(func(t *testing.T, d float64, err error) {
			tt.True(d >= math.Sqrt(2.0)-.01 && d <= math.Sqrt(2.0)+.01)
		})
	PointDistance(t, ctx, testerapi.XYCoord{X: 6.1, Y: 7.6}, &testerapi.XYCoord{X: 6.1, Y: 7.6}).
		Expect(0)

	// --- Requests ---
	res, err := Svc.Request(ctx, pub.GET("https://"+Hostname+"/point-distance?p1.x=1&p1.y=1&p2.x=4&p2.y=5"))
	if tt.NoError(err) {
		var out testerapi.PointDistanceOut
		json.NewDecoder(res.Body).Decode(&out)
		tt.Equal(5.0, out.D)
	}
	_, err = Svc.Request(ctx, pub.POST("https://"+Hostname+"/point-distance?p1.x=1&p1.y=1&p2.x=4&p2.y=5"))
	tt.Error(err)

	// --- OpenAPI ---
	basePath := "paths|/" + Hostname + ":443/point-distance|get|"
	// Input argument p1 refers to XYCoors with its own x and y
	tt.Equal("p1", openAPIValue(basePath+"parameters|0|name"))
	tt.Equal("query", openAPIValue(basePath+"parameters|0|in"))
	schemaRef := openAPIValue(basePath + "parameters|0|schema|$ref").(string)
	schemaRef = strings.ReplaceAll(schemaRef, "/", "|")[2:] + "|"
	tt.Equal("object", openAPIValue(schemaRef+"type"))
	tt.Equal("number", openAPIValue(schemaRef+"properties|x|type"))
	tt.Equal("number", openAPIValue(schemaRef+"properties|y|type"))
	// Input argument p2 refers to XYCoors with its own x and y
	tt.Equal("p2", openAPIValue(basePath+"parameters|1|name"))
	tt.Equal("query", openAPIValue(basePath+"parameters|1|in"))
	schemaRef = openAPIValue(basePath + "parameters|1|schema|$ref").(string)
	schemaRef = strings.ReplaceAll(schemaRef, "/", "|")[2:] + "|"
	tt.Equal("object", openAPIValue(schemaRef+"type"))
	tt.Equal("number", openAPIValue(schemaRef+"properties|x|type"))
	tt.Equal("number", openAPIValue(schemaRef+"properties|y|type"))
	// Output argument d is an int
	schemaRef = openAPIValue(basePath + "responses|2XX|content|application/json|schema|$ref").(string)
	schemaRef = strings.ReplaceAll(schemaRef, "/", "|")[2:] + "|"
	tt.Equal("object", openAPIValue(schemaRef+"type"))
	tt.Equal("number", openAPIValue(schemaRef+"properties|d|type"))
}

func TestTester_EchoAnything(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)
	/*
		ctx := Context()
		EchoAnything(t, ctx, x).
			Expect(y)
	*/

	ctx := Context()
	// --- Test cases ---
	EchoAnything(t, ctx, "string").
		Expect("string")
	EchoAnything(t, ctx, 5.0).
		Expect(5.0)
	EchoAnything(t, ctx, nil).
		Expect(nil)
	EchoAnything(t, ctx, testerapi.XYCoord{X: 5, Y: 6}).
		Expect(testerapi.XYCoord{X: 5, Y: 6})

	// --- Requests ---
	res, err := Svc.Request(ctx, pub.POST("https://"+Hostname+"/echo-anything"), pub.Body(struct {
		Original testerapi.XYCoord `json:"original"`
	}{
		testerapi.XYCoord{X: 1, Y: 2},
	}))
	if tt.NoError(err) {
		var out struct {
			Echoed testerapi.XYCoord `json:"echoed"`
		}
		json.NewDecoder(res.Body).Decode(&out)
		tt.Equal(1.0, out.Echoed.X)
		tt.Equal(2.0, out.Echoed.Y)
	}
	res, err = Svc.Request(ctx, pub.POST("https://"+Hostname+"/echo-anything?original=hello"))
	if tt.NoError(err) {
		var out struct {
			Echoed string `json:"echoed"`
		}
		json.NewDecoder(res.Body).Decode(&out)
		tt.Equal("hello", out.Echoed)
	}

	// --- OpenAPI ---
	basePath := "paths|/" + Hostname + ":443/echo-anything|post|"
	// Input argument should exist but have no type
	schemaRef := openAPIValue(basePath + "requestBody|content|application/json|schema|$ref").(string)
	schemaRef = strings.ReplaceAll(schemaRef, "/", "|")[2:] + "|"
	tt.NotNil(openAPIValue(schemaRef + "properties|original"))
	tt.Nil(openAPIValue(schemaRef + "properties|original|type"))
	// Output argument should exist but have no type
	schemaRef = openAPIValue(basePath + "responses|2XX|content|application/json|schema|$ref").(string)
	schemaRef = strings.ReplaceAll(schemaRef, "/", "|")[2:] + "|"
	tt.NotNil(openAPIValue(schemaRef + "properties|echoed"))
	tt.Nil(openAPIValue(schemaRef + "properties|echoed|type"))
}

func TestTester_ShiftPoint(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)
	/*
		ctx := Context()
		ShiftPoint(t, ctx, p, x, y).
			Expect(shifted)
	*/

	ctx := Context()

	// --- Test cases ---
	ShiftPoint(t, ctx, &testerapi.XYCoord{X: 5, Y: 6}, 3, 4).
		Expect(&testerapi.XYCoord{X: 5 + 3, Y: 6 + 4})
	ShiftPoint(t, ctx, &testerapi.XYCoord{X: 5, Y: 6}, -5, -6).
		Expect(&testerapi.XYCoord{})

	// --- Requests ---
	res, err := Svc.Request(ctx,
		pub.POST("https://"+Hostname+"/shift-point?x=10&y=10"),
		pub.Body(testerapi.ShiftPointIn{
			P: &testerapi.XYCoord{
				X: 5,
				Y: 6,
			},
		}))
	if tt.NoError(err) {
		var out testerapi.ShiftPointOut
		json.NewDecoder(res.Body).Decode(&out)
		tt.Equal(15.0, out.Shifted.X)
		tt.Equal(16.0, out.Shifted.Y)
	}
	res, err = Svc.Request(ctx, pub.GET("https://"+Hostname+"/shift-point?x=10&y=10&p.x=5&p.y=6"))
	if tt.NoError(err) {
		var out testerapi.ShiftPointOut
		json.NewDecoder(res.Body).Decode(&out)
		tt.Equal(15.0, out.Shifted.X)
		tt.Equal(16.0, out.Shifted.Y)
	}

	// --- OpenAPI ---
	basePath := "paths|/" + Hostname + ":443/shift-point|post|"
	// Input arguments x and y are ints
	schemaRef := openAPIValue(basePath + "requestBody|content|application/json|schema|$ref").(string)
	schemaRef = strings.ReplaceAll(schemaRef, "/", "|")[2:] + "|"
	tt.Equal("number", openAPIValue(schemaRef+"properties|x|type"))
	tt.Equal("number", openAPIValue(schemaRef+"properties|y|type"))
	// Input argument p refers to XYCoors with its own x and y
	schemaRef = openAPIValue(schemaRef + "properties|p|$ref").(string)
	schemaRef = strings.ReplaceAll(schemaRef, "/", "|")[2:] + "|"
	tt.Equal("number", openAPIValue(schemaRef+"properties|x|type"))
	tt.Equal("number", openAPIValue(schemaRef+"properties|y|type"))
	// Output argument shifted also refers to XYCoors
	schemaRef = openAPIValue(basePath + "responses|2XX|content|application/json|schema|$ref").(string)
	schemaRef = strings.ReplaceAll(schemaRef, "/", "|")[2:] + "|"
	schemaRef = openAPIValue(schemaRef + "properties|shifted|$ref").(string)
	schemaRef = strings.ReplaceAll(schemaRef, "/", "|")[2:] + "|"
	tt.Equal("object", openAPIValue(schemaRef+"type"))
	tt.Equal("number", openAPIValue(schemaRef+"properties|x|type"))
	tt.Equal("number", openAPIValue(schemaRef+"properties|y|type"))
}

func TestTester_SubArrayRange(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)
	/*
		ctx := Context()
		SubArrayRange(t, ctx, httpRequestBody, min, max).
			Expect(httpResponseBody, httpStatusCode)
	*/

	ctx := Context()

	// --- Test cases ---
	SubArrayRange(t, ctx, []int{1, 2, 3, 4, 5, 6}, 2, 4).
		Expect([]int{2, 3, 4}, http.StatusAccepted) // Sum is returned because calling directly

	sub, status, err := testerapi.NewClient(Svc).SubArrayRange(ctx, []int{1, 2, 3, 4, 5, 6}, 2, 4)
	if tt.NoError(err) {
		tt.Equal(sub, []int{2, 3, 4})
		tt.Equal(http.StatusAccepted, status)
	}

	// --- OpenAPI ---
	basePath := "paths|/" + Hostname + ":443/sub-array-range/{max}|post|"
	// Argument pushed to query because of httpRequestBody
	tt.Equal("min", openAPIValue(basePath+"parameters|0|name"))
	tt.Equal("query", openAPIValue(basePath+"parameters|0|in"))
	// Argument indicated in path
	tt.Equal("max", openAPIValue(basePath+"parameters|1|name"))
	tt.Equal("path", openAPIValue(basePath+"parameters|1|in"))
	// httpRequestBody should not be listed as an argument
	tt.Len(openAPIValue(basePath+"parameters").([]any), 2)
	// --- Requests --- schema is an array
	schemaRef := openAPIValue(basePath + "requestBody|content|application/json|schema|$ref").(string)
	schemaRef = strings.ReplaceAll(schemaRef, "/", "|")[2:] + "|"
	tt.Equal("array", openAPIValue(schemaRef+"type"))
	tt.Equal("integer", openAPIValue(schemaRef+"items|type"))
	// Response schema is an array
	schemaRef = openAPIValue(basePath + "responses|2XX|content|application/json|schema|$ref").(string)
	schemaRef = strings.ReplaceAll(schemaRef, "/", "|")[2:] + "|"
	tt.Equal("array", openAPIValue(schemaRef+"type"))
	tt.Equal("integer", openAPIValue(schemaRef+"items|type"))
}

func TestTester_WebPathArguments(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)
	/*
		ctx := Context()
		httpReq, _ := http.NewRequestWithContext(ctx, method, "?arg=val", body)
		WebPathArguments_Get(t, ctx, "").BodyContains(value)
		WebPathArguments_Post(t, ctx, "", "", body).BodyContains(value)
		WebPathArguments(t, httpRequest).BodyContains(value)
	*/

	ctx := Context()

	// --- Test cases ---
	WebPathArguments_Get(t, ctx, "?named=1&path2=2&suffix=3/4").
		BodyContains("/fixed/1/2/3/4$").
		BodyNotContains("?").
		BodyNotContains("{").
		BodyNotContains("}")
	WebPathArguments_Get(t, ctx, "?named=1&path2=2&suffix=3/4&q=5").
		BodyContains("/fixed/1/2/3/4?q=5$").
		BodyNotContains("&").
		BodyNotContains("{").
		BodyNotContains("}")
	WebPathArguments_Get(t, ctx, "").
		BodyContains("/fixed///$").
		BodyNotContains("?").
		BodyNotContains("&").
		BodyNotContains("{").
		BodyNotContains("}")
	WebPathArguments_Get(t, ctx, "?named="+url.QueryEscape("[a&b/c]")+"&path2="+url.QueryEscape("[d&e/f]")+"&suffix="+url.QueryEscape("[g&h/i]")+"&q="+url.QueryEscape("[j&k/l]")).
		BodyContains("/fixed/" + url.PathEscape("[a&b/c]") + "/" + url.PathEscape("[d&e/f]") + "/" + url.PathEscape("[g&h") + "/" + url.PathEscape("i]") + "?q=" + url.QueryEscape("[j&k/l]")).
		BodyNotContains("{").
		BodyNotContains("}")

	// --- OpenAPI ---
	basePath := "paths|/" + Hostname + ":443/web-path-arguments/fixed/{named}/{path2}/{suffix+}|get|"
	// named
	tt.Equal("named", openAPIValue(basePath+"parameters|0|name"))
	tt.Equal("path", openAPIValue(basePath+"parameters|0|in"))
	tt.Equal("string", openAPIValue(basePath+"parameters|0|schema|type"))
	// path2
	tt.Equal("path2", openAPIValue(basePath+"parameters|1|name"))
	tt.Equal("path", openAPIValue(basePath+"parameters|1|in"))
	tt.Equal("string", openAPIValue(basePath+"parameters|1|schema|type"))
	// suffix
	tt.Equal("suffix+", openAPIValue(basePath+"parameters|2|name"))
	tt.Equal("path", openAPIValue(basePath+"parameters|2|in"))
	tt.Equal("string", openAPIValue(basePath+"parameters|2|schema|type"))
}

func TestTester_FunctionPathArguments(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)
	/*
		ctx := Context()
		FunctionPathArguments(t, ctx, named, path2, suffix).
			Expect(joined)
	*/

	ctx := Context()

	// --- Test cases ---
	FunctionPathArguments(t, ctx, "1", "2", "3/4").
		Expect("1 2 3/4")
	FunctionPathArguments(t, ctx, "", "", "").
		Expect("  ")
	FunctionPathArguments(t, ctx, "[a&b$c]", "[d&e$f]", "[g&h/i]").
		Expect("[a&b$c] [d&e$f] [g&h/i]")

	// --- Client ---
	joined, err := testerapi.NewClient(Svc).FunctionPathArguments(ctx, "1", "2", "3/4")
	if tt.NoError(err) {
		tt.Equal(joined, "1 2 3/4")
	}
	joined, err = testerapi.NewClient(Svc).FunctionPathArguments(ctx, "", "", "")
	if tt.NoError(err) {
		tt.Equal(joined, "  ")
	}
	joined, err = testerapi.NewClient(Svc).FunctionPathArguments(ctx, "[a&b$c]", "[d&e$f]", "[g&h/i]")
	if tt.NoError(err) {
		tt.Equal(joined, "[a&b$c] [d&e$f] [g&h/i]")
	}

	// --- Requests ---
	res, err := Svc.Request(ctx, pub.GET("https://"+Hostname+"/function-path-arguments/fixed/1/2/3/4"))
	if tt.NoError(err) {
		var out testerapi.FunctionPathArgumentsOut
		json.NewDecoder(res.Body).Decode(&out)
		tt.Equal(`1 2 3/4`, out.Joined)
	}
	res, err = Svc.Request(ctx, pub.GET("https://"+Hostname+"/function-path-arguments/fixed///"))
	if tt.NoError(err) {
		var out testerapi.FunctionPathArgumentsOut
		json.NewDecoder(res.Body).Decode(&out)
		tt.Equal(`  `, out.Joined)
	}
	res, err = Svc.Request(ctx, pub.GET("https://"+Hostname+"/function-path-arguments/fixed/[a&b$c]/[d&e$f]/[g&h/i]"))
	if tt.NoError(err) {
		var out testerapi.FunctionPathArgumentsOut
		json.NewDecoder(res.Body).Decode(&out)
		tt.Equal(`[a&b$c] [d&e$f] [g&h/i]`, out.Joined)
	}

	// --- OpenAPI ---
	basePath := "paths|/" + Hostname + ":443/function-path-arguments/fixed/{named}/{path2}/{suffix+}|get|"
	// named
	tt.Equal("named", openAPIValue(basePath+"parameters|0|name"))
	tt.Equal("path", openAPIValue(basePath+"parameters|0|in"))
	tt.Equal("string", openAPIValue(basePath+"parameters|0|schema|type"))
	// path2
	tt.Equal("path2", openAPIValue(basePath+"parameters|1|name"))
	tt.Equal("path", openAPIValue(basePath+"parameters|1|in"))
	tt.Equal("string", openAPIValue(basePath+"parameters|1|schema|type"))
	// suffix
	tt.Equal("suffix+", openAPIValue(basePath+"parameters|2|name"))
	tt.Equal("path", openAPIValue(basePath+"parameters|2|in"))
	tt.Equal("string", openAPIValue(basePath+"parameters|2|schema|type"))
	// Response
	schemaRef := openAPIValue(basePath + "responses|2XX|content|application/json|schema|$ref").(string)
	schemaRef = strings.ReplaceAll(schemaRef, "/", "|")[2:] + "|"
	tt.Equal("string", openAPIValue(schemaRef+"properties|joined|type"))
}

func TestTester_NonStringPathArguments(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)
	/*
		ctx := Context()
		NonStringPathArguments(t, ctx, named, path2, suffix).
			Expect(joined)
	*/

	ctx := Context()

	// --- Test cases ---
	NonStringPathArguments(t, ctx, 1, true, 0.75).
		Expect("1 true 0.75")

	// --- Requests ---
	_, err := Svc.Request(ctx, pub.GET("https://"+Hostname+"/non-string-path-arguments/fixed/1.5/true/0.75"))
	tt.Contains(err, "json")
	_, err = Svc.Request(ctx, pub.GET("https://"+Hostname+"/non-string-path-arguments/fixed/1/x/0.75"))
	tt.Contains(err, "invalid character")
	_, err = Svc.Request(ctx, pub.GET("https://"+Hostname+"/non-string-path-arguments/fixed/1/true/x"))
	tt.Contains(err, "invalid character")
	_, err = Svc.Request(ctx, pub.GET("https://"+Hostname+"/non-string-path-arguments/fixed/1/true/0.75"))
	tt.NoError(err)

	// --- OpenAPI ---
	basePath := "paths|/" + Hostname + ":443/non-string-path-arguments/fixed/{named}/{path2}/{suffix+}|get|"
	// named
	tt.Equal("named", openAPIValue(basePath+"parameters|0|name"))
	tt.Equal("path", openAPIValue(basePath+"parameters|0|in"))
	tt.Equal("integer", openAPIValue(basePath+"parameters|0|schema|type"))
	// path2
	tt.Equal("path2", openAPIValue(basePath+"parameters|1|name"))
	tt.Equal("path", openAPIValue(basePath+"parameters|1|in"))
	tt.Equal("boolean", openAPIValue(basePath+"parameters|1|schema|type"))
	// suffix
	tt.Equal("suffix+", openAPIValue(basePath+"parameters|2|name"))
	tt.Equal("path", openAPIValue(basePath+"parameters|2|in"))
	tt.Equal("number", openAPIValue(basePath+"parameters|2|schema|type"))
	// Response
	schemaRef := openAPIValue(basePath + "responses|2XX|content|application/json|schema|$ref").(string)
	schemaRef = strings.ReplaceAll(schemaRef, "/", "|")[2:] + "|"
	tt.Equal("string", openAPIValue(schemaRef+"properties|joined|type"))
}

func TestTester_UnnamedFunctionPathArguments(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)
	/*
		ctx := Context()
		UnnamedFunctionPathArguments(t, ctx, path1, path2, path3).
			Expect(joined)
	*/

	ctx := Context()

	// --- Requests ---
	res, err := Svc.Request(ctx, pub.GET("https://"+Hostname+"/unnamed-function-path-arguments/x123/foo/y345/bar/z1/z2/z3"))
	tt.NoError(err)
	body, _ := io.ReadAll(res.Body)
	tt.Contains(string(body), "x123 y345 z1/z2/z3")

	// --- OpenAPI ---
	basePath := "paths|/" + Hostname + ":443/unnamed-function-path-arguments/{path1}/foo/{path2}/bar/{path3+}|get|"
	tt.Equal("path1", openAPIValue(basePath+"parameters|0|name"))
	tt.Equal("path", openAPIValue(basePath+"parameters|0|in"))
	tt.Equal("path2", openAPIValue(basePath+"parameters|1|name"))
	tt.Equal("path", openAPIValue(basePath+"parameters|1|in"))
	tt.Equal("path3+", openAPIValue(basePath+"parameters|2|name"))
	tt.Equal("path", openAPIValue(basePath+"parameters|2|in"))
}

func TestTester_UnnamedWebPathArguments(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)
	/*
		ctx := Context()
		httpReq, _ := http.NewRequestWithContext(ctx, method, "?arg=val", body)
		UnnamedWebPathArguments(t, ctx, "").BodyContains(value)
		UnnamedWebPathArguments_Do(t, httpRequest).BodyContains(value)
	*/

	ctx := Context()

	// --- Requests ---
	res, err := Svc.Request(ctx, pub.GET("https://"+Hostname+"/unnamed-web-path-arguments/x123/foo/y345/bar/z1/z2/z3"))
	tt.NoError(err)
	body, _ := io.ReadAll(res.Body)
	tt.Contains(string(body), "x123 y345 z1/z2/z3")

	// --- OpenAPI ---
	basePath := "paths|/" + Hostname + ":443/unnamed-web-path-arguments/{path1}/foo/{path2}/bar/{path3+}|get|"
	tt.Equal("path1", openAPIValue(basePath+"parameters|0|name"))
	tt.Equal("path", openAPIValue(basePath+"parameters|0|in"))
	tt.Equal("path2", openAPIValue(basePath+"parameters|1|name"))
	tt.Equal("path", openAPIValue(basePath+"parameters|1|in"))
	tt.Equal("path3+", openAPIValue(basePath+"parameters|2|name"))
	tt.Equal("path", openAPIValue(basePath+"parameters|2|in"))
}

func TestTester_SumTwoIntegers(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)
	/*
		ctx := Context()
		SumTwoIntegers(t, ctx, x, y).
			Expect(sum, httpStatusCode)
	*/

	ctx := Context()

	// --- Test cases ---
	SumTwoIntegers(t, ctx, 5, 6).
		Expect(11, http.StatusAccepted)
	SumTwoIntegers(t, ctx, 5, -6).
		Expect(-1, http.StatusNotAcceptable)

	// --- Requests ---
	res, err := Svc.Request(ctx, pub.GET("https://"+Hostname+"/sum-two-integers?x=73&y=83"))
	if tt.NoError(err) {
		// The status code is not returned in the body but only through the status code field of the response
		tt.Equal(http.StatusAccepted, res.StatusCode)
		body, _ := io.ReadAll(res.Body)
		tt.Contains(string(body), "156")
		tt.NotContains("httpStatusCode", string(body))
		tt.NotContains(strconv.Itoa(http.StatusAccepted), string(body))
	}
}

func TestTester_Echo(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)
	/*
		ctx := Context()
		Echo_Get(t, ctx, "").BodyContains(value)
		Echo_Post(t, ctx, "", "", body).BodyContains(value)
		httpReq, _ := http.NewRequestWithContext(ctx, method, "?arg=val", body)
		Echo(t, httpReq).BodyContains(value)
	*/

	ctx := Context()

	// --- Test cases ---
	Echo_Get(t, ctx, "?alpha=111&beta=222").
		BodyContains("GET /").
		BodyContains("alpha=111&beta=222").
		NoError()
	Echo_Post(t, ctx, "?alpha=111&beta=222", "text/plain", "HEAVY PAYLOAD").
		BodyContains("POST /").
		BodyContains("alpha=111&beta=222").
		BodyContains("text/plain").
		BodyContains("HEAVY PAYLOAD").
		NoError()
	httpReq, _ := http.NewRequestWithContext(ctx, "PUT", "?alpha=111&beta=222", strings.NewReader("HEAVY PAYLOAD"))
	httpReq.Header.Set("Content-Type", "text/plain")
	Echo(t, httpReq).
		BodyContains("PUT /").
		BodyContains("alpha=111&beta=222").
		BodyContains("Content-Type: text/plain").
		BodyContains("HEAVY PAYLOAD").
		NoError()

	// --- Requests ---
	res, err := Svc.Request(ctx,
		pub.PATCH("https://"+Hostname+"/echo?alpha=111&beta=222"),
		pub.Body("HEAVY PAYLOAD"),
		pub.ContentType("text/plain"),
	)
	if tt.NoError(err) {
		body, _ := io.ReadAll(res.Body)
		tt.Contains(string(body), "PATCH /")
		tt.Contains(string(body), "alpha=111&beta=222")
		tt.Contains(string(body), "Content-Type: text/plain")
		tt.Contains(string(body), "HEAVY PAYLOAD")
	}

	// --- Client ---

	r, err := http.NewRequestWithContext(ctx, "PATCH", "https://"+Hostname+"/echo", strings.NewReader("Lorem ipsum"))
	r.Header.Add("Foo", "Bar")
	tt.NoError(err)
	res, err = testerapi.NewClient(Svc).Echo(r)
	if tt.NoError(err) {
		body, _ := io.ReadAll(res.Body)
		tt.Contains(string(body), "PATCH /")
		tt.Contains(string(body), "Lorem ipsum")
		tt.Contains(string(body), "Foo: Bar")
	}
	res, err = testerapi.NewClient(Svc).WithOptions(
		pub.Method("POST"),
		pub.Body("Dolor sit amet"),
		pub.Header("Foo", "Baz"),
	).Echo(r)
	if tt.NoError(err) {
		body, _ := io.ReadAll(res.Body)
		tt.NotContains(string(body), "PATCH /")
		tt.NotContains(string(body), "Lorem ipsum")
		tt.NotContains(string(body), "Foo: Bar")
		tt.Contains(string(body), "POST /")
		tt.Contains(string(body), "Dolor sit amet")
		tt.Contains(string(body), "Foo: Baz")
	}

	// --- OpenAPI ---
	basePath := "paths|/" + Hostname + ":443/echo|get|"
	tt.NotNil(openAPIValue(basePath + "responses|2XX"))
	tt.NotNil(openAPIValue(basePath + "responses|4XX"))
	tt.NotNil(openAPIValue(basePath + "responses|5XX"))
}

func TestTester_MultiValueHeaders(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)
	/*
		ctx := Context()
		MultiValueHeaders_Get(t, ctx, "").BodyContains(value)
		MultiValueHeaders_Post(t, ctx, "", "", body).BodyContains(value)
		httpReq, _ := http.NewRequestWithContext(ctx, method, "?arg=val", body)
		MultiValueHeaders(t, httpReq).BodyContains(value)
	*/

	ctx := Context()

	// --- Test cases ---
	httpReq, _ := http.NewRequestWithContext(ctx, "GET", "", nil)
	httpReq.Header.Add("Multi-In", "In1")
	httpReq.Header.Add("Multi-In", "In2")
	res, _ := MultiValueHeaders(t, httpReq).NoError().Get()
	tt.Len(res.Header["Multi-Out"], 2)
	httpReq, _ = http.NewRequestWithContext(ctx, "POST", "", strings.NewReader("Payload"))
	httpReq.Header.Add("Multi-In", "In1")
	httpReq.Header.Add("Multi-In", "In2")
	res, _ = MultiValueHeaders(t, httpReq).NoError().Get()
	tt.Len(res.Header["Multi-Out"], 2)
}

func TestTester_PathArgumentsPriority(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)
	/*
		ctx := Context()
		PathArgumentsPriority(t, ctx, foo).
			Expect(echo)
	*/

	ctx := Context()

	// --- Test cases ---
	PathArgumentsPriority(t, ctx, "BAR").
		Expect("BAR")
	PathArgumentsPriority(t, ctx, "XYZ").
		Expect("XYZ")

	// --- Requests ---
	// Argument in the path should take priority over that in the query
	res, err := Svc.Request(ctx, pub.GET("https://"+Hostname+"/path-arguments-priority/BAR?foo=XYZ"))
	if tt.NoError(err) {
		b, _ := io.ReadAll(res.Body)
		tt.Contains(string(b), "BAR")
		tt.NotContains(string(b), "XYZ")
	}

	// If argument is not provided in the path, take from the query
	res, err = Svc.Request(ctx, pub.GET("https://"+Hostname+"/path-arguments-priority/{foo}?foo=BAR"))
	if tt.NoError(err) {
		b, _ := io.ReadAll(res.Body)
		tt.Contains(string(b), "BAR")
	}

	// Argument in the path should take priority over that in the body
	res, err = Svc.Request(ctx, pub.POST("https://"+Hostname+"/path-arguments-priority/BAR"), pub.Body(`{"foo":"XYZ"}`))
	if tt.NoError(err) {
		b, _ := io.ReadAll(res.Body)
		tt.Contains(string(b), "BAR")
		tt.NotContains(string(b), "XYZ")
	}

	// If argument is not provided in the path, take from the body
	res, err = Svc.Request(ctx, pub.POST("https://"+Hostname+"/path-arguments-priority/{foo}"), pub.Body(`{"foo":"BAR"}`))
	if tt.NoError(err) {
		b, _ := io.ReadAll(res.Body)
		tt.Contains(string(b), "BAR")
	}

	// If argument is not provided in the path, take from the query over the body
	res, err = Svc.Request(ctx, pub.POST("https://"+Hostname+"/path-arguments-priority/{foo}?foo=BAR"), pub.Body(`{"foo":"XYZ"}`))
	if tt.NoError(err) {
		b, _ := io.ReadAll(res.Body)
		tt.Contains(string(b), "BAR")
		tt.NotContains(string(b), "XYZ")
	}
}

func TestTester_DirectoryServer(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)
	/*
		ctx := Context()
		DirectoryServer(t, ctx, "").BodyContains(value)
		httpReq, _ := http.NewRequestWithContext(ctx, method, "?arg=val", body)
		DirectoryServer_Do(t, httpReq).BodyContains(value)
	*/

	ctx := Context()

	// --- Test cases ---
	DirectoryServer(t, ctx, "1.txt").BodyContains("111")
	DirectoryServer(t, ctx, "/directory-server/1.txt").BodyContains("111")
	DirectoryServer(t, ctx, "https://"+Hostname+"/directory-server/1.txt").BodyContains("111")

	DirectoryServer(t, ctx, "sub/2.txt").BodyContains("222")
	DirectoryServer(t, ctx, "sub/3.txt").ErrorCode(http.StatusNotFound)

	DirectoryServer(t, ctx, "../3.txt").ErrorCode(http.StatusNotFound)
	DirectoryServer(t, ctx, "sub/../../3.txt").ErrorCode(http.StatusNotFound)

	httpReq, _ := http.NewRequestWithContext(ctx, "GET", "1.txt", nil)
	DirectoryServer_Do(t, httpReq).BodyContains("111")
	httpReq, _ = http.NewRequestWithContext(ctx, "POST", "1.txt", strings.NewReader("Payload"))
	DirectoryServer_Do(t, httpReq).ErrorCode(http.StatusNotFound)

	// --- Client ---
	res, err := testerapi.NewClient(Svc).DirectoryServer(ctx, "1.txt")
	if tt.NoError(err) {
		b, _ := io.ReadAll(res.Body)
		tt.Contains(string(b), "111")
	}
	res, err = testerapi.NewClient(Svc).DirectoryServer(ctx, "sub/2.txt")
	if tt.NoError(err) {
		b, _ := io.ReadAll(res.Body)
		tt.Contains(string(b), "222")
	}
	_, err = testerapi.NewClient(Svc).DirectoryServer(ctx, "../3.txt")
	tt.Error(err)
	httpReq, _ = http.NewRequestWithContext(ctx, "POST", "1.txt", strings.NewReader("Payload"))
	_, err = testerapi.NewClient(Svc).DirectoryServer_Do(httpReq)
	tt.Error(err)

	// --- Requests ---
	res, err = Svc.Request(ctx, pub.GET("https://"+Hostname+"/directory-server/1.txt"))
	if tt.NoError(err) {
		b, _ := io.ReadAll(res.Body)
		tt.Contains(string(b), "111")
	}
	res, err = Svc.Request(ctx, pub.GET("https://"+Hostname+"/directory-server/sub/2.txt"))
	if tt.NoError(err) {
		b, _ := io.ReadAll(res.Body)
		tt.Contains(string(b), "222")
	}
	_, err = Svc.Request(ctx, pub.GET("https://"+Hostname+"/directory-server/../3.txt"))
	tt.Error(err)
	_, err = Svc.Request(ctx, pub.POST("https://"+Hostname+"/directory-server/1.txt"))
	tt.Error(err)

	// --- OpenAPI ---
	basePath := "paths|/" + Hostname + ":443/directory-server/{filename+}|get|"
	tt.Equal("filename+", openAPIValue(basePath+"parameters|0|name"))
	tt.Equal("path", openAPIValue(basePath+"parameters|0|in"))
}

func TestTester_LinesIntersection(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)
	/*
		ctx := Context()
		LinesIntersection(t, ctx, l1, l2).
			Expect(b)
	*/

	ctx := Context()

	// --- Test cases ---
	LinesIntersection(t, ctx,
		testerapi.XYLine{
			Start: testerapi.XYCoord{X: 1, Y: 1},
			End:   testerapi.XYCoord{X: 10, Y: 1},
		}, &testerapi.XYLine{
			Start: testerapi.XYCoord{X: 1, Y: 2},
			End:   testerapi.XYCoord{X: 10, Y: 2},
		}).
		Expect(false)
	LinesIntersection(t, ctx,
		testerapi.XYLine{
			Start: testerapi.XYCoord{X: 10, Y: 1},
			End:   testerapi.XYCoord{X: 0, Y: 10},
		}, &testerapi.XYLine{
			Start: testerapi.XYCoord{X: 0, Y: 0},
			End:   testerapi.XYCoord{X: 10, Y: 10},
		}).
		Expect(true)
	LinesIntersection(t, ctx,
		testerapi.XYLine{
			Start: testerapi.XYCoord{X: -5, Y: -5},
			End:   testerapi.XYCoord{X: 0, Y: 0},
		}, &testerapi.XYLine{
			Start: testerapi.XYCoord{X: 1, Y: 1},
			End:   testerapi.XYCoord{X: 10, Y: 10},
		}).
		Expect(false)

	// --- Client ---
	b, err := testerapi.NewClient(Svc).LinesIntersection(ctx,
		testerapi.XYLine{
			Start: testerapi.XYCoord{X: 10, Y: 1},
			End:   testerapi.XYCoord{X: 0, Y: 10},
		}, &testerapi.XYLine{
			Start: testerapi.XYCoord{X: 0, Y: 0},
			End:   testerapi.XYCoord{X: 10, Y: 10},
		})
	if tt.NoError(err) {
		tt.True(b)
	}
	b, err = testerapi.NewClient(Svc).LinesIntersection(ctx,
		testerapi.XYLine{
			Start: testerapi.XYCoord{X: -5, Y: -5},
			End:   testerapi.XYCoord{X: 0, Y: 0},
		}, &testerapi.XYLine{
			Start: testerapi.XYCoord{X: 1, Y: 1},
			End:   testerapi.XYCoord{X: 10, Y: 10},
		})
	if tt.NoError(err) {
		tt.False(b)
	}

	// --- Requests ---
	res, err := Svc.Request(ctx,
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
	if tt.NoError(err) {
		var out testerapi.LinesIntersectionOut
		json.NewDecoder(res.Body).Decode(&out)
		tt.Equal(out.B, true)
	}

	// --- OpenAPI ---
	basePath := "paths|/" + Hostname + ":443/lines-intersection|post|"
	// Input arguments l1 and l2 are lines
	schemaRef := openAPIValue(basePath + "requestBody|content|application/json|schema|$ref").(string)
	schemaRef = strings.ReplaceAll(schemaRef, "/", "|")[2:] + "|"
	l1SchemaRef := openAPIValue(schemaRef + "properties|l1|$ref").(string)
	l1SchemaRef = strings.ReplaceAll(l1SchemaRef, "/", "|")[2:] + "|"
	startSchemaRef := openAPIValue(l1SchemaRef + "properties|start|$ref").(string)
	startSchemaRef = strings.ReplaceAll(startSchemaRef, "/", "|")[2:] + "|"
	tt.Equal("number", openAPIValue(startSchemaRef+"properties|x|type"))
	tt.Equal("number", openAPIValue(startSchemaRef+"properties|y|type"))
	endSchemaRef := openAPIValue(l1SchemaRef + "properties|start|$ref").(string)
	endSchemaRef = strings.ReplaceAll(endSchemaRef, "/", "|")[2:] + "|"
	tt.Equal("number", openAPIValue(endSchemaRef+"properties|x|type"))
	tt.Equal("number", openAPIValue(endSchemaRef+"properties|y|type"))

	l2SchemaRef := openAPIValue(schemaRef + "properties|l1|$ref").(string)
	l2SchemaRef = strings.ReplaceAll(l2SchemaRef, "/", "|")[2:] + "|"
	startSchemaRef = openAPIValue(l2SchemaRef + "properties|start|$ref").(string)
	startSchemaRef = strings.ReplaceAll(startSchemaRef, "/", "|")[2:] + "|"
	tt.Equal("number", openAPIValue(startSchemaRef+"properties|x|type"))
	tt.Equal("number", openAPIValue(startSchemaRef+"properties|y|type"))
	endSchemaRef = openAPIValue(l2SchemaRef + "properties|start|$ref").(string)
	endSchemaRef = strings.ReplaceAll(endSchemaRef, "/", "|")[2:] + "|"
	tt.Equal("number", openAPIValue(endSchemaRef+"properties|x|type"))
	tt.Equal("number", openAPIValue(endSchemaRef+"properties|y|type"))

	// Output argument is a boolean
	schemaRef = openAPIValue(basePath + "responses|2XX|content|application/json|schema|$ref").(string)
	schemaRef = strings.ReplaceAll(schemaRef, "/", "|")[2:] + "|"
	tt.Equal("boolean", openAPIValue(schemaRef+"properties|b|type"))
}

func TestTester_OnDiscoveredSink(t *testing.T) {
	t.Parallel()

	/*
		ctx := Context()
		OnDiscoveredSink(t, ctx, p, n).
			Expect(q, m)
	*/

	ctx := Context()
	OnDiscoveredSink(t, ctx, testerapi.XYCoord{X: 5, Y: -6}, -2).
		Expect(testerapi.XYCoord{X: -5, Y: 6}, -1)
	OnDiscoveredSink(t, ctx, testerapi.XYCoord{X: 5, Y: -6}, 3).
		Expect(testerapi.XYCoord{X: 5, Y: -6}, 4)
	OnDiscoveredSink(t, ctx, testerapi.XYCoord{X: 5, Y: -6}, 0).
		Error("zero")
}

func TestTester_OnDiscovered(t *testing.T) {
	// No parallel: event sinks might clash across tests
	tt := testarossa.For(t)

	/*
		ctx := Context()
		tc := OnDiscovered(t).
			Expect(p, n).
			Return(q, m, err)
		...
		tc.Wait()
	*/

	ctx := Context()

	tc := OnDiscovered(t).
		Expect(testerapi.XYCoord{X: 5, Y: -6}, 3).
		Return(testerapi.XYCoord{X: 5, Y: -6}, 4, nil)
	q, m, err := (<-testerapi.NewMulticastTrigger(Svc).OnDiscovered(ctx, testerapi.XYCoord{X: 5, Y: -6}, 3)).Get()
	if tt.NoError(err) {
		tt.Equal(testerapi.XYCoord{X: 5, Y: -6}, q)
		tt.Equal(4, m)
	}
	tc.Wait()

	tc = OnDiscovered(t).
		Expect(testerapi.XYCoord{X: 5, Y: -6}, -3).
		Return(testerapi.XYCoord{X: -5, Y: 6}, -2, nil)
	go func() { // Async
		q, m, err := (<-testerapi.NewMulticastTrigger(Svc).OnDiscovered(ctx, testerapi.XYCoord{X: 5, Y: -6}, -3)).Get()
		if tt.NoError(err) {
			tt.Equal(testerapi.XYCoord{X: -5, Y: 6}, q)
			tt.Equal(-2, m)
		}
	}()
	tc.Wait()

	tc = OnDiscovered(t).
		Expect(testerapi.XYCoord{X: 5, Y: -6}, -3).
		Return(testerapi.XYCoord{X: -5, Y: 6}, -2, nil)
	res := <-Svc.Publish(ctx, pub.PATCH("https://"+Hostname+":417/on-discovered"), pub.Body(&testerapi.OnDiscoveredIn{
		P: testerapi.XYCoord{X: 5, Y: -6},
		N: -3,
	}))
	tt.Nil(res) // Wrong HTTP method
	res = <-Svc.Publish(ctx, pub.POST("https://"+Hostname+":417/on-discovered"), pub.Body(&testerapi.OnDiscoveredIn{
		P: testerapi.XYCoord{X: 5, Y: -6},
		N: -3,
	}))
	httpRes, err := res.Get()
	if tt.NoError(err) {
		var out testerapi.OnDiscoveredOut
		json.NewDecoder(httpRes.Body).Decode(&out)
		tt.Equal(testerapi.XYCoord{X: -5, Y: 6}, out.Q)
		tt.Equal(-2, out.M)
	}
	tc.Wait()
}

func TestTester_Hello(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)
	/*
		ctx := Context()
		Hello_Get(t, ctx, "").BodyContains(value)
		Hello_Post(t, ctx, "", "", body).BodyContains(value)
		httpReq, _ := http.NewRequestWithContext(ctx, method, "?arg=val", body)
		Hello(t, httpReq).BodyContains(value)
	*/

	// --- Request header ---
	r, _ := http.NewRequest("GET", "", nil)

	Hello(t, r).
		StatusOK().
		BodyContains("Hello")
	frame.Of(r).SetLanguages("en")
	Hello(t, r).
		StatusOK().
		BodyContains("Hello")
	frame.Of(r).SetLanguages("en-NZ")
	Hello(t, r).
		StatusOK().
		BodyContains("Hello")
	frame.Of(r).SetLanguages("it")
	Hello(t, r).
		StatusOK().
		BodyContains("Salve")

	// --- Context ---
	ctx := Context()
	r, _ = http.NewRequestWithContext(ctx, "GET", "", nil)
	Hello(t, r).
		StatusOK().
		BodyContains("Hello")
	frame.Of(ctx).SetLanguages("it")
	Hello(t, r).
		StatusOK().
		BodyContains("Salve")

	// --- Request ---
	ctx = Context()
	res, err := Svc.Request(ctx, pub.GET("https://"+Hostname+"/hello"))
	if tt.NoError(err) {
		b, _ := io.ReadAll(res.Body)
		tt.Equal("Hello", string(b))
	}
	res, err = Svc.Request(ctx, pub.GET("https://"+Hostname+"/hello"), pub.Header("Accept-Language", "it"))
	if tt.NoError(err) {
		b, _ := io.ReadAll(res.Body)
		tt.Equal("Salve", string(b))
	}
	frame.Of(ctx).SetLanguages("it")
	res, err = Svc.Request(ctx, pub.GET("https://"+Hostname+"/hello"))
	if tt.NoError(err) {
		b, _ := io.ReadAll(res.Body)
		tt.Equal("Salve", string(b))
	}
}

func TestTester_WhatTimeIsIt(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)
	/*
		ctx := Context()
		WhatTimeIsIt(t, ctx).
			Expect(t)
	*/

	ctx := Context()
	realNow := time.Now()

	// --- Test cases ---
	withinRange := func(t *testing.T, tm, left, right time.Time) {
		tt.True(!tm.Before(left) && !tm.After(right))
	}
	tm, _ := WhatTimeIsIt(t, ctx).NoError().Get()
	withinRange(t, tm, realNow.Add(-time.Second), realNow.Add(time.Second))

	frame.Of(ctx).SetClockShift(time.Hour)
	tm, _ = WhatTimeIsIt(t, ctx).NoError().Get()
	withinRange(t, tm, realNow.Add(time.Hour-time.Second), realNow.Add(time.Hour+time.Second))

	frame.Of(ctx).SetClockShift(0)
	tm, _ = WhatTimeIsIt(t, ctx).NoError().Get()
	withinRange(t, tm, realNow.Add(-time.Second), realNow.Add(time.Second))

	// --- Client ---
	ctx = Context()
	tm, err := testerapi.NewClient(Svc).WhatTimeIsIt(ctx)
	if tt.NoError(err) {
		withinRange(t, tm, realNow.Add(-time.Second), realNow.Add(time.Second))
	}
	frame.Of(ctx).SetClockShift(time.Hour)
	tm, err = testerapi.NewClient(Svc).WhatTimeIsIt(ctx)
	if tt.NoError(err) {
		withinRange(t, tm, realNow.Add(time.Hour-time.Second), realNow.Add(time.Hour+time.Second))
	}

	// --- Request ---
	ctx = Context()
	res, err := Svc.Request(ctx, pub.GET("https://"+Hostname+"/what-time-is-it"))
	if tt.NoError(err) {
		var out testerapi.WhatTimeIsItOut
		json.NewDecoder(res.Body).Decode(&out)
		withinRange(t, out.T, realNow.Add(-time.Second), realNow.Add(time.Second))
	}
	frame.Of(ctx).SetClockShift(time.Hour)
	res, err = Svc.Request(ctx, pub.GET("https://"+Hostname+"/what-time-is-it"))
	if tt.NoError(err) {
		var out testerapi.WhatTimeIsItOut
		json.NewDecoder(res.Body).Decode(&out)
		withinRange(t, tm, realNow.Add(time.Hour-time.Second), realNow.Add(time.Hour+time.Second))
	}

	// --- OpenAPI ---
	basePath := "paths|/" + Hostname + ":443/what-time-is-it|post|"
	// Output argument
	schemaRef := openAPIValue(basePath + "responses|2XX|content|application/json|schema|$ref").(string)
	schemaRef = strings.ReplaceAll(schemaRef, "/", "|")[2:] + "|"
	tt.Equal("object", openAPIValue(schemaRef+"type"))
	tt.Equal("string", openAPIValue(schemaRef+"properties|t|type"))
	tt.Equal("date-time", openAPIValue(schemaRef+"properties|t|format"))
}

func TestTester_AuthzRequired(t *testing.T) {
	t.Parallel()
	tt := testarossa.For(t)

	ctx := Context()

	// Without auth token
	err := testerapi.NewClient(Svc).AuthzRequired(ctx)
	if tt.Error(err) {
		tt.Equal(http.StatusUnauthorized, errors.StatusCode(err))
	}

	// With insufficient role
	frame.Of(ctx).SetActor(jwt.MapClaims{"roles": "d"})
	err = testerapi.NewClient(Svc).AuthzRequired(ctx)
	if tt.Error(err) {
		tt.Equal(http.StatusForbidden, errors.StatusCode(err))
	}

	// With sufficient roles
	frame.Of(ctx).SetActor(jwt.MapClaims{"roles": "ac"})
	err = testerapi.NewClient(Svc).AuthzRequired(ctx)
	tt.NoError(err)

	frame.Of(ctx).SetActor(jwt.MapClaims{"scopes": "r"})
	err = testerapi.NewClient(Svc).AuthzRequired(ctx)
	tt.NoError(err)

	// --- OpenAPI ---
	basePath := "paths|/" + Hostname + ":443/authz-required|post|"
	tt.Equal([]any{}, openAPIValue(basePath+"security|0|http_bearer_jwt"))
	tt.Contains(openAPIValue(basePath+"responses|403|description").(string), `roles=~"(a|b|c)" || scopes=~"r"`)
	securitySchemePath := "components|securitySchemes|http_bearer_jwt|"
	tt.Equal("http", openAPIValue(securitySchemePath+"type"))
	tt.Equal("bearer", openAPIValue(securitySchemePath+"scheme"))
	tt.Equal("JWT", openAPIValue(securitySchemePath+"bearerFormat"))
}

func TestTester_OnceAMinute(t *testing.T) {
	t.Parallel()

	ctx := Context()
	OnceAMinute(t, ctx).NoError()
}

func TestTester_OnObserveMemoryAvailable(t *testing.T) {
	t.Parallel()

	ctx := Context()
	OnObserveMemoryAvailable(t, ctx).NoError()
}

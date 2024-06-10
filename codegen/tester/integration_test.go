/*
Copyright (c) 2023-2024 Microbus LLC and various contributors

This file and the project encapsulating it are the confidential intellectual property of Microbus LLC.
Neither may be used, copied or distributed without the express written consent of Microbus LLC.
*/

package tester

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/microbus-io/fabric/pub"
	"github.com/microbus-io/fabric/service"

	"github.com/microbus-io/fabric/codegen/tester/testerapi"
)

var (
	openAPI map[string]any
)

// Initialize starts up the testing app.
func Initialize() (err error) {
	App.Init(func(svc service.Service) {
		// Initialize all microservices
	})

	// Include all downstream microservices in the testing app
	App.Include(
		Svc.Init(func(svc *Service) {
			// Initialize the microservice under test
		}),
		// downstream.NewService().Init(func(svc *downstream.Service) {}),
	)

	err = App.Startup()
	if err != nil {
		return err
	}
	// All microservices are now running

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

// Terminate shuts down the testing app.
func Terminate() (err error) {
	err = App.Shutdown()
	if err != nil {
		return err
	}
	return nil
}

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
	if assert.NoError(t, err) {
		var out testerapi.StringCutOut
		json.NewDecoder(res.Body).Decode(&out)
		assert.Equal(t, "Foo", out.Before)
		assert.Equal(t, "Bar", out.After)
		assert.Equal(t, true, out.Found)
	}

	// --- Mock ---
	mock := NewMock()
	mock.SetHostname("string-cut.mock")
	mock.MockStringCut(func(ctx context.Context, s, sep string) (before string, after string, found bool, err error) {
		assert.Equal(t, s, "123XMX456")
		assert.Equal(t, sep, "XMX")
		return "123", "456", true, nil
	})
	App.Join(mock)
	err = mock.Startup()
	assert.NoError(t, err)
	defer mock.Shutdown()

	res, err = Svc.Request(ctx, pub.GET("https://string-cut.mock/string-cut?s=123XMX456&sep=XMX"))
	if assert.NoError(t, err) {
		var out testerapi.StringCutOut
		json.NewDecoder(res.Body).Decode(&out)
		assert.Equal(t, "123", out.Before)
		assert.Equal(t, "456", out.After)
		assert.Equal(t, true, out.Found)
	}
}

func TestTester_PointDistance(t *testing.T) {
	t.Parallel()
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
			assert.InDelta(t, math.Sqrt(2.0), d, 0.01)
		})
	PointDistance(t, ctx, testerapi.XYCoord{X: 6.1, Y: 7.6}, &testerapi.XYCoord{X: 6.1, Y: 7.6}).
		Expect(0)

	// --- Requests ---
	res, err := Svc.Request(ctx, pub.GET("https://"+Hostname+"/point-distance?p1.x=1&p1.y=1&p2.x=4&p2.y=5"))
	if assert.NoError(t, err) {
		var out testerapi.PointDistanceOut
		json.NewDecoder(res.Body).Decode(&out)
		assert.Equal(t, 5.0, out.D)
	}
	_, err = Svc.Request(ctx, pub.POST("https://"+Hostname+"/point-distance?p1.x=1&p1.y=1&p2.x=4&p2.y=5"))
	assert.Error(t, err)

	// --- Mock ---
	mock := NewMock()
	mock.SetHostname("point-distance.mock")
	mock.MockPointDistance(func(ctx context.Context, p1 testerapi.XYCoord, p2 *testerapi.XYCoord) (d float64, err error) {
		assert.Equal(t, testerapi.XYCoord{X: 1, Y: 1}, p1)
		assert.Equal(t, &testerapi.XYCoord{X: 4, Y: 5}, p2)
		return 5.0, nil
	})
	App.Join(mock)
	err = mock.Startup()
	assert.NoError(t, err)
	defer mock.Shutdown()

	res, err = Svc.Request(ctx, pub.GET("https://"+mock.Hostname()+"/point-distance?p1.x=1&p1.y=1&p2.x=4&p2.y=5"))
	if assert.NoError(t, err) {
		var out testerapi.PointDistanceOut
		json.NewDecoder(res.Body).Decode(&out)
		assert.Equal(t, 5.0, out.D)
	}
	_, err = Svc.Request(ctx, pub.POST("https://"+mock.Hostname()+"/point-distance?p1.x=1&p1.y=1&p2.x=4&p2.y=5"))
	assert.Error(t, err)

	// --- OpenAPI ---
	basePath := "paths|/" + Hostname + ":443/point-distance|get|"
	// Input argument p1 refers to XYCoors with its own x and y
	assert.Equal(t, "p1", openAPIValue(basePath+"parameters|0|name"))
	assert.Equal(t, "query", openAPIValue(basePath+"parameters|0|in"))
	schemaRef := openAPIValue(basePath + "parameters|0|schema|$ref").(string)
	schemaRef = strings.ReplaceAll(schemaRef, "/", "|")[2:] + "|"
	assert.Equal(t, "object", openAPIValue(schemaRef+"type"))
	assert.Equal(t, "number", openAPIValue(schemaRef+"properties|x|type"))
	assert.Equal(t, "number", openAPIValue(schemaRef+"properties|y|type"))
	// Input argument p2 refers to XYCoors with its own x and y
	assert.Equal(t, "p2", openAPIValue(basePath+"parameters|1|name"))
	assert.Equal(t, "query", openAPIValue(basePath+"parameters|1|in"))
	schemaRef = openAPIValue(basePath + "parameters|1|schema|$ref").(string)
	schemaRef = strings.ReplaceAll(schemaRef, "/", "|")[2:] + "|"
	assert.Equal(t, "object", openAPIValue(schemaRef+"type"))
	assert.Equal(t, "number", openAPIValue(schemaRef+"properties|x|type"))
	assert.Equal(t, "number", openAPIValue(schemaRef+"properties|y|type"))
	// Output argument d is an int
	schemaRef = openAPIValue(basePath + "responses|2XX|content|application/json|schema|$ref").(string)
	schemaRef = strings.ReplaceAll(schemaRef, "/", "|")[2:] + "|"
	assert.Equal(t, "object", openAPIValue(schemaRef+"type"))
	assert.Equal(t, "number", openAPIValue(schemaRef+"properties|d|type"))
}

func TestTester_ShiftPoint(t *testing.T) {
	t.Parallel()
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
	if assert.NoError(t, err) {
		var out testerapi.ShiftPointOut
		json.NewDecoder(res.Body).Decode(&out)
		assert.Equal(t, 15.0, out.Shifted.X)
		assert.Equal(t, 16.0, out.Shifted.Y)
	}

	// --- Mock ---
	mock := NewMock()
	mock.SetHostname("shift-point.mock")
	mock.MockShiftPoint(func(ctx context.Context, p *testerapi.XYCoord, x, y float64) (shifted *testerapi.XYCoord, err error) {
		if x == 0 && y == 0 {
			return nil, errors.New("zero")
		}
		return &testerapi.XYCoord{X: 88, Y: 99}, nil
	})
	App.Join(mock)
	err = mock.Startup()
	assert.NoError(t, err)
	defer mock.Shutdown()

	res, err = Svc.Request(ctx,
		pub.POST("https://"+mock.Hostname()+"/shift-point?x=10&y=10"),
		pub.Body(testerapi.ShiftPointIn{
			P: &testerapi.XYCoord{
				X: 5,
				Y: 6,
			},
		}))
	if assert.NoError(t, err) {
		var out testerapi.ShiftPointOut
		json.NewDecoder(res.Body).Decode(&out)
		assert.Equal(t, 88.0, out.Shifted.X)
		assert.Equal(t, 99.0, out.Shifted.Y)
	}

	_, err = Svc.Request(ctx,
		pub.POST("https://"+mock.Hostname()+"/shift-point"),
		pub.Body(testerapi.ShiftPointIn{
			P: &testerapi.XYCoord{
				X: 5,
				Y: 6,
			},
		}))
	assert.ErrorContains(t, err, "zero")

	// --- OpenAPI ---
	basePath := "paths|/" + Hostname + ":443/shift-point|post|"
	// Input arguments x and y are ints
	schemaRef := openAPIValue(basePath + "requestBody|content|application/json|schema|$ref").(string)
	schemaRef = strings.ReplaceAll(schemaRef, "/", "|")[2:] + "|"
	assert.Equal(t, "number", openAPIValue(schemaRef+"properties|x|type"))
	assert.Equal(t, "number", openAPIValue(schemaRef+"properties|y|type"))
	// Input argument p refers to XYCoors with its own x and y
	schemaRef = openAPIValue(schemaRef + "properties|p|$ref").(string)
	schemaRef = strings.ReplaceAll(schemaRef, "/", "|")[2:] + "|"
	assert.Equal(t, "number", openAPIValue(schemaRef+"properties|x|type"))
	assert.Equal(t, "number", openAPIValue(schemaRef+"properties|y|type"))
	// Output argument shifted also refers to XYCoors
	schemaRef = openAPIValue(basePath + "responses|2XX|content|application/json|schema|$ref").(string)
	schemaRef = strings.ReplaceAll(schemaRef, "/", "|")[2:] + "|"
	schemaRef = openAPIValue(schemaRef + "properties|shifted|$ref").(string)
	schemaRef = strings.ReplaceAll(schemaRef, "/", "|")[2:] + "|"
	assert.Equal(t, "object", openAPIValue(schemaRef+"type"))
	assert.Equal(t, "number", openAPIValue(schemaRef+"properties|x|type"))
	assert.Equal(t, "number", openAPIValue(schemaRef+"properties|y|type"))
}

func TestTester_SubArrayRange(t *testing.T) {
	t.Parallel()
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
	if assert.NoError(t, err) {
		assert.Equal(t, sub, []int{2, 3, 4})
		assert.Equal(t, http.StatusAccepted, status)
	}

	// --- Mock ---
	mock := NewMock()
	mock.SetHostname("sub-array-range.mock")
	mock.MockSubArrayRange(func(ctx context.Context, httpRequestBody []int, min, max int) (httpResponseBody []int, httpStatusCode int, err error) {
		assert.Equal(t, []int{1, 2, 3, 4, 5}, httpRequestBody)
		assert.Equal(t, 2, min)
		assert.Equal(t, 4, max)
		return []int{2, 3, 4}, http.StatusAccepted, nil
	})
	App.Join(mock)
	err = mock.Startup()
	assert.NoError(t, err)
	defer mock.Shutdown()

	res, err := Svc.Request(ctx, pub.POST("https://"+mock.Hostname()+"/sub-array-range/4?min=2"), pub.Body("[1,2,3,4,5]"), pub.ContentType("application/json"))
	if assert.NoError(t, err) {
		assert.Equal(t, http.StatusAccepted, res.StatusCode)
		var httpResponseBody []int
		json.NewDecoder(res.Body).Decode(&httpResponseBody)
		assert.Equal(t, []int{2, 3, 4}, httpResponseBody)
	}

	// --- OpenAPI ---
	basePath := "paths|/" + Hostname + ":443/sub-array-range/{max}|post|"
	// Argument pushed to query because of httpRequestBody
	assert.Equal(t, "min", openAPIValue(basePath+"parameters|0|name"))
	assert.Equal(t, "query", openAPIValue(basePath+"parameters|0|in"))
	// Argument indicated in path
	assert.Equal(t, "max", openAPIValue(basePath+"parameters|1|name"))
	assert.Equal(t, "path", openAPIValue(basePath+"parameters|1|in"))
	// httpRequestBody should not be listed as an argument
	assert.Len(t, openAPIValue(basePath+"parameters"), 2)
	// --- Requests --- schema is an array
	schemaRef := openAPIValue(basePath + "requestBody|content|application/json|schema|$ref").(string)
	schemaRef = strings.ReplaceAll(schemaRef, "/", "|")[2:] + "|"
	assert.Equal(t, "array", openAPIValue(schemaRef+"type"))
	assert.Equal(t, "integer", openAPIValue(schemaRef+"items|type"))
	// Response schema is an array
	schemaRef = openAPIValue(basePath + "responses|2XX|content|application/json|schema|$ref").(string)
	schemaRef = strings.ReplaceAll(schemaRef, "/", "|")[2:] + "|"
	assert.Equal(t, "array", openAPIValue(schemaRef+"type"))
	assert.Equal(t, "integer", openAPIValue(schemaRef+"items|type"))
}

func TestTester_WebPathArguments(t *testing.T) {
	t.Parallel()
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
}

func TestTester_FunctionPathArguments(t *testing.T) {
	t.Parallel()
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
	FunctionPathArguments(t, ctx, "[a&b/c]", "[d&e/f]", "[g&h/i]").
		Expect("[a&b/c] [d&e/f] [g&h/i]")
}

func TestTester_NonStringPathArguments(t *testing.T) {
	t.Parallel()
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
	assert.ErrorContains(t, err, "json")
	_, err = Svc.Request(ctx, pub.GET("https://"+Hostname+"/non-string-path-arguments/fixed/1/x/0.75"))
	assert.ErrorContains(t, err, "json")
	_, err = Svc.Request(ctx, pub.GET("https://"+Hostname+"/non-string-path-arguments/fixed/1/true/x"))
	assert.ErrorContains(t, err, "json")
	_, err = Svc.Request(ctx, pub.GET("https://"+Hostname+"/non-string-path-arguments/fixed/1/true/0.75"))
	assert.NoError(t, err)

	// --- OpenAPI ---
	basePath := "paths|/" + Hostname + ":443/non-string-path-arguments/fixed/{named}/{path2}/{suffix+}|get|"
	// named
	assert.Equal(t, "named", openAPIValue(basePath+"parameters|0|name"))
	assert.Equal(t, "path", openAPIValue(basePath+"parameters|0|in"))
	assert.Equal(t, "integer", openAPIValue(basePath+"parameters|0|schema|type"))
	// path2
	assert.Equal(t, "path2", openAPIValue(basePath+"parameters|1|name"))
	assert.Equal(t, "path", openAPIValue(basePath+"parameters|1|in"))
	assert.Equal(t, "boolean", openAPIValue(basePath+"parameters|1|schema|type"))
	// suffix
	assert.Equal(t, "suffix+", openAPIValue(basePath+"parameters|2|name"))
	assert.Equal(t, "path", openAPIValue(basePath+"parameters|2|in"))
	assert.Equal(t, "number", openAPIValue(basePath+"parameters|2|schema|type"))
}

func TestTester_UnnamedFunctionPathArguments(t *testing.T) {
	t.Parallel()
	/*
		ctx := Context()
		UnnamedFunctionPathArguments(t, ctx, path1, path2, path3).
			Expect(joined)
	*/

	ctx := Context()

	// --- Requests ---
	res, err := Svc.Request(ctx, pub.GET("https://"+Hostname+"/unnamed-function-path-arguments/x123/foo/y345/bar/z1/z2/z3"))
	assert.NoError(t, err)
	body, _ := io.ReadAll(res.Body)
	assert.Contains(t, string(body), "x123 y345 z1/z2/z3")

	// --- OpenAPI ---
	basePath := "paths|/" + Hostname + ":443/unnamed-function-path-arguments/{path1}/foo/{path2}/bar/{path3+}|get|"
	assert.Equal(t, "path1", openAPIValue(basePath+"parameters|0|name"))
	assert.Equal(t, "path", openAPIValue(basePath+"parameters|0|in"))
	assert.Equal(t, "path2", openAPIValue(basePath+"parameters|1|name"))
	assert.Equal(t, "path", openAPIValue(basePath+"parameters|1|in"))
	assert.Equal(t, "path3+", openAPIValue(basePath+"parameters|2|name"))
	assert.Equal(t, "path", openAPIValue(basePath+"parameters|2|in"))
}

func TestTester_UnnamedWebPathArguments(t *testing.T) {
	t.Parallel()
	/*
		ctx := Context()
		httpReq, _ := http.NewRequestWithContext(ctx, method, "?arg=val", body)
		UnnamedWebPathArguments(t, ctx, "").BodyContains(value)
		UnnamedWebPathArguments_Do(t, httpRequest).BodyContains(value)
	*/

	ctx := Context()

	// --- Requests ---
	res, err := Svc.Request(ctx, pub.GET("https://"+Hostname+"/unnamed-web-path-arguments/x123/foo/y345/bar/z1/z2/z3"))
	assert.NoError(t, err)
	body, _ := io.ReadAll(res.Body)
	assert.Contains(t, string(body), "x123 y345 z1/z2/z3")

	// --- OpenAPI ---
	basePath := "paths|/" + Hostname + ":443/unnamed-web-path-arguments/{path1}/foo/{path2}/bar/{path3+}|get|"
	assert.Equal(t, "path1", openAPIValue(basePath+"parameters|0|name"))
	assert.Equal(t, "path", openAPIValue(basePath+"parameters|0|in"))
	assert.Equal(t, "path2", openAPIValue(basePath+"parameters|1|name"))
	assert.Equal(t, "path", openAPIValue(basePath+"parameters|1|in"))
	assert.Equal(t, "path3+", openAPIValue(basePath+"parameters|2|name"))
	assert.Equal(t, "path", openAPIValue(basePath+"parameters|2|in"))
}

func TestTester_SumTwoIntegers(t *testing.T) {
	t.Parallel()
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
	if assert.NoError(t, err) {
		// The status code is not returned in the body but only through the status code field of the response
		assert.Equal(t, http.StatusAccepted, res.StatusCode)
		body, _ := io.ReadAll(res.Body)
		assert.Contains(t, string(body), "156")
		assert.NotContains(t, "httpStatusCode", string(body))
		assert.NotContains(t, strconv.Itoa(http.StatusAccepted), string(body))
	}
}

func TestTester_Echo(t *testing.T) {
	t.Parallel()
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

	// --- Requests ---
	httpReq, _ := http.NewRequestWithContext(ctx, "PUT", "?alpha=111&beta=222", strings.NewReader("HEAVY PAYLOAD"))
	httpReq.Header.Set("Content-Type", "text/plain")
	Echo(t, httpReq).
		BodyContains("PUT /").
		BodyContains("alpha=111&beta=222").
		BodyContains("text/plain").
		BodyContains("HEAVY PAYLOAD").
		NoError()

	res, err := Svc.Request(ctx, pub.PATCH("https://"+Hostname+"/echo?alpha=111&beta=222"), pub.Body("HEAVY PAYLOAD"), pub.ContentType("text/plain"))
	if assert.NoError(t, err) {
		body, _ := io.ReadAll(res.Body)
		assert.Contains(t, string(body), "PATCH /")
		assert.Contains(t, string(body), "alpha=111&beta=222")
		assert.Contains(t, string(body), "text/plain")
		assert.Contains(t, string(body), "HEAVY PAYLOAD")
	}
}

func TestTester_PathArgumentsPriority(t *testing.T) {
	t.Parallel()
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
	if assert.NoError(t, err) {
		b, _ := io.ReadAll(res.Body)
		assert.Contains(t, string(b), "BAR")
		assert.NotContains(t, string(b), "XYZ")
	}

	// If argument is not provided in the path, take from the query
	res, err = Svc.Request(ctx, pub.GET("https://"+Hostname+"/path-arguments-priority/{foo}?foo=BAR"))
	if assert.NoError(t, err) {
		b, _ := io.ReadAll(res.Body)
		assert.Contains(t, string(b), "BAR")
	}

	// Argument in the path should take priority over that in the body
	res, err = Svc.Request(ctx, pub.POST("https://"+Hostname+"/path-arguments-priority/BAR"), pub.Body(`{"foo":"XYZ"}`))
	if assert.NoError(t, err) {
		b, _ := io.ReadAll(res.Body)
		assert.Contains(t, string(b), "BAR")
		assert.NotContains(t, string(b), "XYZ")
	}

	// If argument is not provided in the path, take from the body
	res, err = Svc.Request(ctx, pub.POST("https://"+Hostname+"/path-arguments-priority/{foo}"), pub.Body(`{"foo":"BAR"}`))
	if assert.NoError(t, err) {
		b, _ := io.ReadAll(res.Body)
		assert.Contains(t, string(b), "BAR")
	}

	// If argument is not provided in the path, take from the query over the body
	res, err = Svc.Request(ctx, pub.POST("https://"+Hostname+"/path-arguments-priority/{foo}?foo=BAR"), pub.Body(`{"foo":"XYZ"}`))
	if assert.NoError(t, err) {
		b, _ := io.ReadAll(res.Body)
		assert.Contains(t, string(b), "BAR")
		assert.NotContains(t, string(b), "XYZ")
	}
}

func TestTester_DirectoryServer(t *testing.T) {
	t.Parallel()
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
	if assert.NoError(t, err) {
		b, _ := io.ReadAll(res.Body)
		assert.Contains(t, string(b), "111")
	}
	res, err = testerapi.NewClient(Svc).DirectoryServer(ctx, "sub/2.txt")
	if assert.NoError(t, err) {
		b, _ := io.ReadAll(res.Body)
		assert.Contains(t, string(b), "222")
	}
	_, err = testerapi.NewClient(Svc).DirectoryServer(ctx, "../3.txt")
	assert.Error(t, err)
	httpReq, _ = http.NewRequestWithContext(ctx, "POST", "1.txt", strings.NewReader("Payload"))
	_, err = testerapi.NewClient(Svc).DirectoryServer_Do(ctx, httpReq)
	assert.Error(t, err)

	// --- Requests ---
	res, err = Svc.Request(ctx, pub.GET("https://"+Hostname+"/directory-server/1.txt"))
	if assert.NoError(t, err) {
		b, _ := io.ReadAll(res.Body)
		assert.Contains(t, string(b), "111")
	}
	res, err = Svc.Request(ctx, pub.GET("https://"+Hostname+"/directory-server/sub/2.txt"))
	if assert.NoError(t, err) {
		b, _ := io.ReadAll(res.Body)
		assert.Contains(t, string(b), "222")
	}
	_, err = Svc.Request(ctx, pub.GET("https://"+Hostname+"/directory-server/../3.txt"))
	assert.Error(t, err)
	_, err = Svc.Request(ctx, pub.POST("https://"+Hostname+"/directory-server/1.txt"))
	assert.Error(t, err)

	// --- Mock ---
	mock := NewMock()
	mock.SetHostname("directory-server.mock")
	mock.MockDirectoryServer(func(w http.ResponseWriter, r *http.Request) (err error) {
		w.Write([]byte("111"))
		return nil
	})
	App.Join(mock)
	err = mock.Startup()
	assert.NoError(t, err)
	defer mock.Shutdown()

	res, err = Svc.Request(ctx, pub.GET("https://"+mock.Hostname()+"/directory-server/1.txt"))
	if assert.NoError(t, err) {
		b, _ := io.ReadAll(res.Body)
		assert.Contains(t, string(b), "111")
	}
	_, err = Svc.Request(ctx, pub.POST("https://"+mock.Hostname()+"/directory-server/1.txt"))
	assert.Error(t, err)

	// --- OpenAPI ---
	basePath := "paths|/" + Hostname + ":443/directory-server/{filename+}|get|"
	assert.Equal(t, "filename+", openAPIValue(basePath+"parameters|0|name"))
	assert.Equal(t, "path", openAPIValue(basePath+"parameters|0|in"))
}

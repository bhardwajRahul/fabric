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

package openapi

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/microbus-io/testarossa"
)

// resolveSchema follows a $ref in an OpenAPI schema map to find the actual schema with properties.
func resolveSchema(schemas map[string]any, name string) map[string]any {
	schema := schemas[name].(map[string]any)
	if ref, ok := schema["$ref"].(string); ok {
		// Extract the schema name from "#/components/schemas/Name"
		parts := strings.Split(ref, "/")
		refName := parts[len(parts)-1]
		return schemas[refName].(map[string]any)
	}
	return schema
}

func TestParseParamDescriptions(t *testing.T) {
	t.Parallel()

	t.Run("full", func(t *testing.T) {
		assert := testarossa.For(t)
		desc := `Forecast returns the weather forecast for a location.

Input:
  - city: The city name, e.g. "San Francisco"
  - days: Number of days to forecast, 1-14

Output:
  - forecast: Daily forecast summaries
  - confidence: Model confidence score, 0.0 to 1.0`

		params, results := parseParamDescriptions(desc)
		assert.Expect(params["city"], `The city name, e.g. "San Francisco"`)
		assert.Expect(params["days"], "Number of days to forecast, 1-14")
		assert.Expect(results["forecast"], "Daily forecast summaries")
		assert.Expect(results["confidence"], "Model confidence score, 0.0 to 1.0")
	})

	t.Run("no_sections", func(t *testing.T) {
		assert := testarossa.For(t)
		desc := "Simple description with no parameter docs."

		params, results := parseParamDescriptions(desc)
		assert.Expect(len(params), 0)
		assert.Expect(len(results), 0)
	})

	t.Run("params_only", func(t *testing.T) {
		assert := testarossa.For(t)
		desc := `DoSomething does something.

Input:
  - name: The name to use`

		params, results := parseParamDescriptions(desc)
		assert.Expect(params["name"], "The name to use")
		assert.Expect(len(results), 0)
	})

	t.Run("empty", func(t *testing.T) {
		assert := testarossa.For(t)
		params, results := parseParamDescriptions("")
		assert.Expect(len(params), 0)
		assert.Expect(len(results), 0)
	})
}

func TestRender_ParamDescriptions(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	type ForecastIn struct {
		City string `json:"city,omitzero"`
		Days int    `json:"days,omitzero"`
	}
	type ForecastOut struct {
		Forecast   string  `json:"forecast,omitzero"`
		Confidence float64 `json:"confidence,omitzero"`
	}

	svc := &Service{
		ServiceName: "weather.test",
		Endpoints: []*Endpoint{
			{
				Type:    "function",
				Name:    "Forecast",
				Method:  "GET",
				Route:   "/forecast",
				Summary: "Forecast(city string, days int) (forecast string, confidence float64)",
				Description: `Forecast returns the weather forecast.

Input:
  - city: The city name
  - days: Number of days to forecast

Output:
  - forecast: Daily forecast summaries
  - confidence: Model confidence score`,
				InputArgs:  ForecastIn{},
				OutputArgs: ForecastOut{},
			},
		},
	}

	data, err := json.Marshal(Render(svc))
	if !assert.NoError(err) {
		return
	}

	// Parse the JSON to verify descriptions are present
	var doc map[string]any
	err = json.Unmarshal(data, &doc)
	if !assert.NoError(err) {
		return
	}

	// GET method: parameters are query args
	paths := doc["paths"].(map[string]any)
	path := paths["/weather.test/forecast"].(map[string]any)
	op := path["get"].(map[string]any)
	params := op["parameters"].([]any)

	// Find city and days parameters
	var cityDesc, daysDesc string
	for _, p := range params {
		param := p.(map[string]any)
		switch param["name"] {
		case "city":
			cityDesc, _ = param["description"].(string)
		case "days":
			daysDesc, _ = param["description"].(string)
		}
	}
	assert.Expect(cityDesc, "The city name")
	assert.Expect(daysDesc, "Number of days to forecast")

	// Verify x-feature-type is present
	assert.Expect(op["x-feature-type"], "function")

	// Check output schema descriptions (follow $ref if needed)
	components := doc["components"].(map[string]any)
	schemas := components["schemas"].(map[string]any)
	outSchema := resolveSchema(schemas, "weather_test__Forecast_OUT")
	outProps := outSchema["properties"].(map[string]any)
	forecastProp := outProps["forecast"].(map[string]any)
	confidenceProp := outProps["confidence"].(map[string]any)
	assert.Expect(forecastProp["description"], "Daily forecast summaries")
	assert.Expect(confidenceProp["description"], "Model confidence score")
}

func TestRender_ParamDescriptions_POST(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	type CreateIn struct {
		Name  string `json:"name,omitzero"`
		Email string `json:"email,omitzero"`
	}
	type CreateOut struct {
		ID string `json:"id,omitzero"`
	}

	svc := &Service{
		ServiceName: "user.test",
		Endpoints: []*Endpoint{
			{
				Type:    "function",
				Name:    "Create",
				Method:  "POST",
				Route:   "/create",
				Summary: "Create(name string, email string) (id string)",
				Description: `Create creates a new user.

Input:
  - name: The user's display name
  - email: The user's email address

Output:
  - id: The generated user ID`,
				InputArgs:  CreateIn{},
				OutputArgs: CreateOut{},
			},
		},
	}

	data, err := json.Marshal(Render(svc))
	if !assert.NoError(err) {
		return
	}

	var doc map[string]any
	err = json.Unmarshal(data, &doc)
	if !assert.NoError(err) {
		return
	}

	// POST method: parameters are in request body schema
	components := doc["components"].(map[string]any)
	schemas := components["schemas"].(map[string]any)

	inSchema := resolveSchema(schemas, "user_test__Create_IN")
	inProps := inSchema["properties"].(map[string]any)
	nameProp := inProps["name"].(map[string]any)
	emailProp := inProps["email"].(map[string]any)
	assert.Expect(nameProp["description"], "The user's display name")
	assert.Expect(emailProp["description"], "The user's email address")

	outSchema := resolveSchema(schemas, "user_test__Create_OUT")
	outProps := outSchema["properties"].(map[string]any)
	idProp := outProps["id"].(map[string]any)
	assert.Expect(idProp["description"], "The generated user ID")
}

func TestRender_JsonSchemaTags(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	type Location struct {
		Lat  float64 `json:"lat" jsonschema:"description=Latitude in decimal degrees"`
		Long float64 `json:"long" jsonschema:"description=Longitude in decimal degrees"`
	}
	type SearchIn struct {
		Location Location `json:"location,omitzero"`
	}
	type SearchOut struct {
		Found bool `json:"found,omitzero"`
	}

	svc := &Service{
		ServiceName: "geo.test",
		Endpoints: []*Endpoint{
			{
				Type:       "function",
				Name:       "Search",
				Method:     "POST",
				Route:      "/search",
				Summary:    "Search(location Location) (found bool)",
				InputArgs:  SearchIn{},
				OutputArgs: SearchOut{},
			},
		},
	}

	data, err := json.Marshal(Render(svc))
	if !assert.NoError(err) {
		return
	}

	var doc map[string]any
	err = json.Unmarshal(data, &doc)
	if !assert.NoError(err) {
		return
	}

	// The Location type should be in components/schemas with field descriptions from jsonschema tags
	components := doc["components"].(map[string]any)
	schemas := components["schemas"].(map[string]any)
	locSchema := schemas["geo_test__Search_IN_Location"].(map[string]any)
	locProps := locSchema["properties"].(map[string]any)
	latProp := locProps["lat"].(map[string]any)
	longProp := locProps["long"].(map[string]any)
	assert.Expect(latProp["description"], "Latitude in decimal degrees")
	assert.Expect(longProp["description"], "Longitude in decimal degrees")
}

func TestRender_GreedyPath(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	type LoadObjectIn struct {
		Category int    `json:"category,omitzero"`
		Name     string `json:"name,omitzero"`
	}
	type LoadObjectOut struct {
		Found bool `json:"found,omitzero"`
	}

	svc := &Service{
		ServiceName: "objects.test",
		Endpoints: []*Endpoint{
			{
				Type:       "function",
				Name:       "LoadObject",
				Method:     "GET",
				Route:      "/load/{category}/{name...}",
				InputArgs:  LoadObjectIn{},
				OutputArgs: LoadObjectOut{},
			},
		},
	}

	data, err := json.Marshal(Render(svc))
	if !assert.NoError(err) {
		return
	}

	var doc map[string]any
	err = json.Unmarshal(data, &doc)
	if !assert.NoError(err) {
		return
	}

	// The greedy "..." suffix must not leak into the rendered path or parameter name.
	paths := doc["paths"].(map[string]any)
	_, hasClean := paths["/objects.test/load/{category}/{name}"]
	assert.True(hasClean, "rendered path should be /objects.test/load/{category}/{name}, got: %v", paths)

	// Both category and name must be path parameters; nothing should be a query parameter.
	op := paths["/objects.test/load/{category}/{name}"].(map[string]any)["get"].(map[string]any)
	params := op["parameters"].([]any)
	gotIn := map[string]string{}
	for _, p := range params {
		param := p.(map[string]any)
		name, _ := param["name"].(string)
		in, _ := param["in"].(string)
		gotIn[name] = in
	}
	assert.Expect(gotIn["category"], "path")
	assert.Expect(gotIn["name"], "path")
	_, nameDotted := gotIn["name..."]
	assert.False(nameDotted, "parameter name must not include the trailing dots")
}

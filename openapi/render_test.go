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

// TestRender_ParamDescriptions covers the GET case where descriptions on scalar query parameters come
// from the In struct fields' jsonschema tags. A query parameter is reflected from its field type alone,
// so the tag is read directly by fieldTagDescription rather than via struct reflection.
func TestRender_ParamDescriptions(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	type ForecastIn struct {
		City string `json:"city,omitzero" jsonschema_description:"The city name"`
		Days int    `json:"days,omitzero" jsonschema_description:"Number of days to forecast"`
	}
	type ForecastOut struct {
		Forecast   string  `json:"forecast,omitzero" jsonschema_description:"Daily forecast summaries"`
		Confidence float64 `json:"confidence,omitzero" jsonschema_description:"Model confidence score"`
	}

	svc := &Service{
		ServiceName: "weather.test",
		Endpoints: []*Endpoint{
			{
				Type:       "function",
				Name:       "Forecast",
				Method:     "GET",
				Route:      "/forecast",
				Summary:    "Forecast(city string, days int) (forecast string, confidence float64)",
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
		Name  string `json:"name,omitzero" jsonschema_description:"The user's display name"`
		Email string `json:"email,omitzero" jsonschema_description:"The user's email address"`
	}
	type CreateOut struct {
		ID string `json:"id,omitzero" jsonschema_description:"The generated user ID"`
	}

	svc := &Service{
		ServiceName: "user.test",
		Endpoints: []*Endpoint{
			{
				Type:       "function",
				Name:       "Create",
				Method:     "POST",
				Route:      "/create",
				Summary:    "Create(name string, email string) (id string)",
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

// TestRender_MagicBodyDescriptions covers the magic HTTP body case. A jsonschema_description tag on the
// HTTPRequestBody / HTTPResponseBody field is dropped by struct reflection (the body schema is reflected
// from the field's type alone), so it is read directly and set on the requestBody / response nodes.
func TestRender_MagicBodyDescriptions(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	type Payload struct {
		Value string `json:"value,omitzero"`
	}
	type CreateIn struct {
		HTTPRequestBody *Payload `json:"-" jsonschema_description:"The object to create"`
	}
	type CreateOut struct {
		HTTPResponseBody *Payload `json:"-" jsonschema_description:"The created object"`
		HTTPStatusCode   int      `json:"-"`
	}

	svc := &Service{
		ServiceName: "store.test",
		Endpoints: []*Endpoint{
			{
				Type:       "function",
				Name:       "Create",
				Method:     "POST",
				Route:      "/create",
				Summary:    "Create(httpRequestBody *Payload) (httpResponseBody *Payload, httpStatusCode int)",
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

	paths := doc["paths"].(map[string]any)
	op := paths["/store.test/create"].(map[string]any)["post"].(map[string]any)

	// Request body description comes from the HTTPRequestBody field tag
	reqBody := op["requestBody"].(map[string]any)
	assert.Expect(reqBody["description"], "The object to create")

	// Response description comes from the HTTPResponseBody field tag, replacing the "OK" default
	responses := op["responses"].(map[string]any)
	resp := responses["2XX"].(map[string]any)
	assert.Expect(resp["description"], "The created object")
}

func TestRender_JsonSchemaTags(t *testing.T) {
	t.Parallel()
	assert := testarossa.For(t)

	type Location struct {
		Lat  float64 `json:"lat" jsonschema_description:"Latitude in decimal degrees"`
		Long float64 `json:"long" jsonschema_description:"Longitude in decimal degrees"`
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

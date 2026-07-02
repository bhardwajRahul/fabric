package weather

import (
	"context"
	"io"
	"net/http"
	"regexp"
	"slices"
	"testing"

	"github.com/golang-jwt/jwt/v5"

	"github.com/microbus-io/dwarf/workflow"
	"github.com/microbus-io/errors"
	"github.com/microbus-io/fabric/application"
	"github.com/microbus-io/fabric/connector"
	"github.com/microbus-io/fabric/coreservices/foreman"
	"github.com/microbus-io/fabric/coreservices/foreman/foremanapi"
	"github.com/microbus-io/fabric/coreservices/llm"
	"github.com/microbus-io/fabric/coreservices/llm/llmapi"
	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/fabric/pub"
	"github.com/microbus-io/fabric/sub"
	"github.com/microbus-io/testarossa"

	"github.com/microbus-io/fabric/exampleservices/weather/weatherapi"
)

var (
	_ context.Context
	_ io.Reader
	_ *http.Request
	_ *regexp.Regexp
	_ *testing.T
	_ jwt.MapClaims
	_ application.Application
	_ connector.Connector
	_ frame.Frame
	_ httpx.BodyReader
	_ pub.Option
	_ sub.Option
	_ *errors.TracedError
	_ *workflow.Flow
	_ testarossa.Asserter
	_ = weatherapi.Hostname
)

func TestWeather_LatLng(t *testing.T) { // MARKER: LatLng
	t.Parallel()
	ctx := t.Context()

	svc := NewService()

	tester := connector.New("tester.client")
	client := weatherapi.NewClient(tester)

	app := application.New()
	app.Add(
		svc,
		tester,
	)
	app.RunInTest(t)

	t.Run("known_city", func(t *testing.T) {
		assert := testarossa.For(t)

		lat, lng, err := client.LatLng(ctx, "London")
		assert.Expect(
			lat, 51.51,
			lng, -0.13,
			err, nil,
		)
	})

	t.Run("known_city_with_qualifier", func(t *testing.T) {
		assert := testarossa.For(t)

		lat, lng, err := client.LatLng(ctx, "London, UK")
		assert.Expect(
			lat, 51.51,
			lng, -0.13,
			err, nil,
		)
	})

	t.Run("unknown_location_is_deterministic", func(t *testing.T) {
		assert := testarossa.For(t)

		lat1, lng1, err := client.LatLng(ctx, "Nowheresville")
		assert.NoError(err)
		lat2, lng2, err := client.LatLng(ctx, "Nowheresville")
		assert.NoError(err)
		assert.Equal(lat1, lat2)
		assert.Equal(lng1, lng2)
		assert.True(lat1 >= -60 && lat1 <= 70)
		assert.True(lng1 >= -180 && lng1 <= 180)
	})
}

func TestWeather_Forecast(t *testing.T) { // MARKER: Forecast
	t.Parallel()
	ctx := t.Context()

	svc := NewService()

	tester := connector.New("tester.client")
	client := weatherapi.NewClient(tester)

	app := application.New()
	app.Add(
		svc,
		tester,
	)
	app.RunInTest(t)

	t.Run("deterministic_and_in_range", func(t *testing.T) {
		assert := testarossa.For(t)

		summary1, temp1, precip1, err := client.Forecast(ctx, 51.51, -0.13)
		assert.NoError(err)
		summary2, temp2, precip2, err := client.Forecast(ctx, 51.51, -0.13)
		assert.Expect(
			summary2, summary1,
			temp2, temp1,
			precip2, precip1,
			err, nil,
		)
		assert.NotEqual(summary1, "")
		assert.True(precip1 >= 0 && precip1 <= 100)
	})
}

func TestWeather_AskAgent(t *testing.T) { // MARKER: AskAgent
	t.Parallel()
	ctx := t.Context()

	svc := NewService()

	tester := connector.New("tester.client")
	foremanClient := foremanapi.NewClient(tester)
	exec := weatherapi.NewExecutor(tester).WithWorkflowRunner(foremanClient)

	// Mock llm.core's ChatLoop so the workflow runs without a provider key. The mock stands in for the
	// multi-step chat subgraph, capturing the offered tools and returning a canned assistant answer.
	var offeredTools []string
	llmMock := llm.NewMock()
	llmMock.MockChatLoop(func(ctx context.Context, flow *workflow.Flow, provider string, model string, items []llmapi.Item, toolURLs []string, options *llmapi.ChatOptions) (itemsOut []llmapi.Item, usage llmapi.Usage, err error) {
		offeredTools = toolURLs
		itemsOut = append(items, llmapi.NewMessage("assistant", "It is sunny and 21C in London.").AsItem())
		return itemsOut, llmapi.Usage{}, nil
	})

	app := application.New()
	app.Add(
		svc,
		foreman.NewService(),
		llmMock,
		tester,
	)
	app.RunInTest(t)

	t.Run("returns_final_assistant_message", func(t *testing.T) {
		assert := testarossa.For(t)

		answer, status, err := exec.AskAgent(ctx, "What is the weather in London?")
		assert.Expect(
			err, nil,
			status, workflow.StatusCompleted,
			answer, "It is sunny and 21C in London.",
		)
		assert.True(slices.Contains(offeredTools, weatherapi.LatLng.URL()))
		assert.True(slices.Contains(offeredTools, weatherapi.Forecast.URL()))
	})
}

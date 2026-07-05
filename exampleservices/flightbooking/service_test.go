package flightbooking

import (
	"context"
	"io"
	"net/http"
	"regexp"
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

	"github.com/microbus-io/fabric/exampleservices/flightbooking/flightbookingapi"
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
	_ = flightbookingapi.Hostname
)

func TestFlightBooking_SearchFlights(t *testing.T) { // MARKER: SearchFlights
	t.Parallel()
	ctx := t.Context()

	svc := NewService()

	tester := connector.New("tester.client")
	exec := flightbookingapi.NewExecutor(tester)

	app := application.New()
	app.Add(
		svc,
		tester,
	)
	app.RunInTest(t)

	t.Run("known_route", func(t *testing.T) {
		assert := testarossa.For(t)

		candidates, flightIndex, err := exec.SearchFlights(ctx, "San Francisco", "London")
		assert.NoError(err)
		assert.Equal(flightIndex, 0)
		assert.True(len(candidates) >= 2)
		assert.Equal(candidates[0].Origin, "San Francisco")
		assert.Equal(candidates[0].Destination, "London")
	})

	t.Run("unknown_route_is_empty", func(t *testing.T) {
		assert := testarossa.For(t)

		candidates, _, err := exec.SearchFlights(ctx, "Nowhere", "Elsewhere")
		assert.NoError(err)
		assert.Equal(len(candidates), 0)
	})
}

func TestFlightBooking_ProposeFlight(t *testing.T) { // MARKER: ProposeFlight
	t.Parallel()
	ctx := t.Context()

	svc := NewService()

	tester := connector.New("tester.client")
	exec := flightbookingapi.NewExecutor(tester)

	app := application.New()
	app.Add(
		svc,
		tester,
	)
	app.RunInTest(t)

	candidates := []flightbookingapi.Flight{
		{Airline: "Oceanic", FlightNo: "OA815"},
		{Airline: "Meridian", FlightNo: "ME221"},
	}

	t.Run("in_range", func(t *testing.T) {
		assert := testarossa.For(t)

		current, exhausted, err := exec.ProposeFlight(ctx, candidates, 1)
		assert.NoError(err)
		assert.False(exhausted)
		assert.Equal(current.FlightNo, "ME221")
	})

	t.Run("past_end_is_exhausted", func(t *testing.T) {
		assert := testarossa.For(t)

		_, exhausted, err := exec.ProposeFlight(ctx, candidates, 2)
		assert.NoError(err)
		assert.True(exhausted)
	})
}

// mockSeatAgent mocks llm.core's ChatLoop so seat selection runs without a provider key.
func mockSeatAgent(seat string) *llm.Mock {
	llmMock := llm.NewMock()
	llmMock.MockChatLoop(func(ctx context.Context, flow *workflow.Flow, provider string, model string, items []llmapi.Item, toolURLs []string, options *llmapi.ChatOptions) (itemsOut []llmapi.Item, usage llmapi.Usage, err error) {
		itemsOut = append(items, llmapi.NewMessage("assistant", seat).AsItem())
		return itemsOut, llmapi.Usage{}, nil
	})
	return llmMock
}

func TestFlightBooking_BookFlight_AcceptFirst(t *testing.T) { // MARKER: BookFlight
	t.Parallel()
	ctx := t.Context()

	svc := NewService()

	tester := connector.New("tester.client")
	foremanClient := foremanapi.NewClient(tester)

	app := application.New()
	app.Add(
		svc,
		foreman.NewService(),
		mockSeatAgent("14A"),
		tester,
	)
	app.RunInTest(t)

	assert := testarossa.For(t)

	in := flightbookingapi.BookFlightIn{Origin: "San Francisco", Destination: "London", SeatPreference: "a window seat"}
	flowKey, err := foremanClient.Create(ctx, flightbookingapi.BookFlight.URL(), in, nil)
	assert.NoError(err)

	// The flow parks on the first proposed flight.
	outcome, err := foremanClient.Await(ctx, flowKey)
	assert.NoError(err)
	assert.Equal(outcome.Status, workflow.StatusInterrupted)
	f, ok := proposedFlight(outcome)
	assert.True(ok)
	assert.Equal(f.Destination, "London")

	// Accept it; the flow chooses a seat via the child agent and confirms the booking.
	err = foremanClient.Resume(ctx, flowKey, map[string]any{"accepted": true})
	assert.NoError(err)
	outcome, err = foremanClient.Await(ctx, flowKey)
	assert.NoError(err)
	assert.Equal(outcome.Status, workflow.StatusCompleted)

	var out flightbookingapi.BookFlightOut
	_, err = foremanClient.SnapshotAndParse(ctx, flowKey, &out)
	assert.NoError(err)
	assert.Equal(out.Seat, "14A")
	assert.NotEqual(out.Airline, "")
	assert.NotEqual(out.Confirmation, "")
}

func TestFlightBooking_BookFlight_KeepSearching(t *testing.T) { // MARKER: BookFlight
	t.Parallel()
	ctx := t.Context()

	svc := NewService()

	tester := connector.New("tester.client")
	foremanClient := foremanapi.NewClient(tester)

	app := application.New()
	app.Add(
		svc,
		foreman.NewService(),
		mockSeatAgent("7A"),
		tester,
	)
	app.RunInTest(t)

	assert := testarossa.For(t)

	in := flightbookingapi.BookFlightIn{Origin: "San Francisco", Destination: "London", SeatPreference: "an aisle seat"}
	flowKey, err := foremanClient.Create(ctx, flightbookingapi.BookFlight.URL(), in, nil)
	assert.NoError(err)

	outcome, err := foremanClient.Await(ctx, flowKey)
	assert.NoError(err)
	assert.Equal(outcome.Status, workflow.StatusInterrupted)
	first, _ := proposedFlight(outcome)

	// Keep searching: the flow loops back and proposes a different flight.
	err = foremanClient.Resume(ctx, flowKey, map[string]any{"accepted": false})
	assert.NoError(err)
	outcome, err = foremanClient.Await(ctx, flowKey)
	assert.NoError(err)
	assert.Equal(outcome.Status, workflow.StatusInterrupted)
	second, _ := proposedFlight(outcome)
	assert.NotEqual(second.FlightNo, first.FlightNo)

	// Accept the second candidate.
	err = foremanClient.Resume(ctx, flowKey, map[string]any{"accepted": true})
	assert.NoError(err)
	outcome, err = foremanClient.Await(ctx, flowKey)
	assert.NoError(err)
	assert.Equal(outcome.Status, workflow.StatusCompleted)
}

func TestFlightBooking_BookFlight_NoFlights(t *testing.T) { // MARKER: BookFlight
	t.Parallel()
	ctx := t.Context()

	svc := NewService()

	tester := connector.New("tester.client")
	foremanClient := foremanapi.NewClient(tester)

	app := application.New()
	app.Add(
		svc,
		foreman.NewService(),
		tester,
	)
	app.RunInTest(t)

	assert := testarossa.For(t)

	in := flightbookingapi.BookFlightIn{Origin: "Nowhere", Destination: "Elsewhere"}
	var out flightbookingapi.BookFlightOut
	outcome, err := foremanClient.RunAndParse(ctx, flightbookingapi.BookFlight.URL(), in, nil, &out)
	assert.NoError(err)
	assert.Equal(outcome.Status, workflow.StatusCompleted)
	assert.Equal(out.Airline, "")
	assert.NotEqual(out.Confirmation, "")
}

func TestFlightBooking_Demo(t *testing.T) { // MARKER: Demo
	t.Parallel()
	ctx := t.Context()
	_ = ctx

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the tester client
	tester := connector.New("tester.client")
	client := flightbookingapi.NewClient(tester)
	_ = client

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	/*
		HINT: Fill in test cases using the following pattern

		t.Run("test_case_name", func(t *testing.T) {
			assert := testarossa.For(t)

			res, err := client.Demo(ctx, "GET", "", nil)
			if assert.NoError(err) {
				assert.Expect(res.StatusCode, http.StatusOK)
			}
		})
	*/
}

func TestFlightBooking_AwaitDecision(t *testing.T) { // MARKER: AwaitDecision
	t.Parallel()
	ctx := t.Context()
	_ = ctx

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the tester
	tester := connector.New("tester.client")
	exec := flightbookingapi.NewExecutor(tester)
	_ = exec

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	/*
		HINT: Fill in test cases using the following pattern.
		Use WithOutputFlow to also verify control signals (Goto, Retry, Interrupt, Sleep) if applicable.

		t.Run("test_case_name", func(t *testing.T) {
			assert := testarossa.For(t)

			accepted, flightIndexOut, err := exec.AwaitDecision(ctx, currentFlight, flightIndex)
			assert.Expect(
				accepted, expectedAccepted,
				flightIndexOut, expectedFlightIndexOut,
				err, nil,
			)
		})
	*/
}

func TestFlightBooking_ChooseSeat(t *testing.T) { // MARKER: ChooseSeat
	t.Parallel()
	ctx := t.Context()
	_ = ctx

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the tester
	tester := connector.New("tester.client")
	exec := flightbookingapi.NewExecutor(tester)
	_ = exec

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	/*
		HINT: Fill in test cases using the following pattern.
		Use WithOutputFlow to also verify control signals (Goto, Retry, Interrupt, Sleep) if applicable.

		t.Run("test_case_name", func(t *testing.T) {
			assert := testarossa.For(t)

			seat, err := exec.ChooseSeat(ctx, seatPreference, currentFlight)
			assert.Expect(
				seat, expectedSeat,
				err, nil,
			)
		})
	*/
}

func TestFlightBooking_ConfirmBooking(t *testing.T) { // MARKER: ConfirmBooking
	t.Parallel()
	ctx := t.Context()
	_ = ctx

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the tester
	tester := connector.New("tester.client")
	exec := flightbookingapi.NewExecutor(tester)
	_ = exec

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	/*
		HINT: Fill in test cases using the following pattern.
		Use WithOutputFlow to also verify control signals (Goto, Retry, Interrupt, Sleep) if applicable.

		t.Run("test_case_name", func(t *testing.T) {
			assert := testarossa.For(t)

			confirmation, airline, flightNo, err := exec.ConfirmBooking(ctx, currentFlight, seat)
			assert.Expect(
				confirmation, expectedConfirmation,
				airline, expectedAirline,
				flightNo, expectedFlightNo,
				err, nil,
			)
		})
	*/
}

func TestFlightBooking_NoFlights(t *testing.T) { // MARKER: NoFlights
	t.Parallel()
	ctx := t.Context()
	_ = ctx

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the tester
	tester := connector.New("tester.client")
	exec := flightbookingapi.NewExecutor(tester)
	_ = exec

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	/*
		HINT: Fill in test cases using the following pattern.
		Use WithOutputFlow to also verify control signals (Goto, Retry, Interrupt, Sleep) if applicable.

		t.Run("test_case_name", func(t *testing.T) {
			assert := testarossa.For(t)

			confirmation, err := exec.NoFlights(ctx)
			assert.Expect(
				confirmation, expectedConfirmation,
				err, nil,
			)
		})
	*/
}

func TestFlightBooking_PickSeat(t *testing.T) { // MARKER: PickSeat
	t.Parallel()
	ctx := t.Context()
	_ = ctx

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the tester
	tester := connector.New("tester.client")
	exec := flightbookingapi.NewExecutor(tester)
	_ = exec

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		tester,
	)
	app.RunInTest(t)

	/*
		HINT: Fill in test cases using the following pattern.
		Use WithOutputFlow to also verify control signals (Goto, Retry, Interrupt, Sleep) if applicable.

		t.Run("test_case_name", func(t *testing.T) {
			assert := testarossa.For(t)

			seat, err := exec.PickSeat(ctx, seatPreference, availableSeats)
			assert.Expect(
				seat, expectedSeat,
				err, nil,
			)
		})
	*/
}

func TestFlightBooking_BookFlight(t *testing.T) { // MARKER: BookFlight
	t.Parallel()
	ctx := t.Context()
	_ = ctx

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("tester.client")
	foremanClient := foremanapi.NewClient(tester)
	exec := flightbookingapi.NewExecutor(tester).WithWorkflowRunner(foremanClient)
	_ = exec

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		foreman.NewService(),
		tester,
	)
	app.RunInTest(t)

	/*
		HINT: Fill in test cases using the following pattern

		t.Run("test_case_name", func(t *testing.T) {
			assert := testarossa.For(t)

			confirmation, airline, flightNo, seat, status, err := exec.BookFlight(ctx, origin, destination, seatPreference)
			assert.Expect(
				err, nil,
				status, workflow.StatusCompleted,
				confirmation, expectedConfirmation,
				airline, expectedAirline,
				flightNo, expectedFlightNo,
				seat, expectedSeat,
			)
		})
	*/
}

func TestFlightBooking_ChooseSeatAgent(t *testing.T) { // MARKER: ChooseSeatAgent
	t.Parallel()
	ctx := t.Context()
	_ = ctx

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the testers
	tester := connector.New("tester.client")
	foremanClient := foremanapi.NewClient(tester)
	exec := flightbookingapi.NewExecutor(tester).WithWorkflowRunner(foremanClient)
	_ = exec

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
		foreman.NewService(),
		tester,
	)
	app.RunInTest(t)

	/*
		HINT: Fill in test cases using the following pattern

		t.Run("test_case_name", func(t *testing.T) {
			assert := testarossa.For(t)

			seat, status, err := exec.ChooseSeatAgent(ctx, seatPreference, availableSeats)
			assert.Expect(
				err, nil,
				status, workflow.StatusCompleted,
				seat, expectedSeat,
			)
		})
	*/
}

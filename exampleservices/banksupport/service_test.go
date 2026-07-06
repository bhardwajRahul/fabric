package banksupport

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
	"github.com/microbus-io/fabric/coreservices/accesstoken"
	"github.com/microbus-io/fabric/coreservices/accesstoken/accesstokenapi"
	"github.com/microbus-io/fabric/coreservices/bearertoken"
	"github.com/microbus-io/fabric/coreservices/foreman"
	"github.com/microbus-io/fabric/coreservices/foreman/foremanapi"
	"github.com/microbus-io/fabric/coreservices/llm"
	"github.com/microbus-io/fabric/coreservices/llm/llmapi"
	"github.com/microbus-io/fabric/exampleservices/banksupport/banksupportapi"
	"github.com/microbus-io/fabric/frame"
	"github.com/microbus-io/fabric/httpx"
	"github.com/microbus-io/fabric/pub"
	"github.com/microbus-io/fabric/sub"
	"github.com/microbus-io/testarossa"
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
	_ = banksupportapi.Hostname
)

// customerToken mints an access token identifying a demo customer, for calling the gated endpoints.
func customerToken(t *testing.T, ctx context.Context, tester *connector.Connector, username string) string {
	actor := banksupportapi.Actor{Subject: username, Roles: []string{"customer"}}
	token, err := accesstokenapi.NewClient(tester).Mint(ctx, actor)
	testarossa.For(t).NoError(err)
	return token
}

func TestBankSupport_Balance(t *testing.T) { // MARKER: Balance
	t.Parallel()
	ctx := t.Context()

	svc := NewService()
	tester := connector.New("tester.client")
	client := banksupportapi.NewClient(tester)

	app := application.New()
	app.Add(
		accesstoken.NewService(),
		bearertoken.NewService(),
		svc,
		tester,
	)
	app.RunInTest(t)

	t.Run("returns_balance", func(t *testing.T) {
		assert := testarossa.For(t)

		balanceCents, holder, err := client.WithOptions(pub.Token(customerToken(t, ctx, tester, "alice"))).Balance(ctx)
		assert.Expect(
			err, nil,
			holder, "Alice Anderson",
		)
		assert.NotEqual(balanceCents, 0)
	})

	t.Run("denied_without_customer_claim", func(t *testing.T) {
		assert := testarossa.For(t)

		_, _, err := client.Balance(ctx)
		assert.Error(err)
	})
}

func TestBankSupport_Transactions(t *testing.T) { // MARKER: Transactions
	t.Parallel()
	ctx := t.Context()

	svc := NewService()
	tester := connector.New("tester.client")
	client := banksupportapi.NewClient(tester)

	app := application.New()
	app.Add(
		accesstoken.NewService(),
		bearertoken.NewService(),
		svc,
		tester,
	)
	app.RunInTest(t)

	t.Run("returns_seeded_history", func(t *testing.T) {
		assert := testarossa.For(t)

		txns, err := client.WithOptions(pub.Token(customerToken(t, ctx, tester, "bob"))).Transactions(ctx, "", "")
		assert.NoError(err)
		assert.True(len(txns) > 0)
		// Every row carries a category and a date, so the model can group and sum.
		for _, txn := range txns {
			assert.NotEqual(txn.Category, "")
			assert.NotEqual(txn.Date, "")
		}
	})
}

func TestBankSupport_Support(t *testing.T) { // MARKER: Support
	t.Parallel()
	ctx := t.Context()

	svc := NewService()
	tester := connector.New("tester.client")
	foremanClient := foremanapi.NewClient(tester)
	exec := banksupportapi.NewExecutor(tester).WithWorkflowRunner(foremanClient)

	// Mock the chat loop so the workflow runs without a provider key; return a structured JSON verdict.
	llmMock := llm.NewMock()
	llmMock.MockChatLoop(func(ctx context.Context, flow *workflow.Flow, provider string, model string, items []llmapi.Item, toolURLs []string, options *llmapi.ChatOptions) (itemsOut []llmapi.Item, usage llmapi.Usage, err error) {
		reply := "```json\n{\"advice\": \"Your balance is healthy.\", \"blockCard\": false, \"risk\": 2}\n```"
		itemsOut = append(items, llmapi.NewMessage("assistant", reply).AsItem())
		return itemsOut, llmapi.Usage{}, nil
	})

	app := application.New()
	app.Add(
		foreman.NewService(),
		llmMock,
		svc,
		tester,
	)
	app.RunInTest(t)

	t.Run("parses_structured_verdict", func(t *testing.T) {
		assert := testarossa.For(t)

		advice, blockCard, risk, status, err := exec.Support(ctx, "What is my balance?")
		assert.Expect(
			err, nil,
			status, workflow.StatusCompleted,
			advice, "Your balance is healthy.",
			blockCard, false,
			risk, 2,
		)
	})
}

func TestBankSupport_ParseVerdict(t *testing.T) {
	t.Parallel()

	t.Run("fenced_json", func(t *testing.T) {
		assert := testarossa.For(t)
		out := parseVerdict("```json\n{\"advice\":\"ok\",\"blockCard\":true,\"risk\":9}\n```")
		assert.Expect(
			out.Advice, "ok",
			out.BlockCard, true,
			out.Risk, 9,
		)
	})

	t.Run("risk_clamped", func(t *testing.T) {
		assert := testarossa.For(t)
		out := parseVerdict("{\"advice\":\"ok\",\"risk\":42}")
		assert.Equal(out.Risk, 10)
	})

	t.Run("non_json_falls_back_to_advice", func(t *testing.T) {
		assert := testarossa.For(t)
		out := parseVerdict("Sorry, I could not help.")
		assert.Expect(
			out.Advice, "Sorry, I could not help.",
			out.Risk, 0,
		)
	})
}

func TestBankSupport_Login(t *testing.T) { // MARKER: Login
	t.Parallel()
	ctx := t.Context()
	_ = ctx

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the tester client
	tester := connector.New("tester.client")
	client := banksupportapi.NewClient(tester)
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

			res, err := client.Login(ctx, "GET", "", nil)
			if assert.NoError(err) {
				assert.Expect(res.StatusCode, http.StatusOK)
			}
		})
	*/
}

func TestBankSupport_Logout(t *testing.T) { // MARKER: Logout
	t.Parallel()
	ctx := t.Context()
	_ = ctx

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the tester client
	tester := connector.New("tester.client")
	client := banksupportapi.NewClient(tester)
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

			res, err := client.Logout(ctx, "GET", "", nil)
			if assert.NoError(err) {
				assert.Expect(res.StatusCode, http.StatusOK)
			}
		})
	*/
}

func TestBankSupport_Demo(t *testing.T) { // MARKER: Demo
	t.Parallel()
	ctx := t.Context()
	_ = ctx

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the tester client
	tester := connector.New("tester.client")
	client := banksupportapi.NewClient(tester)
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

func TestBankSupport_RunSupport(t *testing.T) { // MARKER: RunSupport
	t.Parallel()
	ctx := t.Context()
	_ = ctx

	// Initialize the microservice under test
	svc := NewService()

	// Initialize the tester
	tester := connector.New("tester.client")
	exec := banksupportapi.NewExecutor(tester)
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

			advice, blockCard, risk, err := exec.RunSupport(ctx, query)
			assert.Expect(
				advice, expectedAdvice,
				blockCard, expectedBlockCard,
				risk, expectedRisk,
				err, nil,
			)
		})
	*/
}

func TestBankSupport_DemoStatus(t *testing.T) { // MARKER: DemoStatus
	t.Parallel()
	ctx := t.Context()

	svc := NewService()
	tester := connector.New("tester.client")
	client := banksupportapi.NewClient(tester)
	foremanClient := foremanapi.NewClient(tester)

	llmMock := llm.NewMock()
	llmMock.MockChatLoop(func(ctx context.Context, flow *workflow.Flow, provider string, model string, items []llmapi.Item, toolURLs []string, options *llmapi.ChatOptions) (itemsOut []llmapi.Item, usage llmapi.Usage, err error) {
		itemsOut = append(items, llmapi.NewMessage("assistant", "{\"advice\":\"All good.\",\"blockCard\":false,\"risk\":1}").AsItem())
		return itemsOut, llmapi.Usage{}, nil
	})

	app := application.New()
	app.Add(
		accesstoken.NewService(),
		bearertoken.NewService(),
		foreman.NewService(),
		llmMock,
		svc,
		tester,
	)
	app.RunInTest(t)

	t.Run("returns_completed_verdict", func(t *testing.T) {
		assert := testarossa.For(t)

		// Launch the workflow, then long-poll it to completion through DemoStatus.
		flowKey, err := foremanClient.Create(ctx, banksupportapi.Support.URL(), banksupportapi.SupportIn{Query: "hi"}, nil)
		assert.NoError(err)

		status, advice, blockCard, risk, errorMsg, err := client.WithOptions(pub.Token(customerToken(t, ctx, tester, "alice"))).DemoStatus(ctx, flowKey)
		assert.Expect(
			err, nil,
			status, "completed",
			advice, "All good.",
			blockCard, false,
			risk, 1,
			errorMsg, "",
		)
	})

	t.Run("denied_without_customer_claim", func(t *testing.T) {
		assert := testarossa.For(t)

		_, _, _, _, _, err := client.DemoStatus(ctx, "whatever")
		assert.Error(err)
	})
}

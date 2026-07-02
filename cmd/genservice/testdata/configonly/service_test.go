package configonly

import (
	"testing"

	"github.com/microbus-io/fabric/application"
)

func TestConfigOnly_OnChangedDenyList(t *testing.T) { // MARKER: DenyList
	t.Parallel()
	ctx := t.Context()
	_ = ctx

	// Initialize the microservice under test
	svc := NewService()

	// Run the testing app
	app := application.New()
	app.Add(
		// HINT: Add microservices or mocks required for this test
		svc,
	)
	app.RunInTest(t)

	/*
		HINT: Fill in test cases using the following pattern

		t.Run("test_case_name", func(t *testing.T) {
			assert := testarossa.For(t)

			err := svc.SetDenyList(newValue)
			assert.NoError(err)
		})
	*/
}

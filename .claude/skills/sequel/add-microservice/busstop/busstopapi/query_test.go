package busstopapi

import (
	"strings"
	"testing"

	"github.com/microbus-io/testarossa"
)

func TestBusStop_ValidateQuery(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	assert := testarossa.For(t)

	// Prepare a valid query
	validQuery := Query{
		// HINT: Initialize the query's fields with valid values
		Example: "Valid value",
	}
	err := validQuery.Validate(ctx)
	assert.NoError(err)

	// HINT: Check validation of individual query fields
	t.Run("example_too_long", func(t *testing.T) {
		assert := testarossa.For(t)
		invalidQuery := validQuery
		invalidQuery.Example = strings.Repeat("X", 1024) // Too long
		assert.Error(invalidQuery.Validate(ctx))
	})
}

package foreman

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/microbus-io/dwarf/workflow"
	"github.com/microbus-io/errors"
	"github.com/microbus-io/testarossa"
)

// TestForeman_TimeBudgetCeiling asserts the foreman rejects a per-flow time budget above maxTimeBudget at
// the resolveOptions chokepoint (the dwarf engine imposes no ceiling, so the host enforces the SLA limit).
func TestForeman_TimeBudgetCeiling(t *testing.T) {
	assert := testarossa.For(t)
	svc := NewService()
	ctx := context.Background()

	// Under the ceiling: accepted.
	_, err := svc.resolveOptions(ctx, &workflow.FlowOptions{TimeBudget: 10 * time.Minute})
	assert.NoError(err)

	// At the ceiling exactly: accepted (the bound is exclusive only above).
	_, err = svc.resolveOptions(ctx, &workflow.FlowOptions{TimeBudget: maxTimeBudget})
	assert.NoError(err)

	// Over the ceiling: rejected with 400.
	_, err = svc.resolveOptions(ctx, &workflow.FlowOptions{TimeBudget: maxTimeBudget + time.Second})
	assert.Error(err)
	assert.Equal(http.StatusBadRequest, errors.StatusCode(err))

	// No options / no budget: accepted (uses the engine default).
	_, err = svc.resolveOptions(ctx, nil)
	assert.NoError(err)
}

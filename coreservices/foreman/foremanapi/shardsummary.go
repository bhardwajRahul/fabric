package foremanapi

import (
	"github.com/microbus-io/dwarf/engine"
)

// ShardSummary carries per-shard health and size information, returned by the ShardInfo endpoint.
type ShardSummary = engine.ShardSummary

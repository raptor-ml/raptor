package api

import (
	"time"
)

// Metadata is the metadata of a feature.
type Metadata struct {
	FQN       string        `json:"FQN"`
	Primitive PrimitiveType `json:"primitive"`
	Aggr      []WindowFn    `json:"aggr"`
	Freshness time.Duration `json:"freshness"`
	Staleness time.Duration `json:"staleness"`
	Timeout   time.Duration `json:"timeout"`
	Builder   string        `json:"builder"`
}

// ValidWindow checks if the feature have aggregation enabled, and if it is valid
func (md Metadata) ValidWindow() bool {
	if md.Freshness < 1 {
		return false
	}
	if md.Staleness < md.Freshness {
		return false
	}
	if len(md.Aggr) == 0 {
		return false
	}
	if !(md.Primitive == PrimitiveTypeInteger || md.Primitive == PrimitiveTypeFloat) {
		return false
	}
	return true
}

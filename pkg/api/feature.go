/*
Copyright 2022 Natun.

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

package api

import (
	"fmt"
	manifests "github.com/natun-ai/natun/pkg/api/v1alpha1"
	"strings"
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
func aggrsToStrings(a []manifests.AggrType) []string {
	var res []string
	for _, v := range a {
		res = append(res, string(v))
	}
	return res
}

func MetadataFromManifest(in *manifests.Feature) (*Metadata, error) {
	primitive := StringToPrimitiveType(in.Spec.Primitive)
	if primitive == PrimitiveTypeUnknown {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedPrimitiveError, in.Spec.Primitive)
	}
	aggr, err := StringsToWindowFns(aggrsToStrings(in.Spec.Aggr))
	if err != nil {
		return nil, fmt.Errorf("failed to parse aggregation functions: %w", err)
	}

	md := &Metadata{
		FQN:       in.FQN(),
		Primitive: primitive,
		Aggr:      aggr,
		Freshness: in.Spec.Freshness.Duration,
		Staleness: in.Spec.Staleness.Duration,
		Timeout:   in.Spec.Timeout.Duration,
		Builder:   strings.ToLower(in.Spec.Builder.Kind),
	}
	if len(md.Aggr) > 0 && !md.ValidWindow() {
		return nil, fmt.Errorf("invalid feature specification for windowed feature")
	}
	return md, nil
}

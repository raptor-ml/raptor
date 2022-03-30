package engine

import (
	"encoding/json"
	"fmt"
	"github.com/natun-ai/natun/internal/plugin"
	"github.com/natun-ai/natun/pkg/api"
	manifests "github.com/natun-ai/natun/pkg/api/v1alpha1"
	"github.com/natun-ai/natun/pkg/errors"
)

func aggrsToStrings(a []manifests.AggrType) []string {
	var res []string
	for _, v := range a {
		res = append(res, string(v))
	}
	return res
}

// BindFeature converts the k8s Feature CRD to the internal implementation, and adds it to the engine.
func (e *engine) BindFeature(in manifests.Feature) error {
	primitive := api.StringToPrimitiveType(in.Spec.Primitive)
	if primitive == api.PrimitiveTypeUnknown {
		return fmt.Errorf("%w: %s", errors.ErrUnsupportedPrimitiveError, in.Spec.Primitive)
	}
	aggr, err := api.StringsToWindowFns(aggrsToStrings(in.Spec.Aggr))
	if err != nil {
		return fmt.Errorf("failed to parse aggregation functions: %w", err)
	}

	ft := Feature{
		Metadata: api.Metadata{
			FQN:       in.FQN(),
			Primitive: primitive,
			Aggr:      aggr,
			Freshness: in.Spec.Freshness.Duration,
			Staleness: in.Spec.Staleness.Duration,
			Timeout:   in.Spec.Timeout.Duration,
			Builder:   in.Spec.Builder.Type,
		},
	}

	if len(aggr) > 0 && !ft.ValidWindow() {
		return fmt.Errorf("invalid feature specification for windowed feature")
	}

	if ft.Builder == "" {
		builderType := &manifests.FeatureBuilderType{}
		err := json.Unmarshal(in.Spec.Builder.Raw, builderType)
		if err != nil || builderType.Type == "" {
			return fmt.Errorf("failed to unmarshal builder type: %w", err)
		}
		ft.Builder = builderType.Type
	}

	if p := plugin.FeatureAppliers.Get(ft.Builder); p != nil {
		err := p(ft.Metadata, in.Spec.Builder.JSON.Raw, &ft, e)
		if err != nil {
			return err
		}
	}

	return e.bindFeature(ft)
}

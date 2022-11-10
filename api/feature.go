/*
Copyright (c) 2022 RaptorML authors.

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
	"context"
	"fmt"
	manifests "github.com/raptor-ml/raptor/api/v1alpha1"
	"regexp"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
	"time"
)

const FeatureSetBuilder = "featureset"
const ExpressionBuilder = "expression"

// DataConnector is a parsed abstracted representation of a manifests.DataConnector
type DataConnector struct {
	FQN    string                 `json:"fqn"`
	Kind   string                 `json:"kind"`
	Config manifests.ParsedConfig `json:"config"`
}

// DataConnectorFromManifest returns a DataConnector from a manifests.DataConnector
func DataConnectorFromManifest(ctx context.Context, dc *manifests.DataConnector, r client.Reader) (DataConnector, error) {
	pc, err := dc.ParseConfig(ctx, r)
	if err != nil {
		return DataConnector{}, fmt.Errorf("failed to parse config: %w", err)
	}

	return DataConnector{
		FQN:    dc.FQN(),
		Kind:   dc.Spec.Kind,
		Config: pc,
	}, nil
}

// FeatureDescriptor is describing a feature definition for an internal use of the Core.
type FeatureDescriptor struct {
	FQN           string        `json:"FQN"`
	Primitive     PrimitiveType `json:"primitive"`
	Aggr          []AggrFn      `json:"aggr"`
	Freshness     time.Duration `json:"freshness"`
	Staleness     time.Duration `json:"staleness"`
	Timeout       time.Duration `json:"timeout"`
	Builder       string        `json:"builder"`
	DataConnector string        `json:"connector"`
}

// ValidWindow checks if the feature have aggregation enabled, and if it is valid
func (fd FeatureDescriptor) ValidWindow() bool {
	if fd.Freshness < 1 {
		return false
	}
	if fd.Staleness < fd.Freshness {
		return false
	}
	if len(fd.Aggr) == 0 {
		return false
	}
	if !(fd.Primitive == PrimitiveTypeInteger || fd.Primitive == PrimitiveTypeFloat) {
		return false
	}
	return true
}
func aggrsToStrings(a []manifests.AggrFn) []string {
	var res []string
	for _, v := range a {
		res = append(res, string(v))
	}
	return res
}

// FeatureDescriptorFromManifest returns a FeatureDescriptor from a manifests.Feature
func FeatureDescriptorFromManifest(in *manifests.Feature) (*FeatureDescriptor, error) {
	primitive := StringToPrimitiveType(string(in.Spec.Primitive))
	if primitive == PrimitiveTypeUnknown {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedPrimitiveError, in.Spec.Primitive)
	}
	aggr, err := StringsToAggrFns(aggrsToStrings(in.Spec.Builder.Aggr))
	if err != nil {
		return nil, fmt.Errorf("failed to parse aggregation functions: %w", err)
	}
	if len(aggr) > 0 && primitive != PrimitiveTypeInteger && primitive != PrimitiveTypeFloat {
		return nil, fmt.Errorf("%w with Aggregation: %s", ErrUnsupportedPrimitiveError, in.Spec.Primitive)
	}
	if in.Spec.Builder.AggrGranularity.Milliseconds() > 0 && len(aggr) > 0 {
		in.Spec.Freshness = in.Spec.Builder.AggrGranularity
	}

	fd := &FeatureDescriptor{
		FQN:       in.FQN(),
		Primitive: primitive,
		Aggr:      aggr,
		Freshness: in.Spec.Freshness.Duration,
		Staleness: in.Spec.Staleness.Duration,
		Timeout:   in.Spec.Timeout.Duration,
		Builder:   strings.ToLower(in.Spec.Builder.Kind),
	}
	if in.Spec.DataConnector != nil {
		fd.DataConnector = in.Spec.DataConnector.FQN()
	}
	if fd.Builder == "" {
		fd.Builder = ExpressionBuilder
	}

	if len(fd.Aggr) > 0 && !fd.ValidWindow() {
		return nil, fmt.Errorf("invalid feature specification for windowed feature")
	}
	return fd, nil
}

var FQNRegExp = regexp.MustCompile(`(?si)^((?P<namespace>([a0-z9]+[a0-z9_]*[a0-z9]+){1,256})\.)?(?P<name>([a0-z9]+[a0-z9_]*[a0-z9]+){1,256})(\+(?P<aggrFn>([a-z]+_*[a-z]+)))?(@-(?P<version>([0-9]+)))?(\[(?P<encoding>([a-z]+_*[a-z]+))])?$`)

func ParseFQN(fqn string) (namespace, name, aggrFn, version, encoding string, err error) {
	if !FQNRegExp.MatchString(fqn) {
		return "", "", "", "", "", fmt.Errorf("invalid FQN: %s", fqn)
	}

	match := FQNRegExp.FindStringSubmatch(fqn)
	parsedFQN := make(map[string]string)
	for i, name := range FQNRegExp.SubexpNames() {
		if i != 0 && name != "" {
			parsedFQN[name] = match[i]
		}
	}

	namespace = parsedFQN["namespace"]
	name = parsedFQN["name"]
	aggrFn = parsedFQN["aggrFn"]
	version = parsedFQN["version"]
	encoding = parsedFQN["encoding"]
	return
}

func NormalizeFQN(fqn, defaultNamespace string) string {
	ns, name, aggrFn, version, enc, err := ParseFQN(fqn)
	if err != nil {
		panic(err)
	}

	if ns == "" {
		ns = defaultNamespace
	}

	other := ""
	if aggrFn != "" {
		other = fmt.Sprintf("%s+%s", other, aggrFn)
	}
	if version != "" {
		other = fmt.Sprintf("%s@-%s", other, version)
	}
	if enc != "" {
		other = fmt.Sprintf("%s[%s]", other, enc)
	}
	return fmt.Sprintf("%s.%s%s", ns, name, other)
}

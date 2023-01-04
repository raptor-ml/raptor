/*
 * Copyright (c) 2022 RaptorML authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package querybuilder

import (
	"bytes"
	"context"
	"fmt"
	"github.com/raptor-ml/raptor/api"
	manifests "github.com/raptor-ml/raptor/api/v1alpha1"
	"time"
)

type featureSetQuery struct {
	baseQuery
	BeforePadding time.Duration
	Features      []api.FeatureDescriptor
	KeyFeature    string
}

func (qb *queryBuilder) FeatureSet(ctx context.Context, fs manifests.ModelSpec, getter api.FeatureDescriptorGetter) (string, error) {
	if fs.KeyFeature == "" {
		fs.KeyFeature = fs.Features[0]
	}

	found := false
	for _, f := range fs.Features {
		if f == fs.KeyFeature {
			found = true
			break
		}
	}
	if !found {
		fs.Features = append(fs.Features, fs.KeyFeature)
	}

	data := featureSetQuery{
		baseQuery: baseQuery{
			Since:         "$SINCE",
			Until:         "$UNTIL",
			FeaturesTable: qb.featureTable,
		},
		KeyFeature: fs.KeyFeature,
	}

	for _, fqn := range fs.Features {
		ft, err := getter(ctx, fqn)
		if err != nil {
			return "", fmt.Errorf("failed to get FeatureDescriptor for %s: %w", fqn, err)
		}
		if ft.ValidWindow() {
			if data.BeforePadding < ft.Staleness {
				data.BeforePadding = ft.Staleness
			}
		}
		data.Features = append(data.Features, ft)
	}

	var query bytes.Buffer
	err := qb.tpls.ExecuteTemplate(&query, "featureset.tmpl.sql", data)
	if err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return query.String(), nil
}

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

package snowflake

import (
	"context"
	"fmt"
	"time"

	"github.com/raptor-ml/raptor/api"
	manifests "github.com/raptor-ml/raptor/api/v1alpha1"
	"github.com/raptor-ml/raptor/pkg/querybuilder"
	sf "github.com/snowflakedb/gosnowflake"
)

func (sw *snowflakeWriter) BindFeature(md *api.Metadata, fs *manifests.FeatureSetSpec, getter api.MetadataGetter) error {
	var query string
	var typ string
	if md.Builder == api.FeatureSetBuilder {
		typ = "FeatureSet"
		if fs == nil {
			return fmt.Errorf("feature set is nil")
		}
		q, err := sw.queryBuilder.FeatureSet(context.TODO(), *fs, getter)
		if err != nil {
			return fmt.Errorf("failed to build FeatureSet query: %w", err)
		}
		query = q
	} else {
		typ = "Feature"
		q, err := sw.queryBuilder.Feature(*md)
		if err != nil {
			return fmt.Errorf("failed to build Feature query: %w", err)
		}
		query = q
	}

	const viewQuery = `SET (SINCE,UNTIL) = ('2020-12-01', '2022-12-31');
CREATE OR REPLACE VIEW %s
	COMMENT ='%s %s. Requires session variables $SINCE and $UNTIL.'
AS %s`

	ctx, _ := sf.WithMultiStatement(context.TODO(), 2)
	_, err := sw.db.ExecContext(ctx, fmt.Sprintf(viewQuery, querybuilder.EscapeName(md.FQN), md.FQN, typ, query))
	if err != nil {
		return fmt.Errorf("failed to create %s view for %s: %w", typ, md.FQN, err)
	}
	return nil
}

func subtractDuration(d time.Duration, field string) string {
	var unit string
	var v int64
	switch {
	case d%time.Hour == 0:
		unit = "hour"
		v = int64(d / time.Hour)
	case d%time.Minute == 0:
		unit = "minute"
		v = int64(d / time.Minute)
	case d%time.Second == 0:
		unit = "second"
		v = int64(d / time.Second)
	case d%time.Millisecond == 0:
		unit = "millisecond"
		v = int64(d / time.Millisecond)
	case d%time.Microsecond == 0:
		unit = "microsecond"
		v = int64(d / time.Microsecond)
	default:
		unit = "nanosecond"
		v = int64(d / time.Nanosecond)
	}
	if v > 0 {
		v *= -1
	}
	return fmt.Sprintf("DATEADD('%s', %d, %s)", unit, v, field)
}

func castFeature(ft api.Metadata) string {
	if ft.ValidWindow() {
		return "OBJECT"
	}
	if !ft.Primitive.Scalar() {
		return "ARRAY"
	}
	switch ft.Primitive {
	case api.PrimitiveTypeString:
		return "STRING"
	case api.PrimitiveTypeInteger:
		return "INT"
	case api.PrimitiveTypeFloat:
		return "DOUBLE"
	case api.PrimitiveTypeTimestamp:
		return "TIMESTAMP"
	}
	return "VARIANT"
}

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

package sdk

import (
	"fmt"
	"github.com/raptor-ml/raptor/api"
	coreApi "github.com/raptor-ml/raptor/api/proto/gen/go/core/v1alpha1"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"reflect"
	"time"
)

func ToAPIScalar(val any) *coreApi.Scalar {
	primitive := api.TypeDetect(val)

	switch primitive {
	case api.PrimitiveTypeString:
		return &coreApi.Scalar{Value: &coreApi.Scalar_StringValue{StringValue: val.(string)}}
	case api.PrimitiveTypeInteger:
		return &coreApi.Scalar{Value: &coreApi.Scalar_IntValue{IntValue: int32(val.(int))}}
	case api.PrimitiveTypeFloat:
		return &coreApi.Scalar{Value: &coreApi.Scalar_FloatValue{FloatValue: val.(float64)}}
	case api.PrimitiveTypeBoolean:
		return &coreApi.Scalar{Value: &coreApi.Scalar_BoolValue{BoolValue: val.(bool)}}
	case api.PrimitiveTypeTimestamp:
		return &coreApi.Scalar{Value: &coreApi.Scalar_TimestampValue{TimestampValue: timestamppb.New(val.(time.Time))}}
	default:
		panic(fmt.Sprintf("unsupported type - is it scalar? (%v)", primitive.Scalar()))
	}
}
func ToAPIValue(val any) *coreApi.Value {
	if val == nil {
		return nil
	}

	var ret coreApi.Value
	primitive := api.TypeDetect(val)

	if primitive == api.PrimitiveTypeUnknown {
		panic("unknown primitive type")
	}

	if primitive.Scalar() {
		ret.Value = &coreApi.Value_ScalarValue{ScalarValue: ToAPIScalar(val)}
	} else {
		list := &coreApi.List{}
		ret.Value = &coreApi.Value_ListValue{ListValue: list}

		v := reflect.ValueOf(val)
		for i := 0; i < v.Len(); i++ {
			list.Values = append(list.Values, ToAPIScalar(v.Index(i).Interface()))
		}
	}
	return &ret
}

func ToAPIPrimitive(p api.PrimitiveType) coreApi.Primitive {
	switch p {
	default:
		return coreApi.Primitive_PRIMITIVE_UNSPECIFIED
	case api.PrimitiveTypeString:
		return coreApi.Primitive_PRIMITIVE_STRING
	case api.PrimitiveTypeInteger:
		return coreApi.Primitive_PRIMITIVE_INTEGER
	case api.PrimitiveTypeFloat:
		return coreApi.Primitive_PRIMITIVE_FLOAT
	case api.PrimitiveTypeTimestamp:
		return coreApi.Primitive_PRIMITIVE_TIMESTAMP
	case api.PrimitiveTypeBoolean:
		return coreApi.Primitive_PRIMITIVE_BOOL
	case api.PrimitiveTypeStringList:
		return coreApi.Primitive_PRIMITIVE_STRING_LIST
	case api.PrimitiveTypeIntegerList:
		return coreApi.Primitive_PRIMITIVE_INTEGER_LIST
	case api.PrimitiveTypeFloatList:
		return coreApi.Primitive_PRIMITIVE_FLOAT_LIST
	case api.PrimitiveTypeBooleanList:
		return coreApi.Primitive_PRIMITIVE_BOOL_LIST
	case api.PrimitiveTypeTimestampList:
		return coreApi.Primitive_PRIMITIVE_TIMESTAMP_LIST
	}
}
func ToAPIAggrFn(f api.AggrFn) coreApi.AggrFn {
	switch f {
	default:
		return coreApi.AggrFn_AGGR_FN_UNSPECIFIED
	case api.AggrFnCount:
		return coreApi.AggrFn_AGGR_FN_COUNT
	case api.AggrFnSum:
		return coreApi.AggrFn_AGGR_FN_SUM
	case api.AggrFnAvg:
		return coreApi.AggrFn_AGGR_FN_AVG
	case api.AggrFnMin:
		return coreApi.AggrFn_AGGR_FN_MIN
	case api.AggrFnMax:
		return coreApi.AggrFn_AGGR_FN_MAX
	}
}
func ToAPIAggrFns(fs []api.AggrFn) []coreApi.AggrFn {
	ret := make([]coreApi.AggrFn, len(fs))
	for i, f := range fs {
		ret[i] = ToAPIAggrFn(f)
	}
	return ret
}

func ToAPIFeatureDescriptor(fd api.FeatureDescriptor) *coreApi.FeatureDescriptor {
	var kp *coreApi.KeepPrevious
	if fd.KeepPrevious != nil {
		kp = &coreApi.KeepPrevious{
			Versions: uint32(fd.KeepPrevious.Versions),
			Over:     durationpb.New(fd.KeepPrevious.Over),
		}
	}
	ret := &coreApi.FeatureDescriptor{
		Fqn:          fd.FQN,
		Primitive:    ToAPIPrimitive(fd.Primitive),
		Aggr:         ToAPIAggrFns(fd.Aggr),
		Freshness:    durationpb.New(fd.Freshness),
		Staleness:    durationpb.New(fd.Staleness),
		Timeout:      durationpb.New(fd.Timeout),
		KeepPrevious: kp,
		Builder:      fd.Builder,
		DataSource:   fd.DataSource,
	}

	return ret
}

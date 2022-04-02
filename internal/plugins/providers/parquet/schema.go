package parquet

import (
	"github.com/natun-ai/natun/pkg/api"
	"github.com/xitongsys/parquet-go/types"
	"strings"
	"time"
)

type HistoricalRecord struct {
	FQN       string  `parquet:"name=utf8, type=BYTE_ARRAY, convertedtype=UTF8, encoding=RLE_DICTIONARY"`
	EntityID  string  `parquet:"name=entity_id, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN"`
	Timestamp int64   `parquet:"name=timestamp, type=INT64, logicaltype=TIMESTAMP, logicaltype.isadjustedtoutc=false, logicaltype.unit=MICROS"`
	Value     *Value  `parquet:"name=Value"`
	Bucket    *Bucket `parquet:"name=Bucket"`
}
type Value struct {
	String    *string  `parquet:"name=string, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN"`
	Int       *int64   `parquet:"name=int, type=INT64"`
	Double    *float64 `parquet:"name=double, type=DOUBLE"`
	Timestamp *int64   `parquet:"name=timestamp, type=INT64, logicaltype=TIMESTAMP, logicaltype.isadjustedtoutc=false, logicaltype.unit=MICROS"`

	StringList    *[]string  `parquet:"name=string_list, type=MAP, convertedtype=LIST, valuetype=BYTE_ARRAY, valueconvertedtype=UTF8"`
	IntList       *[]int64   `parquet:"name=int_list, type=MAP, convertedtype=LIST, valuetype=INT64"`
	DoubleList    *[]float64 `parquet:"name=double_list, type=MAP, convertedtype=LIST, valuetype=DOUBLE"`
	TimestampList *[]int64   `parquet:"name=timestamp_list, type=MAP, convertedtype=LIST, valuetype=INT64, valuelogicaltype=TIMESTAMP, valuelogicaltype.isadjustedtoutc=false, valuelogicaltype.unit=MICROS"`
}
type Bucket struct {
	Name  string `parquet:"name=name, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN"`
	Alive *bool  `parquet:"name=alive, type=BOOLEAN"`

	Count *int64   `parquet:"name=count, type=INT64"`
	Sum   *float64 `parquet:"name=sum, type=DOUBLE"`
	Min   *float64 `parquet:"name=min, type=DOUBLE"`
	Max   *float64 `parquet:"name=max, type=DOUBLE"`
}

func NewHistoricalRecord(wn api.WriteNotification) HistoricalRecord {
	hr := HistoricalRecord{
		FQN:       wn.FQN,
		EntityID:  wn.EntityID,
		Timestamp: types.TimeToTIMESTAMP_MICROS(wn.Value.Timestamp, false),
	}
	if wn.Bucket != "" {
		alive := strings.HasPrefix(wn.Bucket, api.AliveMarker)
		wrm := api.ToLowLevelValue[api.WindowResultMap](wn.Value.Value)

		count := int64(wrm[api.WindowFnCount])
		sum := wrm[api.WindowFnSum]
		min := wrm[api.WindowFnMin]
		max := wrm[api.WindowFnMax]
		hr.Bucket = &Bucket{
			Name:  strings.TrimPrefix(wn.Bucket, api.AliveMarker),
			Alive: &alive,
			Count: &count,
			Sum:   &sum,
			Min:   &min,
			Max:   &max,
		}
		return hr
	}
	switch api.TypeDetect(wn.Value.Value) {
	case api.PrimitiveTypeString:
		v := api.ToLowLevelValue[string](wn.Value.Value)
		hr.Value = &Value{
			String: &v,
		}
	case api.PrimitiveTypeInteger:
		v := int64(api.ToLowLevelValue[int](wn.Value.Value))
		hr.Value = &Value{
			Int: &v,
		}
	case api.PrimitiveTypeFloat:
		v := api.ToLowLevelValue[float64](wn.Value.Value)
		hr.Value = &Value{
			Double: &v,
		}
	case api.PrimitiveTypeTimestamp:
		v := types.TimeToTIMESTAMP_MICROS(api.ToLowLevelValue[time.Time](wn.Value.Value), false)
		hr.Value = &Value{
			Timestamp: &v,
		}
	case api.PrimitiveTypeStringList:
		v := api.ToLowLevelValue[[]string](wn.Value.Value)
		hr.Value = &Value{
			StringList: &v,
		}
	case api.PrimitiveTypeIntegerList:
		v := api.ToLowLevelValue[[]int](wn.Value.Value)
		var l []int64
		for _, i := range v {
			l = append(l, int64(i))
		}
		hr.Value = &Value{
			IntList: &l,
		}
	case api.PrimitiveTypeFloatList:
		v := api.ToLowLevelValue[[]float64](wn.Value.Value)
		hr.Value = &Value{
			DoubleList: &v,
		}
	case api.PrimitiveTypeTimestampList:
		v := api.ToLowLevelValue[[]time.Time](wn.Value.Value)
		var l []int64
		for _, t := range v {
			l = append(l, types.TimeToTIMESTAMP_MICROS(t, false))
		}
		hr.Value = &Value{
			TimestampList: &l,
		}
	}
	return hr
}

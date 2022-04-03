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
	"reflect"
	"strconv"
	"strings"
	"time"
)

type PrimitiveType int

const (
	PrimitiveTypeUnknown PrimitiveType = iota
	PrimitiveTypeString
	PrimitiveTypeTimestamp
	PrimitiveTypeInteger
	PrimitiveTypeFloat
	PrimitiveTypeIntegerList
	PrimitiveTypeFloatList
	PrimitiveTypeStringList
	PrimitiveTypeTimestampList
	PrimitiveTypeHeadless
)

func StringToPrimitiveType(s string) PrimitiveType {
	switch strings.ToLower(s) {
	case "string", "text":
		return PrimitiveTypeString
	case "time", "datetime", "timestamp", "time.time":
		return PrimitiveTypeTimestamp
	case "integer", "int", "int64", "int32":
		return PrimitiveTypeInteger
	case "float", "double", "float32", "float64":
		return PrimitiveTypeFloat
	case "[]integer", "[]int", "[]int64", "[]int32":
		return PrimitiveTypeIntegerList
	case "[]float", "[]double", "[]float32", "[]float64":
		return PrimitiveTypeFloatList
	case "[]string", "[]text":
		return PrimitiveTypeStringList
	case "[]time", "[]datetime", "[]timestamp", "[]time.time":
		return PrimitiveTypeTimestampList
	case "headless":
		return PrimitiveTypeHeadless
	default:
		return PrimitiveTypeUnknown
	}
}

func (ft PrimitiveType) Scalar() bool {
	switch ft {
	case PrimitiveTypeTimestampList, PrimitiveTypeStringList, PrimitiveTypeIntegerList, PrimitiveTypeFloatList:
		return false
	default:
		return true
	}
}
func (ft PrimitiveType) Singular() PrimitiveType {
	switch ft {
	case PrimitiveTypeTimestampList:
		return PrimitiveTypeTimestamp
	case PrimitiveTypeStringList:
		return PrimitiveTypeString
	case PrimitiveTypeIntegerList:
		return PrimitiveTypeInteger
	case PrimitiveTypeFloatList:
		return PrimitiveTypeFloat
	default:
		return ft
	}
}
func (ft PrimitiveType) Plural() PrimitiveType {
	switch ft {
	case PrimitiveTypeTimestamp:
		return PrimitiveTypeTimestampList
	case PrimitiveTypeString:
		return PrimitiveTypeStringList
	case PrimitiveTypeInteger:
		return PrimitiveTypeIntegerList
	case PrimitiveTypeFloat:
		return PrimitiveTypeFloatList
	default:
		return ft
	}
}

func ScalarString(val any) string {
	switch v := val.(type) {
	case int:
		return strconv.Itoa(v)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case string:
		return v
	case time.Time:
		return strconv.FormatInt(v.UnixMicro(), 10)
	default:
		panic("unreachable")
	}
}

func ScalarFromString(val string, scalar PrimitiveType) (any, error) {
	switch scalar {
	case PrimitiveTypeInteger:
		return strconv.Atoi(val)
	case PrimitiveTypeFloat:
		return strconv.ParseFloat(val, 64)
	case PrimitiveTypeString:
		return val, nil
	case PrimitiveTypeTimestamp:
		n, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return nil, err
		}
		return time.UnixMicro(n), nil
	default:
		panic("unreachable")
	}
}

// TypeDetect detects the PrimitiveType of the value.
func TypeDetect(t any) PrimitiveType {
	reflectType := reflect.TypeOf(t).String()
	return StringToPrimitiveType(reflectType)
}

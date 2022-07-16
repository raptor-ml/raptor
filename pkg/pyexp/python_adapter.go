/*
 * Copyright (c) 2022 Natun.
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

package pyexp

import (
	"encoding/json"
	"fmt"
	"github.com/go-logr/logr/funcr"
	"github.com/raptor-ml/natun/api"
	"reflect"
	"time"
)

type PyVal struct {
	Value     string    `json:"value"` // Value is a JSON string
	Timestamp time.Time `json:"timestamp"`
	Fresh     bool      `json:"fresh"`
}

type PyDepGetter func(FQN string, entityID string, timestamp string, val *PyVal) string

func PyExecReq(jsonPayload string, p PyDepGetter) (ExecRequest, error) {
	dg := func(FQN string, entityID string, timestamp time.Time) (api.Value, error) {
		pv := PyVal{}
		if err := p(FQN, entityID, timestamp.Format(time.RFC3339), &pv); err != "" {
			return api.Value{}, fmt.Errorf("%v", err)
		}

		ret := api.Value{
			Timestamp: pv.Timestamp,
			Fresh:     pv.Fresh,
		}
		if pv.Value != "" {
			err := json.Unmarshal([]byte(pv.Value), &ret.Value)
			if err != nil {
				return api.Value{}, fmt.Errorf("failed to unmarshal value: %w", err)
			}
		}
		return ret, nil
	}

	logger := funcr.New(func(prefix, args string) {
		fmt.Println(prefix, args)
	}, funcr.Options{})

	ret := ExecRequest{
		DependencyGetter: dg,
		Logger:           logger,
	}
	err := json.Unmarshal([]byte(jsonPayload), &ret.Payload)
	if err != nil {
		return ret, fmt.Errorf("failed to unmarshal payload: %w", err)
	}
	return ret, nil
}

func JsonAny(o any, field string) string {
	v := reflect.ValueOf(o)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	b, err := json.Marshal(v.FieldByName(field).Interface())
	if err != nil {
		panic(err)
	}
	return string(b)
}

func PyTime(str string, layout string) (time.Time, error) {
	if layout == "" {
		layout = time.RFC3339
	}
	return time.Parse(layout, str)
}
func PyTimeRFC3339(t time.Time) string {
	return t.Format(time.RFC3339)
}

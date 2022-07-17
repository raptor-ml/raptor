/*
Copyright (c) 2022 Raptor.

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

package pyexp

import (
	"fmt"
	"github.com/sourcegraph/starlight/convert"
	"go.starlark.net/lib/proto"
	sTime "go.starlark.net/lib/time"
	"go.starlark.net/resolve"
	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
	"time"
)

// HandlerFuncName is the name of the function that the use need to implement to handle the request.
const HandlerFuncName = "handler"

// Find handler in the code, if not found return empty string
func programHandler(file *syntax.File, altHandler string) string {
	for _, stmt := range file.Stmts {
		if def, ok := stmt.(*syntax.DefStmt); ok {
			if def.Name.Name == HandlerFuncName || (altHandler != "" && def.Name.Name == altHandler) {
				if fn, ok := def.Function.(*resolve.Function); ok {
					if fn.HasKwargs && !fn.HasVarargs {
						return def.Name.Name
					}
				}
			}
		}
	}

	return ""
}

// Parse and convert return value. Can be a single value, or a tuple of value, timestamp, entity_id
func parseHandlerResults(returnedValue starlark.Value, thread *starlark.Thread) (val any, ts time.Time, entityID string, err error) {
	ts = nowf(thread)

	if returnedValue == starlark.None {
		return
	}
	switch retVals := returnedValue.(type) {
	case starlark.Tuple:
		val, err = starToGo(retVals[0])
		if err != nil {
			return
		}

		// Second item is timestamp (RFC3339)
		if retVals.Len() > 1 {
			if t, ok := retVals[1].(starlark.String); ok {
				ts, err = time.Parse(time.RFC3339, string(t))
				if err != nil {
					err = fmt.Errorf("returned timestamp(%v) was not PyExp's Time or RFC3339", t)
					return
				}
			} else if t, ok := retVals[1].(sTime.Time); ok {
				ts = time.Time(t)
			} else {
				err = fmt.Errorf("program returned a tuple with an invalid timestamp: %v", retVals[1])
				return
			}
		}

		// Third param is entityID and must be a string
		if retVals.Len() > 2 {
			var ok bool
			entityID, ok = convert.FromValue(retVals[2]).(string)
			if !ok {
				err = fmt.Errorf("program returned a non string value as entity_id (third return tuple item)")
				return
			}
		}
	default:
		val, err = starToGo(returnedValue)
		if err != nil {
			return
		}
	}
	return
}

func recursiveToValue(input any) (out starlark.Value, err error) {
	if err != nil {
		return nil, err
	}
	if input == nil {
		return starlark.None, nil
	}
	switch v := input.(type) {
	case map[string]any:
		dict := starlark.Dict{}
		for k, v := range v {
			key, err := convert.ToValue(k)
			if err != nil {
				return nil, err
			}
			val, err := recursiveToValue(v)
			if err != nil {
				return nil, err
			}
			err = dict.SetKey(key, val)
			if err != nil {
				return nil, err
			}
		}
		return &dict, nil
	case []any:
		out := make([]starlark.Value, len(v))
		for i := 0; i < len(v); i++ {
			val, err := recursiveToValue(v[i])
			if err != nil {
				return nil, err
			}
			out[i] = val
		}
		return starlark.NewList(out), nil
	case *proto.Message:
		return protoToMap(v)
	default:
		return convert.ToValue(input)
	}
}

func protoToMap(p *proto.Message) (starlark.Value, error) {
	ret := starlark.Dict{}
	for _, a := range p.AttrNames() {
		vv, err := p.Attr(a)
		if err != nil {
			return nil, fmt.Errorf("failed to get attribute %v: %v", a, err)
		}
		if f, ok := vv.(*proto.Message); ok {
			vv, err = recursiveToValue(f)
			if err != nil {
				return nil, err
			}
		}
		err = ret.SetKey(starlark.String(a), vv)
		if err != nil {
			return nil, err
		}
	}
	return &ret, nil
}

func requestToKwargs(req ExecRequest) ([]starlark.Tuple, error) {
	var payload starlark.Value
	var err error
	if req.Payload == nil {
		payload = starlark.None
	} else {
		payload, err = recursiveToValue(req.Payload)
		if err != nil {
			return nil, err
		}
	}

	return []starlark.Tuple{
		{starlark.String("payload"), payload},
		{starlark.String("headers"), headersToStarDict(req.Headers)},
		{starlark.String("entity_id"), starlark.String(req.EntityID)},
		{starlark.String("timestamp"), sTime.Time(req.Timestamp)},
	}, nil
}

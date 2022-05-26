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

package pyexp

import (
	"fmt"
	"github.com/sourcegraph/starlight/convert"
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
func parseHandlerResults(returnedValue starlark.Value) (val any, ts time.Time, entityID string, err error) {
	ts = time.Now()

	if returnedValue == starlark.None {
		return
	}
	switch x := returnedValue.(type) {
	case starlark.Tuple:
		val, err = starToGo(x[0])
		if err != nil {
			return
		}

		// Second item is timestamp (RFC3339)
		if x.Len() > 1 {
			timeStr := x[1]
			if sTs, ok := x[1].(sTime.Time); ok {
				ts = time.Time(sTs)
			}
			err = fmt.Errorf("program returned a tuple with an invalid timestamp: %v", timeStr)
			return
		}

		// Third param is entityID and must be a string
		if x.Len() > 2 {
			var ok bool
			entityID, ok = convert.FromValue(x[2]).(string)
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

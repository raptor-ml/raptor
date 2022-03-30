package pyexp

import (
	"fmt"
	"github.com/sourcegraph/starlight/convert"
	sTime "go.starlark.net/lib/time"
	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
	"time"
)

// Check if the `handler:` function is implemented in the code
func isHandlerImplemented(file *syntax.File) bool {
	for _, stmt := range file.Stmts {
		if def, ok := stmt.(*syntax.DefStmt); ok {
			if def.Name.Name == HandlerFuncName {
				return true
			}
		}
	}

	return false
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

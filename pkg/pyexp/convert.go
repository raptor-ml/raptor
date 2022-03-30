package pyexp

import (
	"fmt"
	"github.com/natun-ai/natun/pkg/errors"
	sTime "go.starlark.net/lib/time"
	"go.starlark.net/starlark"
	"time"
)

func starToGo(val any) (any, error) {
	if val == nil {
		return nil, nil
	}

	switch v := val.(type) {
	case starlark.String:
		return string(v), nil
	case starlark.Int:
		return int(v.BigInt().Int64()), nil
	case starlark.Float:
		return float64(v), nil
	case *starlark.List:
		iter := v.Iterate()
		defer iter.Done()
		var elems []any
		var x starlark.Value
		for iter.Next(&x) {
			nv, err := starToGo(x)
			if err != nil {
				return nil, err
			}
			elems = append(elems, nv)
		}
		return elems, nil
	case sTime.Time:
		return time.Time(v), nil
	case starlark.NoneType:
		return nil, nil
	default:
		if v, ok := v.(fmt.Stringer); ok {
			return v.String(), nil
		}
		return nil, errors.ErrUnsupportedPrimitiveError
	}
}

func headersToStarDict(h map[string][]string) starlark.Value {
	headers := starlark.NewDict(len(h))
	for k, v := range h {
		var vls []starlark.Value
		for _, v := range v {
			vls = append(vls, starlark.String(v))
		}
		_ = headers.SetKey(starlark.String(k), starlark.NewList(vls))
	}
	return headers
}

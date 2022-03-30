package pyexp

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/natun-ai/natun/pkg/api"
	"github.com/natun-ai/natun/pkg/errors"
	"github.com/qri-io/starlib/bsoup"
	"github.com/qri-io/starlib/encoding/base64"
	"github.com/qri-io/starlib/geo"
	"github.com/qri-io/starlib/hash"
	"github.com/qri-io/starlib/html"
	"github.com/qri-io/starlib/re"
	"github.com/sourcegraph/starlight/convert"
	sJson "go.starlark.net/lib/json"
	sMath "go.starlark.net/lib/math"
	sTime "go.starlark.net/lib/time"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
	"time"
)

// HandlerFuncName is the name of the function that the use need to implement to handle the request.
const HandlerFuncName = "handler"
const localKeyContext = "go.context"

func init() {
	starlark.Universe["json"] = sJson.Module
	starlark.Universe["time"] = sTime.Module
	starlark.Universe["math"] = sMath.Module
	starlark.Universe["struct"] = starlark.NewBuiltin("struct", starlarkstruct.Make)

	rer, _ := re.LoadModule()
	hashr, _ := hash.LoadModule()
	geor, _ := geo.LoadModule()
	bsoupr, _ := bsoup.LoadModule()
	base64r, _ := base64.LoadModule()
	htmlr, _ := html.LoadModule()
	starlark.Universe["re"] = rer["re"]
	starlark.Universe["hash"] = hashr["hash"]
	starlark.Universe["geo"] = geor["geo"]
	starlark.Universe["bsoup"] = bsoupr["bsoup"]
	starlark.Universe["html"] = htmlr["html"]
	starlark.Universe["base64"] = base64r["base64"]
}

type ExecRequest struct {
	Context   context.Context
	Headers   map[string][]string
	Payload   any
	EntityID  string
	Fqn       string
	Timestamp time.Time
}

// Runtime is the starlark runtime for the PyExp.
type Runtime interface {
	api.Logger
	Exec(ExecRequest) (value any, timestamp time.Time, entityID string, err error)
	Engine() api.Engine
}

type runtime struct {
	fqn      string
	program  *starlark.Program
	builtins starlark.StringDict
	engine   api.Engine
	logger   logr.Logger
}

func (r *runtime) Engine() api.Engine {
	return r.engine
}

func (r *runtime) Logger() logr.Logger {
	return r.logger
}

func getLogger(e api.Engine) logr.Logger {
	if l, ok := e.(api.Logger); ok {
		return l.Logger()
	}

	panic("engine does not implement Logger")
}

// New returns a new PyExp runtime
func New(FQN, program string, e api.Engine) (Runtime, error) {
	d := &runtime{
		fqn:      FQN,
		engine:   e,
		logger:   getLogger(e).WithName(fmt.Sprintf("%s.pyexp", FQN)),
		builtins: starlark.StringDict{},
	}

	// This dictionary defines the pre-declared environment.
	d.builtins["f"] = starlark.NewBuiltin("f", d.GetFeature)
	d.builtins["get_feature"] = starlark.NewBuiltin("get_feature", d.GetFeature)
	d.builtins["set_feature"] = starlark.NewBuiltin("set_feature", d.SetFeature)
	d.builtins["update_feature"] = starlark.NewBuiltin("update_feature", d.Update)
	d.builtins["append_feature"] = starlark.NewBuiltin("append_feature", d.AppendFeature)
	d.builtins["incr_feature"] = starlark.NewBuiltin("incr_feature", d.Incr)
	d.builtins["payload"] = starlark.None
	d.builtins["headers"] = starlark.None
	d.builtins["feature_fqn"] = starlark.None
	d.builtins["entity_id"] = starlark.None
	d.builtins["timestamp"] = starlark.None

	// Parse, resolve, and compile a Starlark source file.
	f, p, err := starlark.SourceProgram(fmt.Sprintf("%s.star", d.fqn), program, d.builtins.Has)
	if err != nil {
		return nil, err
	}

	if !isHandlerImplemented(f) {
		return nil, fmt.Errorf("`%s` func has not declared and is required by the Natun spec", HandlerFuncName)
	}

	d.program = p
	return d, nil
}

func (r *runtime) SetFeature(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var fqn, entityID string
	var val any
	var ts = sTime.Time(time.Now())
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "fqn", &fqn, "entity_id", &entityID, "val", &val, "timestamp?", &ts); err != nil {
		return starlark.None, err
	}

	val, err := starToGo(val)
	if err != nil {
		return nil, err
	}

	ctx, ok := t.Local(localKeyContext).(context.Context)
	if !ok {
		return nil, errors.ErrInvalidPipelineContext
	}

	return starlark.None, r.engine.Set(ctx, fqn, entityID, val, time.Time(ts))
}
func (r *runtime) Update(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var fqn, entityID string
	var val any
	var ts = sTime.Time(time.Now())
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "fqn", &fqn, "entity_id", &entityID, "val", &val, "timestamp?", &ts); err != nil {
		return starlark.None, err
	}

	val, err := starToGo(val)
	if err != nil {
		return nil, err
	}

	ctx, ok := t.Local(localKeyContext).(context.Context)
	if !ok {
		return nil, errors.ErrInvalidPipelineContext
	}
	return starlark.None, r.engine.Update(ctx, fqn, entityID, val, time.Time(ts))
}
func (r *runtime) Incr(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var fqn, entityID string
	var by any
	var ts = sTime.Time(time.Now())
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "fqn", &fqn, "entity_id", &entityID, "by", &by, "timestamp?", &ts); err != nil {
		return starlark.None, err
	}

	by, err := starToGo(by)
	if err != nil {
		return nil, err
	}
	ctx, ok := t.Local(localKeyContext).(context.Context)
	if !ok {
		return nil, errors.ErrInvalidPipelineContext
	}
	return starlark.None, r.engine.Incr(ctx, fqn, entityID, by, time.Time(ts))
}

func (r *runtime) AppendFeature(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var fqn, entityID string
	var val any
	var tm = sTime.Time(time.Now())
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "fqn", &fqn, "entity_id", &entityID, "val", &val, "timestamp?", &tm); err != nil {
		return starlark.None, err
	}

	if _, ok := val.(starlark.List); ok {
		return nil, errors.ErrUnsupportedPrimitiveError
	}

	val, err := starToGo(val)
	if err != nil {
		return nil, err
	}

	ctx, ok := t.Local(localKeyContext).(context.Context)
	if !ok {
		return nil, errors.ErrInvalidPipelineContext
	}
	return starlark.None, r.engine.Append(ctx, fqn, entityID, val, time.Time(tm))
}

func (r *runtime) GetFeature(t *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var fqn string
	var entityID string
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "fqn", &fqn, "entity_id", &entityID); err != nil {
		return nil, err
	}

	// TODO protect cyclic fetch
	if r.fqn == fqn {
		return nil, fmt.Errorf("cyclic Get: you tried to fetch the feature your'e in")
	}

	ctx, ok := t.Local(localKeyContext).(context.Context)
	if !ok {
		return nil, errors.ErrInvalidPipelineContext
	}

	val, _, err := r.engine.Get(ctx, fqn, entityID)
	if err != nil {
		return nil, err
	}
	if val.Value == nil {
		return nil, fmt.Errorf("feature value not found for this fqn and entity id")
	}

	// Return the value and the timestamp
	starlarkVal, err := convert.ToValue(val.Value)
	if err != nil {
		return nil, err
	}

	return starlark.Tuple{
		starlarkVal,
		sTime.Time(val.Timestamp),
	}, nil

}

func (r *runtime) Exec(req ExecRequest) (any, time.Time, string, error) {
	// Prepare globals
	predeclared, err := requestToPredeclared(req, r.builtins)
	if err != nil {
		return nil, time.Now(), "", err
	}

	// Create a Thread and redefine the behavior of the built-in 'print' function.
	thread := &starlark.Thread{
		Name:  r.fqn,
		Print: func(_ *starlark.Thread, msg string) { r.logger.WithName("expr").Info(msg) },
	}
	thread.SetLocal(localKeyContext, req.Context)

	// Execute the program
	outGlobals, err := r.program.Init(thread, predeclared)
	outGlobals.Freeze()

	if err != nil {
		if evalErr, ok := err.(*starlark.EvalError); ok {
			r.logger.WithValues("backtrace", evalErr.Backtrace()).Error(evalErr, "execution failed")
		} else {
			r.logger.Error(err, "execution failed")
		}
		return nil, time.Now(), "", err
	}

	// Call the handler
	v, err := starlark.Call(thread, outGlobals[HandlerFuncName], nil, nil)
	if err != nil {
		return nil, time.Now(), "", err
	}

	// Convert and validate the returned value
	returnedValue, outTs, outEntityID, err := parseHandlerResults(v)
	if err != nil {
		return nil, time.Now(), "", err
	}
	if req.EntityID != "" && outEntityID != "" && outEntityID != req.EntityID {
		err := fmt.Errorf("execution returned entity id %s, but the request was for entity id %s", outEntityID, req.EntityID)
		return nil, outTs, req.EntityID, err
	}
	if req.EntityID == "" && outEntityID == "" {
		return nil, time.Now(), "", fmt.Errorf("this program must return an entity_id along with the value")
	}

	return returnedValue, outTs, outEntityID, nil
}

func requestToPredeclared(req ExecRequest, builtins starlark.StringDict) (starlark.StringDict, error) {
	var payload starlark.Value
	if req.Payload == nil {
		payload = starlark.None
	} else {
		var err error
		payload, err = convert.ToValue(req.Payload)
		if err != nil {
			return nil, err
		}
	}

	// Create the globals dict
	globals := starlark.StringDict{}
	// Set builtins types
	for k, v := range builtins {
		globals[k] = v
	}

	// Set per invocation environment for the script
	globals["headers"] = headersToStarDict(req.Headers)
	globals["payload"] = payload
	globals["entity_id"] = starlark.String(req.EntityID)
	globals["feature_fqn"] = starlark.String(req.Fqn)
	globals["timestamp"] = sTime.Time(req.Timestamp)
	return globals, nil
}

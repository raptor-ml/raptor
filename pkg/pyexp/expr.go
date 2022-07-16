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
	"context"
	"errors"
	"fmt"
	"github.com/qri-io/starlib/bsoup"
	"github.com/qri-io/starlib/encoding/base64"
	"github.com/qri-io/starlib/geo"
	"github.com/qri-io/starlib/hash"
	"github.com/qri-io/starlib/html"
	"github.com/qri-io/starlib/re"
	"github.com/raptor-ml/natun/api"
	sJson "go.starlark.net/lib/json"
	sMath "go.starlark.net/lib/math"
	sTime "go.starlark.net/lib/time"
	"go.starlark.net/resolve"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
	"strings"
)

func init() {
	starlark.Universe["json"] = sJson.Module
	starlark.Universe["time"] = sTime.Module
	starlark.Universe["math"] = sMath.Module
	starlark.Universe["struct"] = starlark.NewBuiltin("struct", starlarkstruct.Make)

	// Clone the time module, and override the now function
	tm := &(*sTime.Module)
	tm.Members["now"] = starlark.NewBuiltin("now", now)
	starlark.Universe["time"] = tm

	resolve.AllowSet = true

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

// Runtime is the starlark runtime for the PyExp.
type Runtime interface {
	Exec(ExecRequest) (*ExecResponse, error)
	ExecWithEngine(ctx context.Context, req ExecRequest, e api.Engine) (*ExecResponse, error)
	DiscoverDependencies() ([]string, error)
}

type runtime struct {
	program  *starlark.Program
	builtins starlark.StringDict
	fqn      string
	handler  string
}

// New returns a new PyExp runtime
func New(program string, fqn string) (Runtime, error) {
	d := &runtime{
		fqn:      fqn,
		builtins: starlark.StringDict{},
	}

	// This dictionary defines the pre-declared environment.
	d.builtins["f"] = starlark.NewBuiltin("f", d.GetFeature)
	d.builtins["get_feature"] = starlark.NewBuiltin("get_feature", d.GetFeature)
	d.builtins["set_feature"] = starlark.NewBuiltin("set_feature", d.SetFeature)
	d.builtins["update_feature"] = starlark.NewBuiltin("update_feature", d.Update)
	d.builtins["append_feature"] = starlark.NewBuiltin("append_feature", d.AppendFeature)
	d.builtins["incr_feature"] = starlark.NewBuiltin("incr_feature", d.Incr)

	// Parse, resolve, and compile a Starlark source file.
	f, p, err := starlark.SourceProgram("<pyexp>", program, d.builtins.Has)
	if err != nil {
		return nil, err
	}

	if p.NumLoads() > 0 {
		return nil, errors.New("pyexp cannot load files")
	}

	// Todo try multiple alt handlers, i.e. convert snake to camel case
	altHandler := strings.SplitN(fqn, ".", 2)[0]
	altHandler = strings.ReplaceAll(altHandler, "-", "_")
	d.handler = programHandler(f, altHandler)
	if d.handler == "" {
		return nil, fmt.Errorf("`%s` func or `%s` has not declared and is required by the Natun spec", HandlerFuncName, altHandler)
	}

	d.program = p
	return d, nil
}

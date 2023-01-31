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

package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/die-net/lrucache"
	"github.com/gregjones/httpcache"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/raptor-ml/raptor/api"
	manifests "github.com/raptor-ml/raptor/api/v1alpha1"
	"github.com/raptor-ml/raptor/pkg/plugins"
	"io"
	"net/http"
	"strings"
	"time"
)

const name = "rest"

func init() {
	plugins.FeatureAppliers.Register(name, FeatureApply)
}

type config struct {
	//+optional
	URL string `mapstructure:"url"`
	//+optional
	Method string `mapstructure:"method"`
	//+optional
	Body string `mapstructure:"body"`
	//+optional
	Headers http.Header `mapstructure:"headers"`

	runtime api.RuntimeManager
	client  http.Client
}

var httpMemoryCache = lrucache.New(500<<(10*2), 60*15) // 500MB; 15min

func FeatureApply(fd api.FeatureDescriptor, builder manifests.FeatureBuilder, pl api.Pipeliner, engine api.ExtendedManager) error {
	if fd.DataSource == "" {
		return fmt.Errorf("DataSource must be set for `%s` builder", name)
	}
	if len(fd.Aggr) > 0 {
		return fmt.Errorf("aggregation is not supported for `%s` builder", name)
	}

	src, err := engine.GetDataSource(fd.DataSource)
	if err != nil {
		return fmt.Errorf("failed to get DataSource: %v", err)
	}

	if src.Kind != name {
		return fmt.Errorf("DataSource must be of type `%s`. got `%s`", name, src.Kind)
	}

	cfg := config{}
	err = src.Config.Unmarshal(&cfg)
	if err != nil {
		return fmt.Errorf("failed to unmarshal DataSource config: %v", err)
	}

	timeout := time.Duration(float32(fd.Timeout) * 0.8)
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	tr := httpcache.NewTransport(httpMemoryCache)
	tr.Transport = &retryablehttp.RoundTripper{}

	cfg.runtime = engine
	cfg.client = http.Client{
		Transport: httpcache.NewTransport(httpMemoryCache),
		Timeout:   timeout,
	}

	if fd.Freshness <= 0 {
		pl.AddPreGetMiddleware(0, cfg.getMiddleware)
	} else {
		pl.AddPostGetMiddleware(0, cfg.getMiddleware)
	}
	return nil
}

func (rest *config) getMiddleware(next api.MiddlewareHandler) api.MiddlewareHandler {
	return func(ctx context.Context, fd api.FeatureDescriptor, keys api.Keys, val api.Value) (api.Value, error) {
		cache, cacheOk := ctx.Value(api.ContextKeyFromCache).(bool)
		if cacheOk && cache && val.Fresh && !fd.ValidWindow() {
			return next(ctx, fd, keys, val)
		}

		var body io.Reader
		if rest.Body != "" {
			body = strings.NewReader(rest.Body)
		}

		u := strings.ReplaceAll(rest.URL, "{keys}", keys.String())
		//todo: add support for specific key placeholder

		req, err := http.NewRequest(rest.Method, u, body)
		if err != nil {
			return val, err
		}
		req = req.WithContext(ctx)
		req.Header = rest.Headers

		resp, err := rest.client.Do(req)
		if err != nil {
			return val, err
		}

		defer resp.Body.Close()
		buf, err := io.ReadAll(resp.Body)
		if err != nil {
			return val, err
		}

		// Try to parse the input as JSON. If successful pass the unmarshalled object, otherwise pass the body as-is
		var payload map[string]any
		err = json.NewDecoder(bytes.NewReader(buf)).Decode(&payload)
		if err != nil {
			return val, fmt.Errorf("failed to parse response as JSON: %w", err)
		}

		val, keys, err = rest.runtime.ExecuteProgram(ctx, fd.RuntimeEnv, fd.FQN, keys, payload, val.Timestamp, true)
		if err != nil {
			return val, err
		}

		return next(ctx, fd, keys, val)
	}
}

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

package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/die-net/lrucache"
	"github.com/gregjones/httpcache"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/raptor-ml/natun/api"
	"github.com/raptor-ml/natun/pkg/plugins"
	"github.com/raptor-ml/natun/pkg/pyexp"
	"io"
	"net/http"
	"strings"
	"time"
)

func init() {
	const name = "rest"
	plugins.FeatureAppliers.Register(name, FeatureApply)
}

type Spec struct {
	//+optional
	URL string `json:"url"`
	//+optional
	Method string `json:"method"`
	//+optional
	Body string `json:"body"`
	//+optional
	Headers http.Header `json:"headers"`
	// +kubebuilder:validation:Required
	Expression string `json:"pyexp"`
}

var httpMemoryCache = lrucache.New(500<<(10*2), 60*15) // 500MB; 15min

func FeatureApply(md api.Metadata, builderSpec []byte, api api.FeatureAbstractAPI, engine api.EngineWithConnector) error {
	spec := &Spec{}
	err := json.Unmarshal(builderSpec, spec)
	if err != nil {
		return fmt.Errorf("failed to unmarshal expression spec: %w", err)
	}

	if spec.Expression == "" {
		return fmt.Errorf("expression is empty")
	}
	runtime, err := pyexp.New(spec.Expression, md.FQN)
	if err != nil {
		return fmt.Errorf("failed to create expression runtime: %w", err)
	}

	timeout := time.Duration(float32(md.Timeout) * 0.8)
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	tr := httpcache.NewTransport(httpMemoryCache)
	tr.Transport = &retryablehttp.RoundTripper{}

	r := rest{
		engine:  engine,
		runtime: runtime,
		client: http.Client{
			Transport: httpcache.NewTransport(httpMemoryCache),
			Timeout:   timeout,
		},
		method:  spec.Method,
		url:     spec.URL,
		body:    spec.Body,
		headers: spec.Headers,
	}

	if md.Freshness <= 0 {
		api.AddPreGetMiddleware(0, r.getMiddleware)
	} else {
		api.AddPostGetMiddleware(0, r.getMiddleware)
	}
	return nil
}

type rest struct {
	runtime pyexp.Runtime
	engine  api.Engine
	client  http.Client
	url     string
	method  string
	headers http.Header
	body    string
}

func (rp *rest) getMiddleware(next api.MiddlewareHandler) api.MiddlewareHandler {
	return func(ctx context.Context, md api.Metadata, entityID string, val api.Value) (api.Value, error) {
		cache, cacheOk := ctx.Value(api.ContextKeyFromCache).(bool)
		if cacheOk && cache && val.Fresh && !md.ValidWindow() {
			return next(ctx, md, entityID, val)
		}

		var body io.Reader
		if rp.body != "" {
			body = strings.NewReader(rp.body)
		}

		u := strings.ReplaceAll(rp.url, "{entity_id}", entityID)

		req, err := http.NewRequest(rp.method, u, body)
		if err != nil {
			return val, err
		}
		req.WithContext(ctx)
		req.Header = rp.headers

		resp, err := rp.client.Do(req)
		if err != nil {
			return val, err
		}

		defer resp.Body.Close()
		buf, err := io.ReadAll(resp.Body)
		if err != nil {
			return val, err
		}

		// Try to parse the input as JSON. If successful pass the unmarshalled object, otherwise pass the body as-is
		var payload any
		var unmarshalledPayload map[string]any
		err = json.NewDecoder(bytes.NewReader(buf)).Decode(&unmarshalledPayload)
		if err != nil {
			payload = buf
		} else {
			payload = unmarshalledPayload
		}

		ret, err := rp.runtime.ExecWithEngine(ctx, pyexp.ExecRequest{
			Headers:   resp.Header,
			Payload:   payload,
			EntityID:  entityID,
			Timestamp: val.Timestamp,
			Logger:    api.LoggerFromContext(ctx),
		}, rp.engine)
		if err != nil {
			return val, err
		}

		if ret.Value != nil {
			if ret.Timestamp.IsZero() && !val.Timestamp.IsZero() {
				ret.Timestamp = val.Timestamp
			}
			val = api.Value{
				Value:     ret.Value,
				Timestamp: ret.Timestamp,
				Fresh:     true,
			}
		}

		return next(ctx, md, entityID, val)
	}
}

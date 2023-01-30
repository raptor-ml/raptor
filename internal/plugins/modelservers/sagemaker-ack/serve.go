/*
 * Copyright (c) 2022 RaptorML authors.
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

package sagemaker_ack

import (
	"context"
	"encoding/json"
	"fmt"
	awsCfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sagemakerruntime"
	"github.com/raptor-ml/raptor/api"
	"strings"
	"time"
)

func (*ack) Serve(ctx context.Context, fd api.FeatureDescriptor, md api.ModelDescriptor, val api.Value) (api.Value, error) {
	ret := api.Value{}

	cfg := config{}
	err := cfg.Parse(md.InferenceConfig)
	if err != nil {
		return ret, fmt.Errorf("failed to parse inference config: %w", err)
	}
	if cfg.ModelName == "" {
		cfg.ModelName = strings.ReplaceAll(fd.FQN, ".", "-")
	}

	aCfg, err := awsCfg.LoadDefaultConfig(ctx)
	if err != nil {
		return ret, fmt.Errorf("failed to load default AWS config: %w", err)
	}
	sr := sagemakerruntime.NewFromConfig(aCfg)

	fs, ok := val.Value.(map[string]api.Value)
	if !ok {
		return ret, fmt.Errorf("cannot cast feature set to map[string]api.Value")
	}

	featureSetMap := make(map[string]any)
	for k, v := range fs {
		featureSetMap[k] = v.Value
	}

	body, err := json.Marshal(featureSetMap)
	if err != nil {
		return ret, fmt.Errorf("failed to marshal FeatureSet: %w", err)
	}

	jsonMimeType := "application/json"
	resp, err := sr.InvokeEndpoint(ctx, &sagemakerruntime.InvokeEndpointInput{
		Body:         body,
		EndpointName: &cfg.ModelName,
		Accept:       &jsonMimeType,
		ContentType:  &jsonMimeType,
	})
	if err != nil {
		return ret, fmt.Errorf("failed to invoke endpoint: %w", err)
	}

	var result any
	err = json.Unmarshal(resp.Body, &result)
	if err != nil {
		return ret, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	ret.Timestamp = time.Now()
	ret.Value, err = api.NormalizeAny(result)
	if err != nil {
		return ret, fmt.Errorf("failed to normalize result: %w", err)
	}
	return ret, nil
}

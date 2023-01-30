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

package sagemaker_ack

import (
	"fmt"
	manifests "github.com/raptor-ml/raptor/api/v1alpha1"
)

type config struct {
	// ModelName is the name of the model in SageMaker.
	// if not set, the name of the Raptor model will be used.
	// +optional
	ModelName string `mapstructure:"modelName,omitempty"`

	// Region is the AWS region to use for the SageMaker endpoint.
	// +optional
	Region string `mapstructure:"region"`

	// InstanceType is the instance type to use for the SageMaker endpoint.
	// If not specified, we'll use serverless deployment.
	// For more info: https://aws.amazon.com/sagemaker/pricing/instance-types/
	// +optional
	InstanceType string `mapstructure:"instanceType"`

	// InitialInstanceCount is the initial number of instances to use for the SageMaker endpoint.
	// If not specified, it will default to 1.
	// +optional
	InitialInstanceCount int `mapstructure:"initialInstanceCount"`

	// ExecutionRoleARN is the ARN of the IAM role to use for the model.
	// The Amazon Resource Name (ARN) of the IAM role that SageMaker can assume
	// to access model artifacts and docker image for deployment on ML compute instances
	// or for batch transform jobs. Deploying on ML compute instances is part of
	// model hosting. For more information, see SageMaker Roles (https://docs.aws.amazon.com/sagemaker/latest/dg/sagemaker-roles.html).
	//
	// To be able to pass this role to SageMaker, the caller of this API must have
	// the iam:PassRole permission.
	// +required
	ExecutionRoleARN string `mapstructure:"executionRoleARN"`

	// ServerlessMaxConcurrency is the maximum number of concurrent invocations for a serverless endpoint.
	// If this is set, we'll create a serverless deployment.
	// Default is 20.
	// +optional
	ServerlessMaxConcurrency int `mapstructure:"serverlessMaxConcurrency"`

	// ServerlessMemorySizeInMB is the amount of memory to use for a serverless endpoint.
	// If this is set, we'll create a serverless deployment.
	// Default is 2048.
	// +optional
	ServerlessMemorySizeInMB int `mapstructure:"serverlessMemorySizeInMB"`

	serverless bool
}

func (cfg *config) Parse(pc manifests.ParsedConfig) error {
	err := pc.Unmarshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to parse ACK Inference config: %v", err)
	}

	// Check for required fields
	if cfg.Region == "" {
		return fmt.Errorf("region must be set")
	}
	if cfg.ExecutionRoleARN == "" {
		return fmt.Errorf("executionRoleARN must be set")
	}

	// Set defaults
	if cfg.InitialInstanceCount == 0 {
		cfg.InitialInstanceCount = 1
	}

	//serverless
	if cfg.InstanceType == "" || cfg.ServerlessMaxConcurrency > 0 || cfg.ServerlessMemorySizeInMB > 0 {
		cfg.serverless = true
	}
	if cfg.ServerlessMaxConcurrency == 0 {
		cfg.ServerlessMaxConcurrency = 20
	}
	if cfg.ServerlessMaxConcurrency > 200 {
		return fmt.Errorf("serverlessMaxConcurrency must be <= 200")
	}
	if cfg.ServerlessMemorySizeInMB == 0 {
		cfg.ServerlessMemorySizeInMB = 2048
	}
	switch cfg.ServerlessMemorySizeInMB {
	case 1024, 2048, 3072, 4096, 5120, 6144:
	default:
		return fmt.Errorf("serverlessMemorySizeInMB must be one of 1024, 2048, 3072, 4096, 5120, 6144")
	}

	return nil
}

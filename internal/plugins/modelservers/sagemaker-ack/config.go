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
	"context"
	"fmt"
	manifests "github.com/raptor-ml/raptor/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type config struct {
	// ModelName is the name of the model in SageMaker
	// if not set, the name of the Raptor model will be used
	// +optional
	ModelName string `mapstructure:"modelName,omitempty"`

	// ContainerImage is the container image to use for the model
	// if not specified, the default image for the model framework will be used
	// +optional
	ContainerImage string `mapstructure:"containerImage,omitempty"`

	// Region is the AWS region to use for the SageMaker endpoint
	// +required
	Region string `mapstructure:"region"`

	// InstanceType is the instance type to use for the SageMaker endpoint
	// If not specified, it will default to ml.t2.medium
	// +optional
	InstanceType string `mapstructure:"instanceType"`

	// InitialInstanceCount is the initial number of instances to use for the SageMaker endpoint
	// If not specified, it will default to 1
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
}

func (cfg *config) Parse(ctx context.Context, model *manifests.Model, client client.Reader) error {
	pc, err := model.ParseInferenceConfig(ctx, client)
	if err != nil {
		return fmt.Errorf("failed to parse inference config: %v", err)
	}

	err = pc.Unmarshal(cfg)
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
	if cfg.ModelName == "" {
		cfg.ModelName = fmt.Sprintf("%s-%s", model.GetNamespace(), model.GetName())
	}
	if cfg.ContainerImage == "" {
		img, err := ImageURI(model.Spec.ModelFramework, cfg.Region, model.Spec.ModelFrameworkVersion)
		if err != nil {
			return fmt.Errorf("failed to get default image for model framework: %v", err)
		}
		cfg.ContainerImage = img
	}
	if cfg.InstanceType == "" {
		cfg.InstanceType = "ml.t2.medium"
	}
	if cfg.InitialInstanceCount == 0 {
		cfg.InitialInstanceCount = 1
	}

	return nil
}

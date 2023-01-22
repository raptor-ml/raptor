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
	"github.com/raptor-ml/raptor/api"
	manifests "github.com/raptor-ml/raptor/api/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func reconcile(ctx context.Context, req api.ModelReconcileRequest) (bool, error) {
	logger := log.FromContext(ctx).WithName("ack")

	cfg := config{}
	err := cfg.Parse(ctx, req.Model, req.Client)
	if err != nil {
		return false, err
	}

	if _, err = req.Client.RESTMapper().RESTMapping(ackModelGVK.GroupKind(), ackModelGVK.Version); err != nil {
		logger.Error(err, "unable to find SageMaker ACK Model")
		return false, fmt.Errorf("unable to find SageMaker ACK Model: %w", err)
	}

	// Create an ACK Model object
	// We are using Unstructured because we don't want to add a dependency on the ACK API
	am := &unstructured.Unstructured{}
	am.SetGroupVersionKind(ackModelGVK)
	am.SetName(req.Model.GetName())
	am.SetNamespace(req.Model.GetNamespace())
	amop, err := ctrl.CreateOrUpdate(ctx, req.Client, am, func() error {
		if err := updateAckModel(am, req, cfg); err != nil {
			return fmt.Errorf("unable to update SageMaker ACK Model: %w", err)
		}
		return ctrl.SetControllerReference(req.Model, am, req.Scheme)
	})
	if err != nil {
		logger.Error(err, "ACK Model reconcile failed")
		return false, err
	}

	// Create an ACK EndpointConfig object
	aec := &unstructured.Unstructured{}
	aec.SetGroupVersionKind(ackEndpointConfigGVK)
	aec.SetName(req.Model.GetName())
	aec.SetNamespace(req.Model.GetNamespace())
	aecop, err := ctrl.CreateOrUpdate(ctx, req.Client, aec, func() error {
		if err := updateAckEndpointConfig(aec, req, cfg); err != nil {
			return fmt.Errorf("unable to update SageMaker ACK EndpointConfig: %w", err)
		}
		return ctrl.SetControllerReference(req.Model, aec, req.Scheme)
	})
	if err != nil {
		logger.Error(err, "ACK EndpointConfig reconcile failed")
		return false, err
	}

	// Create an ACK Endpoint object
	ae := &unstructured.Unstructured{}
	ae.SetGroupVersionKind(ackEndpointGVK)
	ae.SetName(req.Model.GetName())
	ae.SetNamespace(req.Model.GetNamespace())
	aeop, err := ctrl.CreateOrUpdate(ctx, req.Client, ae, func() error {
		if err := updateAckEndpoint(ae, req, cfg); err != nil {
			return fmt.Errorf("unable to update SageMaker ACK Endpoint: %w", err)
		}
		return ctrl.SetControllerReference(req.Model, ae, req.Scheme)
	})
	if err != nil {
		logger.Error(err, "ACK Endpoint reconcile failed")
		return false, err
	}

	return amop != controllerutil.OperationResultNone ||
		aecop != controllerutil.OperationResultNone ||
		aeop != controllerutil.OperationResultNone, nil
}

func updateAckModel(am *unstructured.Unstructured, req api.ModelReconcileRequest, cfg config) error {
	if err := unstructured.SetNestedField(am.Object, cfg.ModelName, "spec", "modelName"); err != nil {
		return err
	}
	if err := unstructured.SetNestedField(am.Object, req.Model.Spec.StorageURI, "spec", "primaryContainer",
		"modelDataURL"); err != nil {
		return err
	}

	if err := unstructured.SetNestedField(am.Object, cfg.ContainerImage, "spec", "primaryContainer",
		"image"); err != nil {
		return err
	}

	if cfg.ExecutionRoleARN != "" {
		if err := unstructured.SetNestedField(am.Object, cfg.ExecutionRoleARN, "spec", "executionRoleARN"); err != nil {
			return err
		}
	}

	return setTags(am, req.Model, cfg)
}

func updateAckEndpointConfig(aec *unstructured.Unstructured, req api.ModelReconcileRequest, cfg config) error {
	if err := unstructured.SetNestedField(aec.Object, cfg.ModelName, "spec", "endpointConfigName"); err != nil {
		return err
	}

	if err := unstructured.SetNestedSlice(aec.Object, []any{
		map[string]any{
			"modelName":            cfg.ModelName,
			"variantName":          "Raptor",
			"initialInstanceCount": int64(cfg.InitialInstanceCount),
			"instanceType":         cfg.InstanceType,
		},
	}, "spec", "productionVariants"); err != nil {
		return err
	}

	return setTags(aec, req.Model, cfg)
}

func updateAckEndpoint(ae *unstructured.Unstructured, req api.ModelReconcileRequest, cfg config) error {
	if err := unstructured.SetNestedField(ae.Object, cfg.ModelName, "spec", "endpointName"); err != nil {
		return err
	}
	if err := unstructured.SetNestedField(ae.Object, cfg.ModelName, "spec", "endpointConfigName"); err != nil {
		return err
	}

	return setTags(ae, req.Model, cfg)
}

func setTags(u *unstructured.Unstructured, model *manifests.Model, cfg config) error {
	tags := []any{
		map[string]any{
			"key":   "k8s.raptor.ml/model",
			"value": model.FQN(),
		},
	}
	if err := unstructured.SetNestedField(u.Object, tags, "spec", "tags"); err != nil {
		return err
	}

	u.SetAnnotations(map[string]string{
		"k8s.raptor.ml/model":     model.FQN(),
		"services.k8s.aws/region": cfg.Region,
	})
	return nil
}

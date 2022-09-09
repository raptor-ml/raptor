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

package controllers

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type coreController struct {
	controller.Controller
}

func (c *coreController) NeedLeaderElection() bool {
	return false
}

func attachCoreConnector(rcl reconcile.Reconciler, obj client.Object, updatesAllowed bool, mgr manager.Manager) error {
	_, err := newCoreController(rcl, obj, updatesAllowed, mgr)
	return err
}

func newCoreController(rcl reconcile.Reconciler, obj client.Object, updatesAllowed bool, mgr manager.Manager) (controller.Controller, error) {
	basec, err := controller.NewUnmanaged("core", mgr, controller.Options{Reconciler: rcl})
	if err != nil {
		return nil, err
	}
	c := &coreController{basec}

	// Predicates
	prct := []predicate.Predicate{predicate.Funcs{GenericFunc: func(genericEvent event.GenericEvent) bool {
		return false
	}}}
	if updatesAllowed {
		prct = append(prct, predicate.GenerationChangedPredicate{})
	} else {
		prct = append(prct, predicate.Funcs{
			UpdateFunc: func(event event.UpdateEvent) bool {
				return false
			},
		})
	}
	src := &source.Kind{Type: obj}
	err = c.Watch(src, new(handler.EnqueueRequestForObject), prct...)
	if err != nil {
		return nil, err
	}

	return c, mgr.Add(c)
}

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

package natun

// Since controller-tools cannot scan internal packages, we're specifying here all the RBAC markers

// Certs
// +kubebuilder:rbac:groups=cert-manager.io,resources=issuers;certificates,verbs=get;create;update;patch;delete;watch;list
// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;update;patch;watch;list
// +kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=validatingwebhookconfigurations,verbs=get;update;patch;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;create;update;patch;list;watch

// Stats
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=nodes,verbs=list;watch;get

// Operator Controllers
// +kubebuilder:rbac:groups=k8s.natun.ai,resources=dataconnectors,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k8s.natun.ai,resources=dataconnectors/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=k8s.natun.ai,resources=dataconnectors/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k8s.natun.ai,resources=features,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k8s.natun.ai,resources=features/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=k8s.natun.ai,resources=features/finalizers,verbs=update

// Engine Controllers
// +kubebuilder:rbac:groups=k8s.natun.ai,resources=features,verbs=get;list;watch
// +kubebuilder:rbac:groups=k8s.natun.ai,resources=dataconnectors,verbs=get;list;watch

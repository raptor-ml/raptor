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

package runner

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func deploymentWithDefaults(dep *appsv1.Deployment) {
	if dep.Spec.Template.Spec.DNSPolicy == "" {
		dep.Spec.Template.Spec.DNSPolicy = corev1.DNSDefault
	}
	if dep.Spec.Template.Spec.RestartPolicy == "" {
		dep.Spec.Template.Spec.RestartPolicy = corev1.RestartPolicyAlways
	}
	if dep.Spec.Template.Spec.TerminationGracePeriodSeconds == nil {
		v := int64(corev1.DefaultTerminationGracePeriodSeconds)
		dep.Spec.Template.Spec.TerminationGracePeriodSeconds = &v
	}
	if dep.Spec.Template.Spec.SchedulerName == "" {
		dep.Spec.Template.Spec.SchedulerName = corev1.DefaultSchedulerName
	}

	if dep.Spec.Strategy.Type == "" {
		dep.Spec.Strategy.Type = appsv1.RollingUpdateDeploymentStrategyType
	}
	if dep.Spec.Strategy.RollingUpdate == nil {
		dep.Spec.Strategy.RollingUpdate = &appsv1.RollingUpdateDeployment{}
	}
	if dep.Spec.Strategy.RollingUpdate.MaxUnavailable == nil {
		dep.Spec.Strategy.RollingUpdate.MaxUnavailable = &intstr.IntOrString{Type: intstr.String, StrVal: "25%"}
	}
	if dep.Spec.Strategy.RollingUpdate.MaxSurge == nil {
		dep.Spec.Strategy.RollingUpdate.MaxSurge = &intstr.IntOrString{Type: intstr.String, StrVal: "25%"}
	}

	if dep.Spec.Template.Spec.SecurityContext == nil {
		t := true

		dep.Spec.Template.Spec.SecurityContext = &corev1.PodSecurityContext{
			RunAsNonRoot: &t,
		}
	}
	if dep.Spec.RevisionHistoryLimit == nil {
		ten := int32(10)
		dep.Spec.RevisionHistoryLimit = &ten
	}
	if dep.Spec.ProgressDeadlineSeconds == nil {
		ten := int32(600)
		dep.Spec.ProgressDeadlineSeconds = &ten
	}

	for i := range dep.Spec.Template.Spec.Containers {
		container := &dep.Spec.Template.Spec.Containers[i]
		if container.TerminationMessagePath == "" {
			container.TerminationMessagePath = corev1.TerminationMessagePathDefault
		}
		if container.TerminationMessagePolicy == "" {
			container.TerminationMessagePolicy = corev1.TerminationMessageReadFile
		}
		if container.ImagePullPolicy == "" {
			container.ImagePullPolicy = corev1.PullAlways
		}
		if container.SecurityContext == nil {
			t := true
			container.SecurityContext = &corev1.SecurityContext{
				RunAsNonRoot: &t,
			}
		}
	}
}

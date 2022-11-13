/*
Copyright 2021 The tKeel Authors.

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

package analysis

import (
	"context"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type Mutating struct {
	*AnalysisTaskReconciler
}

func (m *Mutating) Handle(ctx context.Context, request admission.Request) admission.Response {
	return m.AnalysisTaskReconciler.Mutating(ctx, request)
}

type Validating struct {
	*AnalysisTaskReconciler
}

func (m *Validating) Handle(ctx context.Context, request admission.Request) admission.Response {
	return m.AnalysisTaskReconciler.Validating(ctx, request)
}

// set scheduler for workloads
func (r *AnalysisTaskReconciler) Mutating(ctx context.Context, request admission.Request) admission.Response {
	var workload client.Object
	var key = client.ObjectKey{request.Namespace, request.Name}
	switch request.Kind.Kind {
	case "Deployment":
		var deploy = &appsv1.Deployment{}
		err := r.Get(ctx, key, deploy)
		if err != nil {
			klog.Errorf("get Deployment error, %s", err.Error())
			return admission.Denied("Workload not found.")
		}
		deploy.Spec.Template.Spec.SchedulerName = r.ClusterScheduleConfig.DefaultScheduler
		workload = deploy
	case "StatefulSet":
		var state = &appsv1.StatefulSet{}
		err := r.Get(ctx, key, state)
		if err != nil {
			klog.Errorf("get StatefulSet error, %s", err.Error())
			return admission.Denied("Workload not found.")
		}
		state.Spec.Template.Spec.SchedulerName = r.ClusterScheduleConfig.DefaultScheduler
		workload = state
	default:
		return admission.Allowed("Kind do not match, pass.")
	}

	err := r.Update(ctx, workload)
	if err != nil {
		klog.Errorf("update Workload error, %s", err.Error())
		return admission.Denied("Workload update failed.")
	}
	return admission.Allowed("Workload update was successful.")
}

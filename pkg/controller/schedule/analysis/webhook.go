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
	"encoding/json"
	"net/http"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
			if apierrors.IsNotFound(err) {
				return admission.Denied("Deployment not found.")
			}
			return admission.Denied("Get Deployment error.")
		}
		deploy.Spec.Template.Spec.SchedulerName = r.ClusterScheduleConfig.DefaultScheduler
		workload = deploy
	case "StatefulSet":
		var state = &appsv1.StatefulSet{}
		err := r.Get(ctx, key, state)
		if err != nil {
			klog.Errorf("get StatefulSet error, %s", err.Error())
			if apierrors.IsNotFound(err) {
				return admission.Denied("StatefulSet not found.")
			}
			return admission.Denied("Get StatefulSet error.")
		}
		state.Spec.Template.Spec.SchedulerName = r.ClusterScheduleConfig.DefaultScheduler
		workload = state
	case "DaemonSet":
		var daemon = &appsv1.DaemonSet{}
		err := r.Get(ctx, key, daemon)
		if err != nil {
			klog.Errorf("get DaemonSet error, %s", err.Error())
			if apierrors.IsNotFound(err) {
				return admission.Denied("DaemonSet not found.")
			}
			return admission.Denied("Get DaemonSet error.")
		}
		daemon.Spec.Template.Spec.SchedulerName = r.ClusterScheduleConfig.DefaultScheduler
		workload = daemon
	default:
		return admission.Allowed("Kind do not match, pass.")
	}

	marshaled, err := json.Marshal(workload)
	if err != nil {
		klog.Info("marshal pod error", "error", err, "namespace", request.Namespace, "name", request.Name)
		return admission.Errored(http.StatusInternalServerError, err)
	}
	return admission.PatchResponseFromRaw(request.Object.Raw, marshaled)
}

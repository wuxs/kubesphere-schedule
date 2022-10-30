/*
Copyright 2022.

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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	urlruntime "k8s.io/apimachinery/pkg/util/runtime"
	v1 "k8s.io/client-go/informers/apps/v1"
	corev1informer "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"
	"kubesphere.io/schedule/api/schedule/v1alpha1"
	schedulev1alpha1 "kubesphere.io/schedule/api/schedule/v1alpha1"
	"kubesphere.io/schedule/pkg/client/k8s"
	"kubesphere.io/schedule/pkg/models/schedule"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sync"
)

// AnalysisReconciler reconciles a Analysis object
type AnalysisReconciler struct {
	sync.Mutex
	client.Client
	Scheme *runtime.Scheme

	K8SClient          k8s.Client
	ScheduleClient     schedule.Interface
	DeploymentInformer v1.DeploymentInformer
	NamespaceInformer  corev1informer.NamespaceInformer
	NameSpaceCache     map[string]*v1alpha1.Analysis
}

//+kubebuilder:rbac:groups=schedule.kubesphere.io,resources=analyses,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=schedule.kubesphere.io,resources=analyses/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=schedule.kubesphere.io,resources=analyses/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Analysis object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.0/pkg/reconcile
func (r *AnalysisReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	klog.Infof("[+]---4---")

	analysis := &v1alpha1.Analysis{}
	err := r.Client.Get(ctx, req.NamespacedName, analysis)
	if err != nil {
		return ctrl.Result{}, err
	}

	switch analysis.Spec.Type {
	case v1alpha1.ResourceTypeDeployment:
		r.ReconcileDeploymentAnalysis(ctx, analysis)
		//r.ScheduleClient.CreateAnalysis(analysis.Namespace, analysis.Spec.Target, analysis.Spec.CompletionStrategy)
	case v1alpha1.ResourceTypeNamespace:
		r.ReconcileNamespaceAnalysis(ctx, analysis)
	default:
		klog.Infof("not support resource type", analysis.Spec.Type)
	}

	return ctrl.Result{}, nil
}

func (r *AnalysisReconciler) ReconcileDeploymentAnalysis(ctx context.Context, analysis *schedulev1alpha1.Analysis) {
	_ = log.FromContext(ctx)
	for _, resource := range analysis.Spec.ResourceSelectors {
		if resource.Kind != schedulev1alpha1.ResourceTypeDeployment {
			klog.Errorf("unknown kind", resource)
			continue
		}
		r.ScheduleClient.CreateAnalysis(analysis.Namespace, resource, analysis.Spec.CompletionStrategy)
	}
}

func (r *AnalysisReconciler) ReconcileNamespaceAnalysis(ctx context.Context, analysis *schedulev1alpha1.Analysis) (ctrl.Result, error) {
	_ = log.FromContext(ctx)
	for _, resource := range analysis.Spec.ResourceSelectors {
		if resource.Kind != schedulev1alpha1.ResourceTypeNamespace {
			klog.Errorf("unknown kind", resource)
			continue
		}

		namespace := resource.Name
		if namespace == "" {
			return ctrl.Result{}, nil
		}
		r.NameSpaceCache[namespace] = analysis

		deployments, err := r.K8SClient.Kubernetes().AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			klog.Errorf("get deployment error", err)
			return ctrl.Result{}, err
		}
		for _, deployment := range deployments.Items {
			resource := convertResource(&deployment)
			r.ScheduleClient.CreateAnalysis(namespace, resource, analysis.Spec.CompletionStrategy)
		}
	}

	return ctrl.Result{}, nil
}

func (r *AnalysisReconciler) DeploymentEventHandler() cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			klog.Infof("reciver deployment add event", obj.(*appsv1.Deployment).Name)
			deployment := obj.(*appsv1.Deployment)
			namespace := deployment.Namespace
			if analysis, ok := r.NameSpaceCache[namespace]; ok {
				klog.Infof("create deployment analysis", obj.(*appsv1.Deployment).Name)
				r.ScheduleClient.CreateAnalysis(
					deployment.Namespace,
					convertResource(deployment),
					analysis.Spec.CompletionStrategy)
			} else {
				klog.Infof("skip create deployment analysis", obj.(*appsv1.Deployment).Name)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			klog.Infof("reciver deployment update event", newObj.(*appsv1.Deployment).Name)
		},
		DeleteFunc: func(obj interface{}) {
			klog.Infof("delete deployment delete event", obj.(*appsv1.Deployment).Name)
		},
	}
}

func (r *AnalysisReconciler) NamespaceEventHandler() cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			klog.Infof("reciver namespace add event", obj.(*corev1.Namespace).Name)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			klog.Infof("reciver namespace update event", newObj.(*corev1.Namespace).Name)
		},
		DeleteFunc: func(obj interface{}) {
			klog.Infof("delete namespace delete event", obj.(*corev1.Namespace).Name)
		},
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *AnalysisReconciler) SetupWithManager(mgr ctrl.Manager) error {
	urlruntime.Must(NotNil(r.K8SClient))
	urlruntime.Must(NotNil(r.ScheduleClient))
	urlruntime.Must(NotNil(r.DeploymentInformer))
	urlruntime.Must(NotNil(r.NamespaceInformer))
	urlruntime.Must(NotNil(r.NameSpaceCache))

	klog.Infof("start eatch deployment event")
	r.DeploymentInformer.Informer().AddEventHandler(r.DeploymentEventHandler())
	klog.Infof("start eatch namespace event")
	r.NamespaceInformer.Informer().AddEventHandler(r.NamespaceEventHandler())

	return ctrl.NewControllerManagedBy(mgr).
		For(&schedulev1alpha1.Analysis{}).
		Complete(r)
}

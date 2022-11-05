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
	cranev1alpha1 "github.com/gocrane/api/analysis/v1alpha1"
	cranev1informer "github.com/gocrane/api/pkg/generated/informers/externalversions/analysis/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	urlruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/dynamic/dynamicinformer"
	v1 "k8s.io/client-go/informers/apps/v1"
	corev1informer "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"
	"kubesphere.io/schedule/api/schedule/v1alpha1"
	schedulev1alpha1 "kubesphere.io/schedule/api/schedule/v1alpha1"
	"kubesphere.io/schedule/pkg/client/k8s"
	"kubesphere.io/schedule/pkg/constants"
	"kubesphere.io/schedule/pkg/models/schedule"
	"kubesphere.io/schedule/pkg/utils/jsonpath"
	"kubesphere.io/schedule/pkg/utils/sliceutil"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sync"
)

// AnalysisTaskReconciler reconciles a Analysis object
type AnalysisTaskReconciler struct {
	sync.Mutex
	client.Client
	Scheme *runtime.Scheme

	SchedulerConfig schedule.SchedulerConfig

	K8SClient              k8s.Client
	ScheduleClient         schedule.Operator
	DeploymentInformer     v1.DeploymentInformer
	AnalyticsInformer      cranev1informer.AnalyticsInformer
	RecommendationInformer cranev1informer.RecommendationInformer
	DynamicInformer        dynamicinformer.DynamicSharedInformerFactory
	NamespaceInformer      corev1informer.NamespaceInformer
	NameSpaceCache         map[string]*v1alpha1.AnalysisTask
	DeploymentIndexCache   map[string]*v1alpha1.AnalysisTask
}

//+kubebuilder:rbac:groups=schedule.kubesphere.io,resources=analysistasks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=schedule.kubesphere.io,resources=analysistasks/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=schedule.kubesphere.io,resources=analysistasks/finalizers,verbs=update
//+kubebuilder:rbac:groups=installer.kubesphere.io,resources=clusterconfigurations,verbs= get;list;watch;patch;update

//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch
//+kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch

//+kubebuilder:rbac:groups=analysis.crane.io,resources=analytics;recommendations,verbs= get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Analysis object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.0/pkg/reconcile
func (r *AnalysisTaskReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	klog.Infof("Reconcile AnalysisTask %s", req.NamespacedName)

	instance := &v1alpha1.AnalysisTask{}
	err := r.Client.Get(ctx, req.NamespacedName, instance)
	if apierrors.IsNotFound(err) {
		return ctrl.Result{}, nil
	}
	if err != nil {
		return ctrl.Result{}, err
	}

	isDeletion := !instance.ObjectMeta.DeletionTimestamp.IsZero()
	isConstructed := sliceutil.HasString(instance.ObjectMeta.Finalizers, constants.AnalysisTaskFinalizer)

	if isDeletion {
		if isConstructed {
			if _, err := r.undoReconcile(ctx, instance); err != nil {
				return reconcile.Result{}, err
			}

			instance.ObjectMeta.Finalizers = sliceutil.RemoveString(instance.ObjectMeta.Finalizers, func(item string) bool {
				if item == constants.AnalysisTaskFinalizer {
					return true
				}
				return false
			})
			if err := r.Update(context.Background(), instance); err != nil {
				return reconcile.Result{}, err
			}
			return reconcile.Result{}, nil
		}
	} else {
		if !isConstructed {
			instance.ObjectMeta.Finalizers = append(instance.ObjectMeta.Finalizers, constants.AnalysisTaskFinalizer)
			if err := r.Update(context.Background(), instance); err != nil {
				return reconcile.Result{}, err
			}
			return reconcile.Result{}, nil
		} else {
			if _, err := r.doReconcile(ctx, instance); err != nil {
				return reconcile.Result{}, err
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *AnalysisTaskReconciler) doReconcile(ctx context.Context, instance *schedulev1alpha1.AnalysisTask) (ctrl.Result, error) {
	switch instance.Spec.Type {
	case v1alpha1.ResourceTypeDeployment:
		return r.doReconcileDeploymentAnalysis(ctx, instance)
	case v1alpha1.ResourceTypeNamespace:
		return r.doReconcileNamespaceAnalysis(ctx, instance)
	default:
		klog.Infof("not support resource type %s", instance.Spec.Type)
		return ctrl.Result{}, nil
	}

}

func (r *AnalysisTaskReconciler) undoReconcile(ctx context.Context, instance *schedulev1alpha1.AnalysisTask) (ctrl.Result, error) {
	switch instance.Spec.Type {
	case v1alpha1.ResourceTypeDeployment:
		return r.undoReconcileDeploymentAnalysis(ctx, instance)
		//r.ScheduleClient.CreateCraneAnalysis(analysis.Namespace, analysis.Spec.Target, analysis.Spec.CompletionStrategy)
	case v1alpha1.ResourceTypeNamespace:
		return r.undoReconcileNamespaceAnalysis(ctx, instance)
	default:
		klog.Infof("not support resource type %s", instance.Spec.Type)
		return ctrl.Result{}, nil
	}
}

func (r *AnalysisTaskReconciler) doReconcileDeploymentAnalysis(ctx context.Context, instance *schedulev1alpha1.AnalysisTask) (ctrl.Result, error) {
	_ = log.FromContext(ctx)
	for _, resource := range instance.Spec.ResourceSelectors {
		if resource.Kind != schedulev1alpha1.ResourceTypeDeployment {
			klog.Errorf("unknown kind %s", resource)
			continue
		}
		name, analytics := convertAnalytics("deployment", resource, instance.Spec.CompletionStrategy)
		analytics = labelAnalyticsWithAnalysisName(analytics, instance)
		if err := r.ScheduleClient.CreateCraneAnalysis(ctx, instance.Namespace, name, analytics); err != nil {
			klog.Errorf("creat analysis error %s", err.Error())
			return ctrl.Result{}, err
		}

		// add deployment index
		key := deploymentIndexKey(instance.Namespace, resource.Name)
		r.UpdateDeploymentIndexCache(key, instance)
		if deployment, err := r.K8SClient.Kubernetes().AppsV1().Deployments(instance.Namespace).Get(ctx, resource.Name, metav1.GetOptions{}); err == nil {
			r.UpdateDeploymentStatus(key, deployment)
		}

	}
	return ctrl.Result{}, nil
}

func (r *AnalysisTaskReconciler) undoReconcileDeploymentAnalysis(ctx context.Context, instance *schedulev1alpha1.AnalysisTask) (ctrl.Result, error) {
	//删除 Analysis
	for _, resource := range instance.Spec.ResourceSelectors {
		if resource.Kind != schedulev1alpha1.ResourceTypeDeployment {
			klog.Errorf("unknown kind %s", resource.Name)
			continue
		}

		name, analytics := convertAnalytics("deployment", resource, instance.Spec.CompletionStrategy)
		analytics = labelAnalyticsWithAnalysisName(analytics, instance)
		if err := r.ScheduleClient.DeleteCraneAnalysis(ctx, instance.Namespace, name, analytics); err != nil {
			klog.Errorf("delete analysis error: %s", err.Error())
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *AnalysisTaskReconciler) doReconcileNamespaceAnalysis(ctx context.Context, instance *schedulev1alpha1.AnalysisTask) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	for _, resource := range instance.Spec.ResourceSelectors {
		if resource.Kind != schedulev1alpha1.ResourceTypeNamespace {
			klog.Errorf("unknown kind %v", resource)
			continue
		}

		namespace := resource.Name
		if namespace == "" {
			return ctrl.Result{}, nil
		}
		r.NameSpaceCache[namespace] = instance

		deployments, err := r.K8SClient.Kubernetes().AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			klog.Errorf("get deployment error: %s", err.Error())
			return ctrl.Result{}, err
		}
		for _, deployment := range deployments.Items {
			resource := convertResource(&deployment)
			name, analytics := convertAnalytics("namespace", resource, instance.Spec.CompletionStrategy)
			analytics = labelAnalyticsWithAnalysisName(analytics, instance)
			err = r.ScheduleClient.CreateCraneAnalysis(ctx, namespace, name, analytics)
			if err != nil {
				klog.Errorf("creat analysis error: %s", err.Error())
				return ctrl.Result{}, err
			}

			// add deployment index
			key := deploymentIndexKey(instance.Namespace, resource.Name)
			r.UpdateDeploymentIndexCache(key, instance)
			r.UpdateDeploymentStatus(key, &deployment)
		}
	}
	return ctrl.Result{}, nil
}

func (r *AnalysisTaskReconciler) undoReconcileNamespaceAnalysis(ctx context.Context, instance *schedulev1alpha1.AnalysisTask) (ctrl.Result, error) {
	//删除 Analysis

	for _, resource := range instance.Spec.ResourceSelectors {
		if resource.Kind != schedulev1alpha1.ResourceTypeNamespace ||
			resource.Name == "" {
			klog.Errorf("unknown kind %v", resource)
			continue
		}
		namespace := resource.Name
		r.NameSpaceCache[namespace] = instance

		deployments, err := r.K8SClient.Kubernetes().AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			klog.Errorf("get deployment error: %s", err.Error())
			return ctrl.Result{}, err
		}
		for _, deployment := range deployments.Items {
			resource := convertResource(&deployment)
			name, analytics := convertAnalytics("namespace", resource, instance.Spec.CompletionStrategy)
			analytics = labelAnalyticsWithAnalysisName(analytics, instance)
			err = r.ScheduleClient.DeleteCraneAnalysis(ctx, namespace, name, analytics)
			if err != nil {
				klog.Errorf("creat analysis error: %s", err.Error())
				return ctrl.Result{}, err
			}
		}
	}
	return ctrl.Result{}, nil
}

func (r *AnalysisTaskReconciler) DeploymentEventHandler() cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			klog.V(4).Infof("reciver deployment add event %s", obj.(*appsv1.Deployment).Name)
			deployment := obj.(*appsv1.Deployment)
			namespace := deployment.Namespace
			if analysis, ok := r.NameSpaceCache[namespace]; ok {
				klog.V(4).Infof("create deployment analysis %s", obj.(*appsv1.Deployment).Name)
				name, analytics := convertAnalytics("namespace", convertResource(deployment), analysis.Spec.CompletionStrategy)
				r.ScheduleClient.CreateCraneAnalysis(context.Background(),
					deployment.Namespace,
					name,
					analytics)
			} else {
				klog.V(4).Infof("skip create deployment analysis %s", obj.(*appsv1.Deployment).Name)
			}

			key := deploymentIndexKey(deployment.Namespace, deployment.Name)
			r.UpdateDeploymentStatus(key, deployment)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			deployment := newObj.(*appsv1.Deployment)
			klog.V(4).Infof("reciver deployment update event %s", newObj.(*appsv1.Deployment).Name)

			key := deploymentIndexKey(deployment.Namespace, deployment.Name)
			r.UpdateDeploymentStatus(key, deployment)
		},
		DeleteFunc: func(obj interface{}) {
			klog.V(4).Infof("delete deployment delete event %s", obj.(*appsv1.Deployment).Name)
		},
	}
}

func (r *AnalysisTaskReconciler) UpdateDeploymentIndexCache(key string, instance *schedulev1alpha1.AnalysisTask) {
	r.DeploymentIndexCache[key] = instance
}

func (r *AnalysisTaskReconciler) UpdateDeploymentStatus(key string, deployment *appsv1.Deployment) {
	if instance, ok := r.DeploymentIndexCache[key]; ok {
		instance, err := r.K8SClient.Schedule().ScheduleV1alpha1().AnalysisTasks(instance.Namespace).Get(context.Background(), instance.Name, metav1.GetOptions{})
		if err != nil {
			klog.Errorf("get analysis task error: %s", err.Error())
			return
		}
		if instance.Status.TargetDeployments == nil {
			instance.Status.TargetDeployments = make(map[string]*appsv1.Deployment)
		}
		instance.Status.TargetDeployments[deployment.Name] = deployment
		instance, err = r.K8SClient.Schedule().ScheduleV1alpha1().AnalysisTasks(instance.Namespace).UpdateStatus(context.Background(), instance, metav1.UpdateOptions{})
		if err != nil {
			klog.Errorf("update deployment status error: %s", err.Error())
		}
		r.DeploymentIndexCache[key] = instance
	}
}

func (r *AnalysisTaskReconciler) NamespaceEventHandler() cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			klog.V(4).Infof("reciver namespace add event %s", obj.(*corev1.Namespace).Name)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			klog.V(4).Infof("reciver namespace update event %s", newObj.(*corev1.Namespace).Name)
		},
		DeleteFunc: func(obj interface{}) {
			klog.V(4).Infof("delete namespace delete event %s", obj.(*corev1.Namespace).Name)
		},
	}
}

func (r *AnalysisTaskReconciler) ConfigEventHandler() cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			r.UpdateScheduleConfig(obj)
			klog.V(4).Infof("reciver Installer add event %v", r.SchedulerConfig)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			r.UpdateScheduleConfig(newObj)
			klog.V(4).Infof("reciver Installer update event %v", r.SchedulerConfig)
		},
		DeleteFunc: func(obj interface{}) {
			klog.V(4).Infof("delete Installer delete event %v", obj)
		},
	}
}

func (r *AnalysisTaskReconciler) AnalyticsEventHandler() cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			o := obj.(*cranev1alpha1.Analytics)
			klog.V(4).Infof("reciver Analytics add event %v", o)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			//o := newObj.(*cranev1alpha1.Analytics)
			//klog.Infof("reciver Analytics update event %v:%v", o.ResourceVersion, o.Status.LastUpdateTime)
			//for _, comm := range o.Status.Recommendations {
			//	klog.V(4).Infof("reciver Analytics update event %s: %s", comm.Name, comm.Message)
			//}
		},
		DeleteFunc: func(obj interface{}) {
			o := obj.(*cranev1alpha1.Analytics)
			klog.V(4).Infof("reciver Analytics delete event %v", o)
		},
	}
}
func (r *AnalysisTaskReconciler) RecommendationsEventHandler() cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			o := obj.(*cranev1alpha1.Recommendation)
			klog.Infof("reciver Installer add event %v", o)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			o := newObj.(*cranev1alpha1.Recommendation)
			klog.Infof("reciver Installer update event %v", o)
		},
		DeleteFunc: func(obj interface{}) {
			o := obj.(*cranev1alpha1.Recommendation)
			klog.Infof("reciver Installer delete event %v", o)
		},
	}
}

func (r *AnalysisTaskReconciler) UpdateScheduleConfig(newObj interface{}) {
	object := jsonpath.New(newObj)
	cpu, err := object.GetInt64("spec.schedule.analysis.notifyThreshold.cpu")
	if err == nil {
		r.SchedulerConfig.CPUNotifyPresent = &cpu
	}
	mem, err := object.GetInt64("spec.schedule.analysis.notifyThreshold.mem")
	if err == nil {
		r.SchedulerConfig.MemNotifyPresent = &mem
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *AnalysisTaskReconciler) SetupWithManager(mgr ctrl.Manager) error {
	urlruntime.Must(NotNil(r.K8SClient))
	urlruntime.Must(NotNil(r.ScheduleClient))
	urlruntime.Must(NotNil(r.DeploymentInformer))
	urlruntime.Must(NotNil(r.NamespaceInformer))
	urlruntime.Must(NotNil(r.DynamicInformer))
	urlruntime.Must(NotNil(r.SchedulerConfig))
	urlruntime.Must(NotNil(r.AnalyticsInformer))
	urlruntime.Must(NotNil(r.RecommendationInformer))
	urlruntime.Must(NotNil(r.NameSpaceCache))
	urlruntime.Must(NotNil(r.DeploymentIndexCache))

	r.DeploymentInformer.Informer().AddEventHandler(r.DeploymentEventHandler())
	r.NamespaceInformer.Informer().AddEventHandler(r.NamespaceEventHandler())
	r.AnalyticsInformer.Informer().AddEventHandler(r.AnalyticsEventHandler())
	r.RecommendationInformer.Informer().AddEventHandler(r.RecommendationsEventHandler())
	r.DynamicInformer.ForResource(schema.GroupVersionResource{
		Group: "installer.kubesphere.io", Version: "v1alpha1", Resource: "clusterconfigurations",
	}).Informer().AddEventHandler(r.ConfigEventHandler())

	return ctrl.NewControllerManagedBy(mgr).
		For(&schedulev1alpha1.AnalysisTask{}).
		Complete(r)
}

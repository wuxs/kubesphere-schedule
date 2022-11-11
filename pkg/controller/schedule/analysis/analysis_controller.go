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
	"fmt"
	"strings"
	"sync"

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
	"k8s.io/client-go/tools/record"
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

// AnalysisTaskReconciler reconciles a Analysis object
type AnalysisTaskReconciler struct {
	sync.Mutex
	client.Client
	Scheme *runtime.Scheme

	ClusterScheduleConfig *schedulev1alpha1.ClusterScheduleConfig

	Recorder                   record.EventRecorder
	K8SClient                  k8s.Client
	ScheduleClient             schedule.Operator
	AnalyticsInformer          cranev1informer.AnalyticsInformer
	RecommendationInformer     cranev1informer.RecommendationInformer
	DynamicInformer            dynamicinformer.DynamicSharedInformerFactory
	NamespaceInformer          corev1informer.NamespaceInformer
	DeploymentsInformer        v1.DeploymentInformer
	DaemonSetsInformer         v1.DaemonSetInformer
	StatefulSetsInformer       v1.StatefulSetInformer
	NameSpaceAnalysisTaskCache map[string]*v1alpha1.AnalysisTask // cache namespace analysis task for new workload
	AnalysisTaskReverseIndex   map[string]*v1alpha1.AnalysisTask // map[<namespace>]map[<taskName>]*v1alpha1.AnalysisTask
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

	if isDeletion { // delete
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
	} else { // create or update
		if !isConstructed {
			instance.ObjectMeta.Finalizers = append(instance.ObjectMeta.Finalizers, constants.AnalysisTaskFinalizer)
			instance.Status.Status = schedulev1alpha1.UpdatingStatus
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

func (r *AnalysisTaskReconciler) doReconcile(ctx context.Context, instance *schedulev1alpha1.AnalysisTask) (ret ctrl.Result, err error) {
	switch instance.Spec.Type {
	case v1alpha1.WorkloadResourceType:
		ret, err = r.doReconcileWorkloadAnalysis(ctx, instance)
	case v1alpha1.NamespaceResourceType:
		ret, err = r.doReconcileNamespaceAnalysis(ctx, instance)
	default:
		klog.Infof("not support resource type %s, should be Namespace/Workload", instance.Spec.Type)
		return ctrl.Result{}, nil
	}
	if err != nil {
		r.UpdateAnalysisTaskStatus(ctx, instance, schedulev1alpha1.ErrorStatus)
	}
	return
}

func (r *AnalysisTaskReconciler) undoReconcile(ctx context.Context, instance *schedulev1alpha1.AnalysisTask) (ctrl.Result, error) {
	switch instance.Spec.Type {
	case v1alpha1.NamespaceResourceType:
		return r.undoReconcileNamespaceAnalysis(ctx, instance)
	case v1alpha1.WorkloadResourceType:
		return r.undoReconcileWorkloadAnalysis(ctx, instance)
	default:
		klog.Infof("not support resource type %s", instance.Spec.Type)
		return ctrl.Result{}, nil
	}
}

func (r *AnalysisTaskReconciler) doReconcileWorkloadAnalysis(ctx context.Context, instance *schedulev1alpha1.AnalysisTask) (ctrl.Result, error) {
	_ = log.FromContext(ctx)
	namespace := instance.Namespace
	workloads := strings.Split(r.GetNamespacesWorkloads(ctx, namespace), ",")
	for _, resource := range instance.Spec.ResourceSelectors {
		workload := corev1.ObjectReference{
			Kind:       resource.Kind,
			Name:       resource.Name,
			APIVersion: resource.APIVersion,
			Namespace:  namespace,
		}
		err := r.CreateCraneAnalysis(ctx, workload, instance)
		if err != nil {
			err := fmt.Errorf("create crane analysis failed, err: %w", err)
			r.Recorder.Event(instance, corev1.EventTypeWarning, "CreateAnalysis", err.Error())
			klog.Error(err)
			continue
		}
		key := genWorkloadName(workload.Kind, workload.Name)
		if !sliceutil.HasString(workloads, key) {
			workloads = append(workloads, key)
		}
	}
	r.UpdateIndexCache(instance)

	workloads = sliceutil.RemoveString(workloads, func(item string) bool { return item == "" })
	if err := r.UpdateNamespacesWorkloads(ctx, namespace, strings.Join(workloads, ",")); err != nil {
		klog.Errorf("get deployment error: %s", err.Error())
		return ctrl.Result{}, err
	}

	var err error
	if instance.Spec.CompletionStrategy.CompletionStrategyType == v1alpha1.CompletionStrategyOnce {
		err = r.UpdateAnalysisTaskStatus(ctx, instance, schedulev1alpha1.CompletedStatus)
	}
	if instance.Spec.Type == v1alpha1.NamespaceResourceType {
		err = r.UpdateAnalysisTaskStatus(ctx, instance, schedulev1alpha1.RunningStatus)
	}
	if err != nil {
		klog.Errorf("update analysis task status error: %s", err.Error())
	}

	return ctrl.Result{}, nil
}

func (r *AnalysisTaskReconciler) undoReconcileWorkloadAnalysis(ctx context.Context, instance *schedulev1alpha1.AnalysisTask) (ctrl.Result, error) {
	//删除 Analysis
	for _, resource := range instance.Spec.ResourceSelectors {
		if resource.Kind == schedulev1alpha1.NamespaceResourceType {
			klog.Errorf("unknown kind %s/%s", resource.Kind, resource.Name)
			continue
		}

		if err := r.DeleteCraneAnalysis(ctx, corev1.ObjectReference{
			Kind:       resource.Kind,
			Name:       resource.Name,
			APIVersion: resource.APIVersion,
			Namespace:  instance.Namespace,
		}, instance); err != nil {
			klog.Errorf("delete analysis error: %s", err.Error())
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *AnalysisTaskReconciler) doReconcileNamespaceAnalysis(ctx context.Context, instance *schedulev1alpha1.AnalysisTask) (ctrl.Result, error) {
	_ = log.FromContext(ctx)
	if instance.Spec.Type != schedulev1alpha1.NamespaceResourceType {
		err := fmt.Errorf("not support resource type %s", instance.Spec.Type)
		return ctrl.Result{}, err
	}
	for _, resource := range instance.Spec.ResourceSelectors {
		if resource.Kind != schedulev1alpha1.NamespaceResourceType {
			klog.Errorf("unknown kind %v", resource)
			continue
		}

		namespace := resource.Name
		if namespace == "" {
			klog.Errorf("namespace is empty, %v", instance)
			continue
		}
		r.NameSpaceAnalysisTaskCache[namespace] = instance

		result, err := r.reconcileNamespaceAnalysis(ctx, namespace, instance)
		if err != nil {
			return result, err
		}

		r.UpdateIndexCache(instance)

		// overwrite Namespaces Annotations
		err = r.UpdateNamespacesWorkloads(ctx, namespace, "")
		if err != nil {
			klog.Errorf("get deployment error: %s", err.Error())
			return ctrl.Result{}, err
		}
	}

	var err error
	if instance.Spec.CompletionStrategy.CompletionStrategyType == v1alpha1.CompletionStrategyOnce {
		err = r.UpdateAnalysisTaskStatus(ctx, instance, schedulev1alpha1.CompletedStatus)
	}
	if instance.Spec.Type == v1alpha1.NamespaceResourceType {
		err = r.UpdateAnalysisTaskStatus(ctx, instance, schedulev1alpha1.RunningStatus)
	}
	if err != nil {
		klog.Errorf("update analysis task status error: %s", err.Error())
	}
	return ctrl.Result{}, nil
}

func (r *AnalysisTaskReconciler) reconcileNamespaceAnalysis(ctx context.Context, namespace string, instance *schedulev1alpha1.AnalysisTask) (ctrl.Result, error) {
	deployments, err := r.K8SClient.Kubernetes().AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		klog.Errorf("get deployment error: %s", err.Error())
		return ctrl.Result{}, err
	}
	for _, resource := range deployments.Items {
		workload := getObjectReference(resource)
		workload.Namespace = namespace
		err := r.CreateCraneAnalysis(ctx, workload, instance)
		if err != nil {
			klog.Errorf("create crane analysis failed, err: %v", err)
			return ctrl.Result{}, err
		}
	}

	statefulSets, err := r.K8SClient.Kubernetes().AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		klog.Errorf("get deployment error: %s", err.Error())
		return ctrl.Result{}, err
	}
	for _, resource := range statefulSets.Items {
		workload := getObjectReference(resource)
		workload.Namespace = namespace
		err := r.CreateCraneAnalysis(ctx, workload, instance)
		if err != nil {
			klog.Errorf("create crane analysis failed, err: %v", err)
			return ctrl.Result{}, err
		}
	}

	daemonSets, err := r.K8SClient.Kubernetes().AppsV1().DaemonSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		klog.Errorf("get deployment error: %s", err.Error())
		return ctrl.Result{}, err
	}
	for _, resource := range daemonSets.Items {
		workload := getObjectReference(resource)
		workload.Namespace = namespace
		err := r.CreateCraneAnalysis(ctx, workload, instance)
		if err != nil {
			klog.Errorf("create crane analysis failed, err: %v", err)
			return ctrl.Result{}, err
		}
	}

	r.UpdateNamespaceAnalysisTaskLabel(ctx, namespace, instance)
	return ctrl.Result{}, nil
}

func (r *AnalysisTaskReconciler) undoReconcileNamespaceAnalysis(ctx context.Context, instance *schedulev1alpha1.AnalysisTask) (ctrl.Result, error) {
	for _, resource := range instance.Spec.ResourceSelectors {
		if resource.Kind != schedulev1alpha1.NamespaceResourceType ||
			resource.Name == "" {
			klog.Errorf("unknown kind %v", resource)
			continue
		}
		namespace := resource.Name
		r.NameSpaceAnalysisTaskCache[namespace] = instance

		deployments, err := r.K8SClient.Kubernetes().AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			klog.Errorf("get deployment error: %s", err.Error())
			return ctrl.Result{}, err
		}
		for _, resource := range deployments.Items {
			if Label(resource.Labels, constants.AnalysisTaskLabelKey) == instance.Name {
				if err := r.DeleteCraneAnalysis(ctx, corev1.ObjectReference{
					Kind:       resource.Kind,
					Name:       resource.Name,
					APIVersion: resource.APIVersion,
					Namespace:  namespace,
				}, instance); err != nil {
					klog.Errorf("delete analysis error: %s", err.Error())
					return ctrl.Result{}, err
				}
			}
		}

		statefulSets, err := r.K8SClient.Kubernetes().AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			klog.Errorf("get deployment error: %s", err.Error())
			return ctrl.Result{}, err
		}
		for _, resource := range statefulSets.Items {
			if Label(resource.Labels, constants.AnalysisTaskLabelKey) == instance.Name {
				if err := r.DeleteCraneAnalysis(ctx, corev1.ObjectReference{
					Kind:       resource.Kind,
					Name:       resource.Name,
					APIVersion: resource.APIVersion,
					Namespace:  namespace,
				}, instance); err != nil {
					klog.Errorf("delete analysis error: %s", err.Error())
					return ctrl.Result{}, err
				}
			}
		}

		daemonSets, err := r.K8SClient.Kubernetes().AppsV1().DaemonSets(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			klog.Errorf("get deployment error: %s", err.Error())
			return ctrl.Result{}, err
		}
		for _, resource := range daemonSets.Items {
			if Label(resource.Labels, constants.AnalysisTaskLabelKey) == instance.Name {
				if err := r.DeleteCraneAnalysis(ctx, corev1.ObjectReference{
					Kind:       resource.Kind,
					Name:       resource.Name,
					APIVersion: resource.APIVersion,
					Namespace:  namespace,
				}, instance); err != nil {
					klog.Errorf("delete analysis error: %s", err.Error())
					return ctrl.Result{}, err
				}
			}
		}

		r.CleanNamespaceAnalysisTaskLabel(ctx, namespace)
	}
	return ctrl.Result{}, nil
}

func (r *AnalysisTaskReconciler) handleWorkloadUpdateEvent(workload interface{}) {
	switch workload := workload.(type) {
	case *appsv1.Deployment:
		namespace := workload.Namespace
		if analysisTask, ok := r.NameSpaceAnalysisTaskCache[namespace]; ok {
			oldTask := r.GetAnalysisTask(namespace, Label(workload.Labels, constants.AnalysisTaskLabelKey))
			if oldTask != nil {
				return
			}
			objectRef := getObjectReference(workload)
			objectRef.Namespace = namespace
			err := r.CreateCraneAnalysis(context.Background(), objectRef, analysisTask)
			if err != nil {
				klog.Errorf("create crane analysis failed, err: %v", err)
				return
			}
		} else {
			klog.V(4).Infof("skip create deployment analysis %s", workload.Name)
		}
	case *appsv1.StatefulSet:
		namespace := workload.Namespace
		if analysisTask, ok := r.NameSpaceAnalysisTaskCache[namespace]; ok {
			oldTask := r.GetAnalysisTask(namespace, Label(workload.Labels, constants.AnalysisTaskLabelKey))
			if oldTask != nil {
				return
			}
			objectRef := getObjectReference(workload)
			objectRef.Namespace = namespace
			err := r.CreateCraneAnalysis(context.Background(), objectRef, analysisTask)
			if err != nil {
				klog.Errorf("create crane analysis failed, err: %v", err)
				return
			}
		} else {
			klog.V(4).Infof("skip create deployment analysis %s", workload.Name)
		}
	case *appsv1.DaemonSet:
		namespace := workload.Namespace
		if analysisTask, ok := r.NameSpaceAnalysisTaskCache[namespace]; ok {
			oldTask := r.GetAnalysisTask(namespace, Label(workload.Labels, constants.AnalysisTaskLabelKey))
			if oldTask != nil {
				return
			}
			objectRef := getObjectReference(workload)
			objectRef.Namespace = namespace
			err := r.CreateCraneAnalysis(context.Background(), objectRef, analysisTask)
			if err != nil {
				klog.Errorf("create crane analysis failed, err: %v", err)
				return
			}
		} else {
			klog.V(4).Infof("skip create deployment analysis %s", workload.Name)
		}
	}
}

func (r *AnalysisTaskReconciler) handleDeploymentRemoveEvent(deployment *appsv1.Deployment) {
	klog.Infof("handle deployment remove event %s, do nothing", deployment.Name)
}

func (r *AnalysisTaskReconciler) WorkloadEventHandler() cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			r.handleWorkloadUpdateEvent(obj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			r.handleWorkloadUpdateEvent(newObj)
		},
		DeleteFunc: func(obj interface{}) {
			deployment := obj.(*appsv1.Deployment)
			klog.V(4).Infof("delete deployment delete event %s", deployment.Name)
		},
	}
}

func (r *AnalysisTaskReconciler) UpdateIndexCache(instance *schedulev1alpha1.AnalysisTask) {
	//r.IndexCache[key] = instance
	key := genAnalysisTaskIndexKey(instance.Namespace, instance.Name)
	if o, ok := r.AnalysisTaskReverseIndex[key]; ok {
		klog.V(4).Infof("analysis task %s %s will overwrite", key, o.CreationTimestamp)
	}
	r.AnalysisTaskReverseIndex[key] = instance
}

func (r *AnalysisTaskReconciler) RemoveIndexCache(namespace, name string) {
	key := genAnalysisTaskIndexKey(namespace, name)
	delete(r.AnalysisTaskReverseIndex, key)
}

func (r *AnalysisTaskReconciler) GetAnalysisTask(namespace string, taskID string) *schedulev1alpha1.AnalysisTask {
	if task, ok := r.AnalysisTaskReverseIndex[genAnalysisTaskIndexKey(namespace, taskID)]; ok {
		return task
	}
	return nil
}

func (r *AnalysisTaskReconciler) ConfigEventHandler() cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			r.UpdateScheduleConfig(obj)
			klog.V(4).Infof("receiver Installer add event %v", r.ClusterScheduleConfig)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			r.UpdateScheduleConfig(newObj)
			klog.V(4).Infof("receiver Installer update event %v", r.ClusterScheduleConfig)
		},
		DeleteFunc: func(obj interface{}) {
			//r.UpdateScheduleConfig(obj)
			klog.V(4).Infof("delete Installer delete event %v", obj, r.ClusterScheduleConfig)
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
			//o.Labels @TODO
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
	var config *schedulev1alpha1.ClusterScheduleConfig
	object := jsonpath.New(newObj)
	err := object.DataAs("spec.scheduler", &config)
	if err != nil {
		klog.Errorf("parse ClusterConfig is error", object)
		return
	}
	r.ClusterScheduleConfig = config
	klog.V(4).Infof("update cluster schedule config: %v", config)
}

// SetupWithManager sets up the controller with the Manager.
func (r *AnalysisTaskReconciler) SetupWithManager(mgr ctrl.Manager) error {
	urlruntime.Must(NotNil(r.K8SClient))
	urlruntime.Must(NotNil(r.ScheduleClient))
	urlruntime.Must(NotNil(r.DeploymentsInformer))
	urlruntime.Must(NotNil(r.DaemonSetsInformer))
	urlruntime.Must(NotNil(r.StatefulSetsInformer))
	urlruntime.Must(NotNil(r.NamespaceInformer))
	urlruntime.Must(NotNil(r.DynamicInformer))
	urlruntime.Must(NotNil(r.ClusterScheduleConfig))
	urlruntime.Must(NotNil(r.AnalyticsInformer))
	urlruntime.Must(NotNil(r.RecommendationInformer))
	urlruntime.Must(NotNil(r.NameSpaceAnalysisTaskCache))
	urlruntime.Must(NotNil(r.AnalysisTaskReverseIndex))

	r.DeploymentsInformer.Informer().AddEventHandler(r.WorkloadEventHandler())
	r.DaemonSetsInformer.Informer().AddEventHandler(r.WorkloadEventHandler())
	r.StatefulSetsInformer.Informer().AddEventHandler(r.WorkloadEventHandler())
	r.AnalyticsInformer.Informer().AddEventHandler(r.AnalyticsEventHandler())
	r.RecommendationInformer.Informer().AddEventHandler(r.RecommendationsEventHandler())
	r.DynamicInformer.ForResource(schema.GroupVersionResource{
		Group: "installer.kubesphere.io", Version: "v1alpha1", Resource: "clusterconfigurations",
	}).Informer().AddEventHandler(r.ConfigEventHandler())

	return ctrl.NewControllerManagedBy(mgr).
		For(&schedulev1alpha1.AnalysisTask{}).
		Complete(r)
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

func (r *AnalysisTaskReconciler) Validating(ctx context.Context, request admission.Request) admission.Response {
	return admission.Denied("@TODO Validating")
}

func genAnalysis(resource corev1.ObjectReference, instance *schedulev1alpha1.AnalysisTask) *cranev1alpha1.Analytics {
	name := genAnalysisName(instance.Spec.Type, resource.Kind, resource.Name)
	analytics := convertAnalytics(name, resource, instance.Spec.CompletionStrategy)
	analytics = labelAnalyticsWithAnalysisName(analytics, instance.Name)
	return analytics
}

func (r *AnalysisTaskReconciler) DeleteCraneAnalysis(ctx context.Context, resource corev1.ObjectReference, instance *v1alpha1.AnalysisTask) (err error) {
	name := genAnalysisName(instance.Spec.Type, resource.Kind, resource.Name)
	err = r.ScheduleClient.DeleteCraneAnalysis(ctx, resource.Namespace, name)
	if err != nil {
		return err
	}
	r.Recorder.Event(instance, corev1.EventTypeNormal, "DeleteCraneAnalysis", fmt.Sprintf("Delete CraneAnalysis %s/%s", resource.Namespace, name))
	return
}

func (r *AnalysisTaskReconciler) CreateCraneAnalysis(ctx context.Context, resource corev1.ObjectReference, instance *v1alpha1.AnalysisTask) (err error) {
	switch resource.Kind {
	case v1alpha1.DeploymentResource:
		err = r.CreateDeploymentCraneAnalysis(ctx, resource, instance)
	case v1alpha1.StatefulSetResource:
		err = r.CreateStatefulSetCraneAnalysis(ctx, resource, instance)
	case v1alpha1.DaemonSetResource:
		err = r.CreateDaemonSetCraneAnalysis(ctx, resource, instance)
	default:
		err = fmt.Errorf("not support resource %s kind %s", resource.Name, resource.Kind)
		r.Recorder.Event(instance, corev1.EventTypeWarning, "CreateCraneAnalysis", err.Error())
		return nil
	}
	r.Recorder.Event(instance, corev1.EventTypeNormal, "CreateCraneAnalysis", fmt.Sprintf("Create CraneAnalysis %s/%s", resource.Namespace, resource.Name))
	return
}

func (r *AnalysisTaskReconciler) CreateDeploymentCraneAnalysis(ctx context.Context, resource corev1.ObjectReference, instance *v1alpha1.AnalysisTask) error {
	workload, err := r.K8SClient.Kubernetes().AppsV1().Deployments(resource.Namespace).Get(ctx, resource.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if oldTask := Label(workload.Labels, constants.AnalysisTaskLabelKey); oldTask != "" {
		if err := r.ScheduleClient.DeleteCraneAnalysis(ctx, workload.Namespace, oldTask); err != nil {
			klog.Errorf("remove old analysis failed: %s", err.Error())
		}
	}
	workloadCopy := workload.DeepCopy()
	if workloadCopy.Labels == nil {
		workloadCopy.Labels = make(map[string]string, 0)
	}
	workloadCopy.Labels[constants.AnalysisTaskLabelKey] = instance.Name
	patch := client.MergeFrom(workload)
	data, err := patch.Data(workloadCopy)
	if err != nil {
		klog.Error("create patch failed", err)
		return err
	}

	// data == "{}", need not to patch
	if len(data) == 2 {
		return nil
	}

	_, err = r.K8SClient.Kubernetes().AppsV1().Deployments(workload.Namespace).Patch(ctx, workload.Name, patch.Type(), data, metav1.PatchOptions{})
	if err != nil {
		klog.Error(err)
		return err
	}
	analytics := genAnalysis(resource, instance)
	if err := r.ScheduleClient.CreateCraneAnalysis(ctx, instance.Namespace, analytics); err != nil {
		klog.Errorf("creat analysis error %s", err.Error())
		return err
	}
	return nil
}

func (r *AnalysisTaskReconciler) CreateStatefulSetCraneAnalysis(ctx context.Context, resource corev1.ObjectReference, instance *v1alpha1.AnalysisTask) error {
	workload, err := r.K8SClient.Kubernetes().AppsV1().StatefulSets(resource.Namespace).Get(ctx, resource.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if oldTask := Label(workload.Labels, constants.AnalysisTaskLabelKey); oldTask != "" {
		if err := r.ScheduleClient.DeleteCraneAnalysis(ctx, workload.Namespace, oldTask); err != nil {
			klog.Errorf("remove old analysis failed: %s", err.Error())
		}
	}

	workloadCopy := workload.DeepCopy()
	if workloadCopy.Labels == nil {
		workloadCopy.Labels = make(map[string]string, 0)
	}
	workloadCopy.Labels[constants.AnalysisTaskLabelKey] = instance.Name
	patch := client.MergeFrom(workload)
	data, err := patch.Data(workloadCopy)
	if err != nil {
		klog.Error("create patch failed", err)
		return err
	}

	// data == "{}", need not to patch
	if len(data) == 2 {
		return nil
	}

	_, err = r.K8SClient.Kubernetes().AppsV1().StatefulSets(workload.Namespace).Patch(ctx, workload.Name, patch.Type(), data, metav1.PatchOptions{})
	if err != nil {
		klog.Error(err)
		return err
	}
	analytics := genAnalysis(resource, instance)
	if err := r.ScheduleClient.CreateCraneAnalysis(ctx, instance.Namespace, analytics); err != nil {
		klog.Errorf("creat analysis error %s", err.Error())
		return err
	}
	return nil
}

func (r *AnalysisTaskReconciler) CreateDaemonSetCraneAnalysis(ctx context.Context, resource corev1.ObjectReference, instance *v1alpha1.AnalysisTask) error {
	workload, err := r.K8SClient.Kubernetes().AppsV1().DaemonSets(resource.Namespace).Get(ctx, resource.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if oldTask := Label(workload.Labels, constants.AnalysisTaskLabelKey); oldTask != "" {
		if err := r.ScheduleClient.DeleteCraneAnalysis(ctx, workload.Namespace, oldTask); err != nil {
			klog.Errorf("remove old analysis failed: %s", err.Error())
		}
	}

	workloadCopy := workload.DeepCopy()
	if workloadCopy.Labels == nil {
		workloadCopy.Labels = make(map[string]string, 0)
	}
	workloadCopy.Labels[constants.AnalysisTaskLabelKey] = instance.Name
	patch := client.MergeFrom(workload)
	data, err := patch.Data(workloadCopy)
	if err != nil {
		klog.Error("create patch failed", err)
		return err
	}

	// data == "{}", need not to patch
	if len(data) == 2 {
		return nil
	}

	_, err = r.K8SClient.Kubernetes().AppsV1().DaemonSets(workload.Namespace).Patch(ctx, workload.Name, patch.Type(), data, metav1.PatchOptions{})
	if err != nil {
		klog.Error(err)
		return err
	}
	analytics := genAnalysis(resource, instance)
	if err := r.ScheduleClient.CreateCraneAnalysis(ctx, instance.Namespace, analytics); err != nil {
		klog.Errorf("creat analysis error %s", err.Error())
		return err
	}
	return nil
}

func (r *AnalysisTaskReconciler) UpdateNamespaceAnalysisTaskLabel(ctx context.Context, namespace string, instance *v1alpha1.AnalysisTask) error {
	ns, err := r.K8SClient.Kubernetes().CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if err != nil {
		return err
	}

	nsCopy := ns.DeepCopy()
	if nsCopy.Labels == nil {
		nsCopy.Labels = make(map[string]string, 0)
	}
	nsCopy.Labels[constants.AnalysisTaskLabelKey] = instance.Name
	patch := client.MergeFrom(ns)
	data, err := patch.Data(nsCopy)
	if err != nil {
		klog.Error("create patch failed", err)
		return err
	}

	// data == "{}", need not to patch
	if len(data) == 2 {
		return nil
	}

	_, err = r.K8SClient.Kubernetes().CoreV1().Namespaces().Patch(ctx, ns.Name, patch.Type(), data, metav1.PatchOptions{})
	if err != nil {
		klog.Error(err)
		return err
	}
	return nil
}

func (r *AnalysisTaskReconciler) CleanNamespaceAnalysisTaskLabel(ctx context.Context, namespace string) error {
	ns, err := r.K8SClient.Kubernetes().CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if err != nil {
		return err
	}

	nsCopy := ns.DeepCopy()
	if nsCopy.Labels == nil {
		nsCopy.Labels = make(map[string]string, 0)
	}
	delete(nsCopy.Labels, constants.AnalysisTaskLabelKey)
	patch := client.MergeFrom(ns)
	data, err := patch.Data(nsCopy)
	if err != nil {
		klog.Error("create patch failed", err)
		return err
	}

	// data == "{}", need not to patch
	if len(data) == 2 {
		return nil
	}

	_, err = r.K8SClient.Kubernetes().CoreV1().Namespaces().Patch(ctx, ns.Name, patch.Type(), data, metav1.PatchOptions{})
	if err != nil {
		klog.Error(err)
		return err
	}
	return nil
}

func (r *AnalysisTaskReconciler) GetNamespacesWorkloads(ctx context.Context, namespace string) string {
	ns, err := r.K8SClient.Kubernetes().CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if err != nil {
		return ""
	}

	value := Label(ns.Annotations, constants.AnalysisTaskWorkloadAnnotationKey)
	return value
}

func (r *AnalysisTaskReconciler) UpdateNamespacesWorkloads(ctx context.Context, namespace string, value string) error {
	ns, err := r.K8SClient.Kubernetes().CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if err != nil {
		return err
	}

	oldValue := Label(ns.Annotations, constants.AnalysisTaskWorkloadAnnotationKey)
	klog.V(4).Infof("overwrite namespace %s annotation %s,old =%s,new = %s", namespace, constants.AnalysisTaskWorkloadAnnotationKey, oldValue, value)
	nsCopy := ns.DeepCopy()
	if nsCopy.Annotations == nil {
		nsCopy.Annotations = make(map[string]string, 0)
	}
	if value != "" {
		nsCopy.Annotations[constants.AnalysisTaskWorkloadAnnotationKey] = value
	} else {
		delete(nsCopy.Annotations, constants.AnalysisTaskWorkloadAnnotationKey)
	}
	patch := client.MergeFrom(ns)
	data, err := patch.Data(nsCopy)
	if err != nil {
		klog.Error("create patch failed", err)
		return err
	}

	// data == "{}", need not to patch
	if len(data) == 2 {
		return nil
	}

	_, err = r.K8SClient.Kubernetes().CoreV1().Namespaces().Patch(ctx, namespace, patch.Type(), data, metav1.PatchOptions{})
	if err != nil {
		klog.Error(err)
		return err
	}
	return nil
}

func (r *AnalysisTaskReconciler) UpdateAnalysisTaskStatus(ctx context.Context, instance *schedulev1alpha1.AnalysisTask, status schedulev1alpha1.Status) error {
	task, err := r.K8SClient.Schedule().ScheduleV1alpha1().AnalysisTasks(instance.Namespace).Get(ctx, instance.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	taskCopy := task.DeepCopy()
	taskCopy.Status.Status = status
	patch := client.MergeFrom(task)
	data, err := patch.Data(taskCopy)
	if err != nil {
		klog.Error("create patch failed", err)
		return err
	}

	// data == "{}", need not to patch
	if len(data) == 2 {
		return nil
	}

	_, err = r.K8SClient.Schedule().ScheduleV1alpha1().AnalysisTasks(instance.Namespace).Patch(ctx, instance.Name, patch.Type(), data, metav1.PatchOptions{})
	if err != nil {
		klog.Error(err)
		return err
	}
	return nil
}

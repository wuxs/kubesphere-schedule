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
	cranev1alpha1 "github.com/gocrane/api/analysis/v1alpha1"
	cranev1informer "github.com/gocrane/api/pkg/generated/informers/externalversions/analysis/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
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
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"sync"
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

	K8SClient                  k8s.Client
	ScheduleClient             schedule.Operator
	DeploymentInformer         v1.DeploymentInformer
	AnalyticsInformer          cranev1informer.AnalyticsInformer
	RecommendationInformer     cranev1informer.RecommendationInformer
	DynamicInformer            dynamicinformer.DynamicSharedInformerFactory
	NamespaceInformer          corev1informer.NamespaceInformer
	NameSpaceAnalysisTaskCache map[string]*v1alpha1.AnalysisTask            // cache namespace analysis task for new workload
	NameSpaceReverseIndex      map[string]map[string]*v1alpha1.AnalysisTask // map[<namespace>]map[<workloadKey>]*v1alpha1.AnalysisTask
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
			instance.Status.Status = schedulev1alpha1.PendingStatus
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
	case v1alpha1.WorkloadResourceType:
		return r.doReconcileWorkloadAnalysis(ctx, instance)
	case v1alpha1.NamespaceResourceType:
		return r.doReconcileNamespaceAnalysis(ctx, instance)
	default:
		klog.Infof("not support resource type %s", instance.Spec.Type)
		return ctrl.Result{}, nil
	}

}

func (r *AnalysisTaskReconciler) undoReconcile(ctx context.Context, instance *schedulev1alpha1.AnalysisTask) (ctrl.Result, error) {
	switch instance.Spec.Type {
	case v1alpha1.NamespaceResourceType:
		return r.undoReconcileNamespaceAnalysis(ctx, instance)
	case v1alpha1.WorkloadResourceType:
		return r.undoReconcileWorkloadAnalysis(ctx, instance)
		//r.ScheduleClient.CreateCraneAnalysis(analysis.Namespace, analysis.Spec.Target, analysis.Spec.CompletionStrategy)
	default:
		klog.Infof("not support resource type %s", instance.Spec.Type)
		return ctrl.Result{}, nil
	}
}

func (r *AnalysisTaskReconciler) doReconcileWorkloadAnalysis(ctx context.Context, instance *schedulev1alpha1.AnalysisTask) (ctrl.Result, error) {
	_ = log.FromContext(ctx)
	deployments := make([]*appsv1.Deployment, 0, len(instance.Spec.ResourceSelectors))
	statefulsets := make([]*appsv1.StatefulSet, 0, len(instance.Spec.ResourceSelectors))

	for _, resource := range instance.Spec.ResourceSelectors {
		switch resource.Kind {
		case v1alpha1.DeploymentResource:
			workload, err := r.K8SClient.Kubernetes().AppsV1().Deployments(instance.Namespace).Get(ctx, resource.Name, metav1.GetOptions{})
			if err == nil {
				deployments = append(deployments, workload)
				err = r.UpdateTargetMark(ctx, workload, instance)
				if err != nil {
					klog.Errorf("update deployment %s/%s annotation error %v", workload.Namespace, workload.Name, err)
				}

				r.UpdateIndexCache(instance.Namespace, resource.Kind, resource.Name, instance)
			}
		case v1alpha1.StatefulSetResource:
			workload, err := r.K8SClient.Kubernetes().AppsV1().StatefulSets(instance.Namespace).Get(ctx, resource.Name, metav1.GetOptions{})
			if err == nil {
				statefulsets = append(statefulsets, workload)
				err = r.UpdateTargetMark(ctx, workload, instance)
				if err != nil {
					klog.Errorf("update deployment %s/%s annotation error %v", workload.Namespace, workload.Name, err)
				}

				r.UpdateIndexCache(instance.Namespace, resource.Kind, resource.Name, instance)
			}
		default:
			klog.Errorf("unsupported kind %s", resource)
			continue
		}

		name := genAnalysisName(instance.Spec.Type, resource.Kind, resource.Name)
		analytics := convertAnalytics(name, resource, instance.Spec.CompletionStrategy)
		analytics = labelAnalyticsWithAnalysisName(analytics, instance)
		if err := r.ScheduleClient.CreateCraneAnalysis(ctx, instance.Namespace, name, analytics); err != nil {
			klog.Errorf("creat analysis error %s", err.Error())
			return ctrl.Result{}, err
		}
	}
	r.UpdateTaskStatistics(instance, deployments)
	return ctrl.Result{}, nil
}

func (r *AnalysisTaskReconciler) undoReconcileWorkloadAnalysis(ctx context.Context, instance *schedulev1alpha1.AnalysisTask) (ctrl.Result, error) {
	//删除 Analysis
	for _, resource := range instance.Spec.ResourceSelectors {
		if resource.Kind != schedulev1alpha1.DeploymentResource {
			klog.Errorf("unknown kind %s", resource.Name)
			continue
		}

		name := genAnalysisName(instance.Spec.Type, resource.Kind, resource.Name)
		analytics := convertAnalytics(name, resource, instance.Spec.CompletionStrategy)
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
			return ctrl.Result{}, nil
		}
		r.NameSpaceAnalysisTaskCache[namespace] = instance

		deployments, err := r.K8SClient.Kubernetes().AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			klog.Errorf("get deployment error: %s", err.Error())
			return ctrl.Result{}, err
		}
		for _, deployment := range deployments.Items {
			resource := convertResource(&deployment)
			name := genAnalysisName(instance.Spec.Type, resource.Kind, resource.Name)
			analytics := convertAnalytics(name, resource, instance.Spec.CompletionStrategy)
			analytics = labelAnalyticsWithAnalysisName(analytics, instance)
			err = r.ScheduleClient.CreateCraneAnalysis(ctx, namespace, name, analytics)
			if err != nil {
				klog.Errorf("creat analysis error: %s", err.Error())
				return ctrl.Result{}, err
			}

			// add deployment index
			r.UpdateIndexCache(instance.Namespace, resource.Kind, resource.Name, instance)
		}
	}
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

		//TODO add
		deployments, err := r.K8SClient.Kubernetes().AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			klog.Errorf("get deployment error: %s", err.Error())
			return ctrl.Result{}, err
		}
		for _, deployment := range deployments.Items {
			resource := convertResource(&deployment)
			name := genAnalysisName(instance.Spec.Type, resource.Kind, resource.Name)
			analytics := convertAnalytics(name, resource, instance.Spec.CompletionStrategy)
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

func (r *AnalysisTaskReconciler) handleDeploymentUpdateEvent(deployment *appsv1.Deployment) {
	namespace := deployment.Namespace
	if analysisTask, ok := r.NameSpaceAnalysisTaskCache[namespace]; ok {
		name := genAnalysisName(analysisTask.Spec.Type, deployment.Kind, deployment.Name)
		analytics := convertAnalytics(name, convertResource(deployment), analysisTask.Spec.CompletionStrategy)
		err := r.ScheduleClient.CreateCraneAnalysis(context.Background(),
			deployment.Namespace,
			name,
			analytics)
		if err != nil {
			klog.Errorf("create deployment analysis task %s error: %s", name, err.Error())
		}
		klog.V(4).Infof("create deployment analysis %s", name)
		r.UpdateIndexCache(deployment.Namespace, deployment.Kind, deployment.Name, analysisTask)
	} else {
		klog.V(4).Infof("skip create deployment analysis %s", deployment.Name)
	}
}

func (r *AnalysisTaskReconciler) handleDeploymentRemoveEvent(deployment *appsv1.Deployment) {
	klog.Infof("handle deployment remove event %s, do nothing", deployment.Name)
}

func (r *AnalysisTaskReconciler) DeploymentEventHandler() cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			deployment := obj.(*appsv1.Deployment)
			klog.V(4).Infof("reciver deployment add event %s", deployment.Name)
			r.handleDeploymentUpdateEvent(deployment)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			deployment := newObj.(*appsv1.Deployment)
			klog.V(4).Infof("reciver deployment update event %s", deployment.Name)
			r.handleDeploymentUpdateEvent(deployment)
		},
		DeleteFunc: func(obj interface{}) {
			deployment := obj.(*appsv1.Deployment)
			klog.V(4).Infof("delete deployment delete event %s", deployment.Name)
		},
	}
}

func (r *AnalysisTaskReconciler) UpdateIndexCache(namespace, workloadKind, workloadName string, instance *schedulev1alpha1.AnalysisTask) {
	//r.IndexCache[key] = instance
	key := genWorkloadIndexKey(namespace, workloadKind, workloadName)
	if _, ok := r.NameSpaceReverseIndex[namespace]; !ok {
		r.NameSpaceReverseIndex[namespace] = make(map[string]*schedulev1alpha1.AnalysisTask)
	}
	r.NameSpaceReverseIndex[namespace][key] = instance
}
func (r *AnalysisTaskReconciler) RemoveIndexCache(namespace, workloadKind, workloadName string) {
	//r.IndexCache[key] = instance
	key := genWorkloadIndexKey(namespace, workloadKind, workloadName)
	if _, ok := r.NameSpaceReverseIndex[namespace]; !ok {
		r.NameSpaceReverseIndex[namespace] = make(map[string]*schedulev1alpha1.AnalysisTask)
	}
	delete(r.NameSpaceReverseIndex[namespace], key)
	if len(r.NameSpaceReverseIndex[namespace]) == 0 {
		delete(r.NameSpaceReverseIndex, namespace)
	}
}

// TODO 修改 AnalysisTask 的所属 Target 信息
func (r *AnalysisTaskReconciler) UpdateTaskStatistics(instance *schedulev1alpha1.AnalysisTask, deployment []*appsv1.Deployment) {
	instance, err := r.K8SClient.Schedule().ScheduleV1alpha1().AnalysisTasks(instance.Namespace).Get(context.Background(), instance.Name, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("get analysis[%s/%s] task error: %s", instance.Namespace, instance.Name, err.Error())
		return
	}

	// update cache
	for _, deployment := range deployment {
		r.UpdateIndexCache(deployment.Namespace, deployment.Kind, deployment.Name, instance)
	}
}

func (r *AnalysisTaskReconciler) ConfigEventHandler() cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			r.UpdateScheduleConfig(obj)
			klog.V(4).Infof("reciver Installer add event %v", r.ClusterScheduleConfig)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			r.UpdateScheduleConfig(newObj)
			klog.V(4).Infof("reciver Installer update event %v", r.ClusterScheduleConfig)
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
	err := object.DataAs("spec.schedule", &config)
	if err != nil {
		klog.Errorf("parse ClusterConfig is error", object)
		return
	}
	r.ClusterScheduleConfig = config
	klog.Errorf("update cluster schedule config: %v", config)
}

// SetupWithManager sets up the controller with the Manager.
func (r *AnalysisTaskReconciler) SetupWithManager(mgr ctrl.Manager) error {
	urlruntime.Must(NotNil(r.K8SClient))
	urlruntime.Must(NotNil(r.ScheduleClient))
	urlruntime.Must(NotNil(r.DeploymentInformer))
	urlruntime.Must(NotNil(r.NamespaceInformer))
	urlruntime.Must(NotNil(r.DynamicInformer))
	urlruntime.Must(NotNil(r.ClusterScheduleConfig))
	urlruntime.Must(NotNil(r.AnalyticsInformer))
	urlruntime.Must(NotNil(r.RecommendationInformer))
	urlruntime.Must(NotNil(r.NameSpaceAnalysisTaskCache))
	urlruntime.Must(NotNil(r.NameSpaceReverseIndex))

	r.DeploymentInformer.Informer().AddEventHandler(r.DeploymentEventHandler())
	r.AnalyticsInformer.Informer().AddEventHandler(r.AnalyticsEventHandler())
	r.RecommendationInformer.Informer().AddEventHandler(r.RecommendationsEventHandler())
	r.DynamicInformer.ForResource(schema.GroupVersionResource{
		Group: "installer.kubesphere.io", Version: "v1alpha1", Resource: "clusterconfigurations",
	}).Informer().AddEventHandler(r.ConfigEventHandler())

	return ctrl.NewControllerManagedBy(mgr).
		For(&schedulev1alpha1.AnalysisTask{}).
		Complete(r)
}

func (r *AnalysisTaskReconciler) Mutating(ctx context.Context, request admission.Request) admission.Response {
	return admission.Denied("@TODO Mutating")
}

func (r *AnalysisTaskReconciler) Validating(ctx context.Context, request admission.Request) admission.Response {
	return admission.Denied("@TODO Validating")
}

func (r *AnalysisTaskReconciler) UpdateTargetMark(ctx context.Context, workload interface{}, analysisTask *v1alpha1.AnalysisTask) error {
	switch workload := workload.(type) {
	case *appsv1.Deployment:
		workloadCopy := workload.DeepCopy()
		if workloadCopy.Labels == nil {
			workloadCopy.Labels = make(map[string]string, 0)
		}
		if workloadCopy.Annotations == nil {
			workloadCopy.Annotations = make(map[string]string, 0)
		}
		workloadCopy.Labels[constants.AnalysisTaskAnnotationLabel] = analysisTask.Name
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
		return nil
	case *appsv1.StatefulSet:
		workloadCopy := workload.DeepCopy()
		if workloadCopy.Labels == nil {
			workloadCopy.Labels = make(map[string]string, 0)
		}
		if workloadCopy.Annotations == nil {
			workloadCopy.Annotations = make(map[string]string, 0)
		}
		workloadCopy.Labels[constants.AnalysisTaskAnnotationLabel] = analysisTask.Name
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
		return nil
	case *appsv1.DaemonSet:
		workloadCopy := workload.DeepCopy()
		if workloadCopy.Labels == nil {
			workloadCopy.Labels = make(map[string]string, 0)
		}
		if workloadCopy.Annotations == nil {
			workloadCopy.Annotations = make(map[string]string, 0)
		}
		workloadCopy.Labels[constants.AnalysisTaskAnnotationLabel] = analysisTask.Name
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
		return nil
	default:
		return fmt.Errorf("not support workload type %v", workload)
	}
}

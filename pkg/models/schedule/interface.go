/*
Copyright 2020 The KubeSphere Authors.

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

package schedule

import (
	"context"
	"fmt"
	cranealpha1 "github.com/gocrane/api/analysis/v1alpha1"
	ext "github.com/gocrane/api/pkg/generated/clientset/versioned"
	"github.com/mdaverde/jsonpath"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"
	"kubesphere.io/schedule/api"
	"kubesphere.io/schedule/api/schedule/v1alpha1"
	"kubesphere.io/schedule/pkg/apiserver/query"
	"kubesphere.io/schedule/pkg/client/clientset/versioned"
	"kubesphere.io/schedule/pkg/client/informers/externalversions"
	"kubesphere.io/schedule/pkg/constants"
	ks_informers "kubesphere.io/schedule/pkg/informers"
	resourcesV1alpha3 "kubesphere.io/schedule/pkg/models/resources/v1alpha3"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

const (
	configMapPrefix      = "kubeconfig-"
	kubeconfigNameFormat = configMapPrefix + "%s"
	defaultClusterName   = "local"
	defaultNamespace     = "default"
	kubeconfigNotify     = "notify"
	configMapKind        = "ConfigMap"
	configMapAPIVersion  = "v1"
	privateKeyAnnotation = "kubesphere.io/private-key"
	residual             = 72 * time.Hour
)

type Operator interface {
	//AnalysisTask
	CreateAnalysisTask(ctx context.Context, namespace string, task *v1alpha1.AnalysisTask) (*v1alpha1.AnalysisTask, error)
	ListAnalysisTask(ctx context.Context, query *query.Query) (api.ListResult, error)
	ModifyAnalysisTask(ctx context.Context, namespace, id string, task *v1alpha1.AnalysisTask) error
	DescribeAnalysisTask(ctx context.Context, namespace, id string) (*v1alpha1.AnalysisTask, error)
	DeleteAnalysisTask(ctx context.Context, namespace, id string) error
	ModifyAnalysisTaskConfig(ctx context.Context, schedule *SchedulerConfig) (*SchedulerConfig, error)

	//Crane
	CreateCraneAnalysis(ctx context.Context, namespace string, name string, analytics *cranealpha1.Analytics) error
	DeleteCraneAnalysis(ctx context.Context, namespace string, name string, analytics *cranealpha1.Analytics) error
}

func NewScheduleOperator(ksInformers ks_informers.InformerFactory,
	k8sClient kubernetes.Interface,
	scheduleClient versioned.Interface,
	resClient ext.Interface,
	dynamicClient dynamic.Interface, stopCh <-chan struct{}) Operator {
	klog.Infof("start helm repo informer")

	return &scheduleOperator{
		informers:      ksInformers.ScheduleSharedInformerFactory(),
		k8sClient:      k8sClient,
		scheduleClient: scheduleClient,
		resClient:      resClient,
		dynamicClient:  dynamicClient,
	}
}

type scheduleOperator struct {
	informers      externalversions.SharedInformerFactory
	scheduleClient versioned.Interface
	k8sClient      kubernetes.Interface
	resClient      ext.Interface
	dynamicClient  dynamic.Interface
}

func (s *scheduleOperator) DescribeAnalysisTask(ctx context.Context, namespace, id string) (*v1alpha1.AnalysisTask, error) {
	return s.scheduleClient.ScheduleV1alpha1().AnalysisTasks(namespace).Get(ctx, id, metav1.GetOptions{})
}

func (s *scheduleOperator) DeleteAnalysisTask(ctx context.Context, namespace, id string) error {
	return s.scheduleClient.ScheduleV1alpha1().AnalysisTasks(namespace).Delete(ctx, id, metav1.DeleteOptions{})
}

func (s *scheduleOperator) ModifyAnalysisTaskConfig(ctx context.Context, config *SchedulerConfig) (*SchedulerConfig, error) {

	gvr := schema.GroupVersionResource{Group: "installer.kubesphere.io", Version: "v1alpha1", Resource: "clusterconfigurations"}

	ksConfig, err := s.dynamicClient.Resource(gvr).Namespace(constants.KubeSphereNamespace).
		Get(ctx, constants.KsClusterConfigurationInstallerName, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			klog.Error(err, "Failed to get clusterconfigurations for install ks in dolphincluster")
			return nil, err
		}
	}

	ksConfigCopy := ksConfig.DeepCopy()
	fmt.Println(jsonpath.Get(ksConfigCopy.Object, "spec.scheduler.analysis.notifyThreshold"))
	if config.CPUNotifyPresent != nil {
		jsonpath.Set(ksConfigCopy.Object, "spec.schedule.analysis.notifyThreshold.cpu", config.CPUNotifyPresent)
	}
	if config.MemNotifyPresent != nil {
		jsonpath.Set(ksConfigCopy.Object, "spec.schedule.analysis.notifyThreshold.mem", config.MemNotifyPresent)
	}
	fmt.Println(jsonpath.Get(ksConfigCopy.Object, "spec.schedule.analysis.notifyThreshold"))

	patch := client.MergeFrom(ksConfig)
	data, err := patch.Data(ksConfigCopy)
	if err != nil {
		klog.Error("create patch failed", err)
		return nil, err
	}

	// data == "{}", need not to patch
	if len(data) == 2 {
		return nil, err
	}

	_, err = s.dynamicClient.Resource(gvr).Namespace(constants.KubeSphereNamespace).
		Patch(ctx, constants.KsClusterConfigurationInstallerName, patch.Type(), data, metav1.PatchOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			klog.Error(err, "Failed to get clusterconfigurations for install ks in dolphincluster")
			return nil, err
		}
	}

	return config, nil
}

func (s *scheduleOperator) CreateAnalysisTask(ctx context.Context, namespace string, task *v1alpha1.AnalysisTask) (*v1alpha1.AnalysisTask, error) {
	item, err := s.scheduleClient.ScheduleV1alpha1().AnalysisTasks(namespace).Create(ctx, task, metav1.CreateOptions{})

	if err != nil {
		klog.Errorf("list helm repo failed: %s", err)
		return nil, err
	}

	return item, nil
}

func (s *scheduleOperator) GetAnalysisTask(ctx context.Context, namespace, id string) (*v1alpha1.AnalysisTask, error) {
	analysis, err := s.scheduleClient.ScheduleV1alpha1().AnalysisTasks(namespace).Get(ctx, id, metav1.GetOptions{})

	if err != nil {
		klog.Error("get repo failed", err)
		return nil, err
	}
	return analysis, nil
}

func (s *scheduleOperator) ModifyAnalysisTask(ctx context.Context, namespace, id string, task *v1alpha1.AnalysisTask) error {
	analysis, err := s.scheduleClient.ScheduleV1alpha1().AnalysisTasks(namespace).Get(ctx, id, metav1.GetOptions{})

	if err != nil {
		klog.Error("get analysis failed", err)
		return err
	}

	analysisCopy := analysis.DeepCopy()
	analysisCopy.Spec = task.Spec
	analysisCopy.Labels = task.Labels

	patch := client.MergeFrom(analysis)
	data, err := patch.Data(analysisCopy)
	if err != nil {
		klog.Error("create patch failed", err)
		return err
	}

	// data == "{}", need not to patch
	if len(data) == 2 {
		return nil
	}

	_, err = s.scheduleClient.ScheduleV1alpha1().AnalysisTasks(namespace).Patch(ctx, id, patch.Type(), data, metav1.PatchOptions{})

	if err != nil {
		klog.Error(err)
		return err
	}
	return nil
}

func (s *scheduleOperator) ListAnalysisTask(ctx context.Context, query *query.Query) (api.ListResult, error) {
	tasks, err := s.scheduleClient.ScheduleV1alpha1().AnalysisTasks(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return api.ListResult{}, err
	}
	var result = make([]runtime.Object, len(tasks.Items))
	for i, _ := range tasks.Items {
		result = append(result, &tasks.Items[i])
	}

	return *resourcesV1alpha3.DefaultList(result, query, resourcesV1alpha3.DefaultCompare(), resourcesV1alpha3.DefaultFilter()), nil

}

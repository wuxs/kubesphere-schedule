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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
	"kubesphere.io/schedule/api/schedule/v1alpha1"
	"kubesphere.io/schedule/pkg/client/clientset/versioned"
	"kubesphere.io/schedule/pkg/client/informers/externalversions"
	ks_informers "kubesphere.io/schedule/pkg/informers"
	"kubesphere.io/schedule/pkg/utils/resourcecache"
	"strings"
)

type Interface interface {
	CreateAnalysis(namespace string, target v1alpha1.ResourceSelector, strategy cranealpha1.CompletionStrategy) error
}

func NewScheduleOperator(ksInformers ks_informers.InformerFactory, ksClient versioned.Interface, resClient ext.Interface, stopCh <-chan struct{}) Interface {
	klog.Infof("start helm repo informer")
	cachedAnalysisData := resourcecache.NewAnalysisCache()

	//analyticsesInformer := ksInformers.ExtensionSharedInformerFactory().Analysis().V1alpha1().Analytics().Informer()
	//analyticsesInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
	//	AddFunc: func(obj interface{}) {
	//		fmt.Println("analyticsesInformer.AddFunc")
	//		r := obj.(*cranev1.Analytics)
	//		cachedAnalysisData.AddAnalytics(r)
	//	},
	//	UpdateFunc: func(oldObj, newObj interface{}) {
	//		fmt.Println("analyticsesInformer.UpdateFunc")
	//		oldAddAnalytics := oldObj.(*cranev1.Analytics)
	//		newAddAnalytics := newObj.(*cranev1.Analytics)
	//		cachedAnalysisData.UpdateAddAnalytics(oldAddAnalytics, newAddAnalytics)
	//	},
	//	DeleteFunc: func(obj interface{}) {
	//		fmt.Println("analyticsesInformer.DeleteFunc")
	//		r := obj.(*cranev1.Analytics)
	//		cachedAnalysisData.DeleteAnalytics(r)
	//	},
	//})

	return &scheduleOperator{
		cached:    cachedAnalysisData,
		informers: ksInformers.KubeSphereSharedInformerFactory(),
		ksClient:  ksClient,
		resClient: resClient,
	}
}

type scheduleOperator struct {
	cached    resourcecache.ResourceCache
	informers externalversions.SharedInformerFactory
	ksClient  versioned.Interface
	resClient ext.Interface
}

func (s *scheduleOperator) CreateAnalysis(namespace string, target v1alpha1.ResourceSelector, strategy cranealpha1.CompletionStrategy) error {
	analytics := convertAnalytics(target, strategy)
	analytics.Name = strings.ToLower(fmt.Sprintf("kubesphere-%s-%s", target.Kind, target.Name))
	ret, err := s.resClient.AnalysisV1alpha1().Analytics(namespace).Create(context.Background(), analytics, metav1.CreateOptions{})
	if err != nil {
		klog.Errorf("create analytics error: %v", err)
		return fmt.Errorf("create analytics error: %w", err)
	}
	klog.Infof("create analytics ok: %v", ret)
	return nil
}

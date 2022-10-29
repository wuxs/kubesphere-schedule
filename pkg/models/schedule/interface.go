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
	"k8s.io/klog"

	"kubesphere.io/schedule/pkg/client/clientset/versioned"
	ks_informers "kubesphere.io/schedule/pkg/informers"
)

type Interface interface {
	AnalysisInterface
}

type openpitrixOperator struct {
	AnalysisInterface
}

func NewScheduleOperator(ksInformers ks_informers.InformerFactory, ksClient versioned.Interface, stopCh <-chan struct{}) Interface {
	klog.Infof("start helm repo informer")
	//cachedAnalysisData := reposcache.NewAnalysisCache()
	//helmReposInformer := ksInformers.KubeSphereSharedInformerFactory().Application().V1alpha1().HelmRepos().Informer()
	//helmReposInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
	//	AddFunc: func(obj interface{}) {
	//		r := obj.(*v1alpha1.HelmRepo)
	//		cachedReposData.AddRepo(r)
	//	},
	//	UpdateFunc: func(oldObj, newObj interface{}) {
	//		oldRepo := oldObj.(*v1alpha1.HelmRepo)
	//		newRepo := newObj.(*v1alpha1.HelmRepo)
	//		cachedAnalysisData.UpdateRepo(oldRepo, newRepo)
	//	},
	//	DeleteFunc: func(obj interface{}) {
	//		r := obj.(*v1alpha1.HelmRepo)
	//		cachedAnalysisData.DeleteRepo(r)
	//	},
	//})
	//
	//ctgInformer := ksInformers.KubeSphereSharedInformerFactory().Application().V1alpha1().HelmCategories().Informer()
	//ctgInformer.AddIndexers(map[string]cache.IndexFunc{
	//	reposcache.CategoryIndexer: func(obj interface{}) ([]string, error) {
	//		ctg, _ := obj.(*v1alpha1.HelmCategory)
	//		return []string{ctg.Spec.Name}, nil
	//	},
	//})
	//indexer := ctgInformer.GetIndexer()
	//
	//cachedAnalysisData.SetCategoryIndexer(indexer)

	return &openpitrixOperator{
		//AttachmentInterface:  newAttachmentOperator(s3Client),
		//ApplicationInterface: newApplicationOperator(cachedAnalysisData, ksInformers.KubeSphereSharedInformerFactory(), ksClient, s3Client),
		//RepoInterface:        newRepoOperator(cachedAnalysisData, ksInformers.KubeSphereSharedInformerFactory(), ksClient),
		//ReleaseInterface:     newReleaseOperator(cachedAnalysisData, ksInformers.KubernetesSharedInformerFactory(), ksInformers.KubeSphereSharedInformerFactory(), ksClient, cc),
		//CategoryInterface:    newCategoryOperator(cachedAnalysisData, ksInformers.KubeSphereSharedInformerFactory(), ksClient),
	}
}

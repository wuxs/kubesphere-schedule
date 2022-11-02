/*
Copyright 2022 The KubeSphere Authors.

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

// Code generated by lister-gen. DO NOT EDIT.

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	v1alpha1 "kubesphere.io/schedule/api/schedule/v1alpha1"
)

// AnalysisTaskLister helps list AnalysisTasks.
// All objects returned here must be treated as read-only.
type AnalysisTaskLister interface {
	// List lists all AnalysisTasks in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.AnalysisTask, err error)
	// AnalysisTasks returns an object that can list and get AnalysisTasks.
	AnalysisTasks(namespace string) AnalysisTaskNamespaceLister
	AnalysisTaskListerExpansion
}

// analysisTaskLister implements the AnalysisTaskLister interface.
type analysisTaskLister struct {
	indexer cache.Indexer
}

// NewAnalysisTaskLister returns a new AnalysisTaskLister.
func NewAnalysisTaskLister(indexer cache.Indexer) AnalysisTaskLister {
	return &analysisTaskLister{indexer: indexer}
}

// List lists all AnalysisTasks in the indexer.
func (s *analysisTaskLister) List(selector labels.Selector) (ret []*v1alpha1.AnalysisTask, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.AnalysisTask))
	})
	return ret, err
}

// AnalysisTasks returns an object that can list and get AnalysisTasks.
func (s *analysisTaskLister) AnalysisTasks(namespace string) AnalysisTaskNamespaceLister {
	return analysisTaskNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// AnalysisTaskNamespaceLister helps list and get AnalysisTasks.
// All objects returned here must be treated as read-only.
type AnalysisTaskNamespaceLister interface {
	// List lists all AnalysisTasks in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.AnalysisTask, err error)
	// Get retrieves the AnalysisTask from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1alpha1.AnalysisTask, error)
	AnalysisTaskNamespaceListerExpansion
}

// analysisTaskNamespaceLister implements the AnalysisTaskNamespaceLister
// interface.
type analysisTaskNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all AnalysisTasks in the indexer for a given namespace.
func (s analysisTaskNamespaceLister) List(selector labels.Selector) (ret []*v1alpha1.AnalysisTask, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.AnalysisTask))
	})
	return ret, err
}

// Get retrieves the AnalysisTask from the indexer for a given namespace and name.
func (s analysisTaskNamespaceLister) Get(name string) (*v1alpha1.AnalysisTask, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha1.Resource("analysistask"), name)
	}
	return obj.(*v1alpha1.AnalysisTask), nil
}
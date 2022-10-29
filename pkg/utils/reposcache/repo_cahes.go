// /*
// Copyright 2020 The KubeSphere Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
// */
//

package reposcache

import (
	"k8s.io/client-go/tools/cache"
	"sync"

	"kubesphere.io/schedule/api/schedule/v1alpha1"
)

const (
	CategoryIndexer       = "category_indexer"
	CategoryAnnotationKey = "app.kubesphere.io/category"
)

var WorkDir string

func NewAnalysisCache() AnalysisCache {
	return &cachedAnalysis{
		analysis:              map[string]*v1alpha1.Analysis{},
		builtinCategoryCounts: map[string]int{},
	}
}

type AnalysisCache interface {
	AddAnalysis(repo *v1alpha1.Analysis) error
	DeleteAnalysis(repo *v1alpha1.Analysis) error
	UpdateAnalysis(old, new *v1alpha1.Analysis) error
}

type workspace string
type cachedAnalysis struct {
	sync.RWMutex

	chartsInAnalysis map[workspace]map[string]int

	// builtinCategoryCounts saves the count of every category in the built-in repo.
	builtinCategoryCounts map[string]int

	analysis map[string]*v1alpha1.Analysis

	// indexerOfHelmCtg is the indexer of HelmCategory, used to query the category id from category name.
	indexerOfHelmCtg cache.Indexer
}

func (c *cachedAnalysis) AddAnalysis(analysis *v1alpha1.Analysis) error {
	//TODO implement me
	panic("implement me")
}

func (c *cachedAnalysis) DeleteAnalysis(analysis *v1alpha1.Analysis) error {
	//TODO implement me
	panic("implement me")
}

func (c *cachedAnalysis) UpdateAnalysis(old, new *v1alpha1.Analysis) error {
	//TODO implement me
	panic("implement me")
}

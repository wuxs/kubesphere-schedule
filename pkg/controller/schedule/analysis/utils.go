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
	"fmt"
	cranev1alpha1 "github.com/gocrane/api/analysis/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	schedulev1alpha1 "kubesphere.io/schedule/api/schedule/v1alpha1"
	"kubesphere.io/schedule/pkg/constants"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"strings"
)

func NotNil(obj interface{}, msgs ...string) error {
	if !IsNil(obj) {
		return nil
	}
	if len(msgs) > 0 {
		info := ""
		info = strings.Join(msgs, " ")
		info = fmt.Sprintf("(%s)", info)
		return fmt.Errorf("[%v] is nil %v", obj, msgs)
	}
	return fmt.Errorf("[%v] is nil", obj)
}
func IsNil(v interface{}) bool {
	valueOf := reflect.ValueOf(v)

	k := valueOf.Kind()

	switch k {
	case reflect.Chan, reflect.Func, reflect.Map, reflect.Ptr, reflect.UnsafePointer, reflect.Interface, reflect.Slice:
		return valueOf.IsNil()
	default:
		return v == nil
	}
}

func convertResource(deployment *appsv1.Deployment) schedulev1alpha1.ResourceSelector {
	gv := appsv1.SchemeGroupVersion
	return schedulev1alpha1.ResourceSelector{
		Kind:       gv.WithKind("Deployment").Kind,
		APIVersion: gv.String(),
		Name:       deployment.Name,
	}
}

var getQKV = apiutil.GVKForObject

func convertAnalytics(target schedulev1alpha1.ResourceSelector, strategy cranev1alpha1.CompletionStrategy) (name string, analytics *cranev1alpha1.Analytics) {
	name = strings.ToLower(fmt.Sprintf("kubesphere-%s-%s", target.Kind, target.Name))
	analytics = &cranev1alpha1.Analytics{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: cranev1alpha1.AnalyticsSpec{
			Type: cranev1alpha1.AnalysisTypeResource,
			ResourceSelectors: []cranev1alpha1.ResourceSelector{cranev1alpha1.ResourceSelector{
				Kind: target.Kind, APIVersion: target.APIVersion, Name: target.Name,
			}},
			CompletionStrategy: strategy,
		},
	}
	return analytics.Name, analytics
}

// labelAnalyticsWithAnalysisName adds a kubesphere.io/analysis=[analysisName] label to crane.analysis
func labelAnalyticsWithAnalysisName(analytics *cranev1alpha1.Analytics, analysisTask *schedulev1alpha1.AnalysisTask) *cranev1alpha1.Analytics {
	if analytics.Labels == nil {
		analytics.Labels = make(map[string]string, 0)
	}

	label := fmt.Sprintf("%s/%s", analysisTask.Spec.Type, analysisTask.Name)
	analytics.Labels[constants.AnalysisLabel] = label

	return analytics
}

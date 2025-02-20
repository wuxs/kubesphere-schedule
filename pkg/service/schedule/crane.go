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

package schedule

import (
	"context"
	"fmt"
	cranealpha1 "github.com/gocrane/api/analysis/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
	"kubesphere.io/schedule/api/schedule/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (s *scheduleOperator) CreateCraneAnalysis(ctx context.Context, namespace string, target v1alpha1.ResourceSelector, strategy cranealpha1.CompletionStrategy) error {
	name, analytics := convertAnalytics(target, strategy)
	analytics, err := s.resClient.AnalysisV1alpha1().Analytics(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) { //creat
		ret, err := s.resClient.AnalysisV1alpha1().Analytics(namespace).Create(context.Background(), analytics, metav1.CreateOptions{})
		if err != nil {
			klog.Errorf("create analytics error: %v", err)
			return fmt.Errorf("create analytics error: %w", err)
		}
		klog.Infof("create analytics ok: %v", ret)
		return nil
	} else if err == nil { //update
		analyticsCopy := analytics.DeepCopy()
		analyticsCopy.Spec = analyticsCopy.Spec

		patch := client.MergeFrom(analytics)
		data, err := patch.Data(analyticsCopy)
		if err != nil {
			klog.Error("create patch failed", err)
			return err
		}
		// data == "{}", need not to patch
		if len(data) == 2 {
			klog.Infof("update analytics skip: no need to modify")
			return nil
		}

		_, err = s.resClient.AnalysisV1alpha1().Analytics(namespace).Patch(ctx, name, patch.Type(), data, metav1.PatchOptions{})

		if err != nil {
			klog.Error(err)
			return err
		}
		return nil
	} else {
		klog.Errorf("create analytics error: %v", err)
		return fmt.Errorf("create analytics error: %w", err)
	}

}

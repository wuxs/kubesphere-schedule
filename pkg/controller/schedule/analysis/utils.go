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
	appsv1 "k8s.io/api/apps/v1"
	schedulev1alpha1 "kubesphere.io/schedule/api/schedule/v1alpha1"
	"strings"
)

func NotNil(obj interface{}, msgs ...string) error {
	if obj != nil {
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

func convertResource(deployment *appsv1.Deployment) schedulev1alpha1.ResourceSelector {
	gv := appsv1.SchemeGroupVersion
	return schedulev1alpha1.ResourceSelector{
		Kind:       gv.WithKind("Deployment").Kind,
		APIVersion: gv.String(),
		Name:       deployment.Name,
	}
}

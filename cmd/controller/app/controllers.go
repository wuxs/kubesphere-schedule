/*
Copyright 2019 The KubeSphere Authors.

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

package app

import (
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog"
	"kubesphere.io/schedule/api/schedule/v1alpha1"
	"kubesphere.io/schedule/pkg/service/model"
	"kubesphere.io/schedule/pkg/service/schedule"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"kubesphere.io/schedule/cmd/controller/app/options"
	"kubesphere.io/schedule/pkg/controller/schedule/analysis"

	"kubesphere.io/schedule/pkg/client/k8s"
	"kubesphere.io/schedule/pkg/informers"
)

var allControllers = []string{
	"analysis",
}

// setup all available controllers one by one
func addAllControllers(mgr manager.Manager, client k8s.Client, informerFactory informers.InformerFactory,
	cmOptions *options.KubeSphereControllerManagerOptions,
	stopCh <-chan struct{}) error {

	////////////////////////////////////
	// begin init necessary informers
	////////////////////////////////////
	// kubernetesInformer := informerFactory.KubernetesSharedInformerFactory()
	// kubesphereInformer := informerFactory.KubeSphereSharedInformerFactory()
	////////////////////////////////////
	// end informers
	////////////////////////////////////

	////////////////////////////////////////////////////////
	// begin init controller and add to manager one by one
	////////////////////////////////////////////////////////

	// "analysis" controller
	// if cmOptions.IsControllerEnabled("analysis") && !cmOptions.GatewayOptions.IsEmpty(){

	scheduleClient := schedule.NewScheduleOperator(informerFactory, client.Kubernetes(), client.Schedule(), client.ExtResources(), client.Dynamic(), stopCh)
	analysisReconciler := &analysis.AnalysisTaskReconciler{
		Client:                 mgr.GetClient(),
		K8SClient:              client,
		ScheduleClient:         scheduleClient,
		DeploymentInformer:     informerFactory.KubernetesSharedInformerFactory().Apps().V1().Deployments(),
		NamespaceInformer:      informerFactory.KubernetesSharedInformerFactory().Core().V1().Namespaces(),
		AnalyticsInformer:      informerFactory.CraneInformer().Analysis().V1alpha1().Analytics(),
		RecommendationInformer: informerFactory.CraneInformer().Analysis().V1alpha1().Recommendations(),
		DynamicInformer:        informerFactory.DynamicSharedInformerFactory(),
		NameSpaceCache:         make(map[string]*v1alpha1.AnalysisTask),
		SchedulerConfig:        model.SchedulerConfig{},
	}
	addControllerWithSetup(mgr, "analysis", analysisReconciler)

	// log all controllers process result
	for _, name := range allControllers {
		if cmOptions.IsControllerEnabled(name) {
			if addSuccessfullyControllers.Has(name) {
				klog.Infof("%s controller is enabled and added successfully.", name)
			} else {
				klog.Infof("%s controller is enabled but is not going to run due to its dependent component being disabled.", name)
			}
		} else {
			klog.Infof("%s controller is disabled by controller selectors.", name)
		}
	}

	return nil
}

var addSuccessfullyControllers = sets.NewString()

type setupableController interface {
	SetupWithManager(mgr ctrl.Manager) error
}

func addControllerWithSetup(mgr manager.Manager, name string, controller setupableController) {
	if err := controller.SetupWithManager(mgr); err != nil {
		klog.Fatalf("Unable to create %v controller: %v", name, err)
	}
	addSuccessfullyControllers.Insert(name)
}

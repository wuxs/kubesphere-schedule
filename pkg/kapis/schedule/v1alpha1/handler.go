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

package v1alpha1

import (
	"fmt"
	restful "github.com/emicklei/go-restful"
	ext "github.com/gocrane/api/pkg/generated/clientset/versioned"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"
	"kubesphere.io/schedule/api"
	"kubesphere.io/schedule/api/schedule/v1alpha1"
	"kubesphere.io/schedule/pkg/apiserver/query"
	"kubesphere.io/schedule/pkg/client/clientset/versioned"
	scheduleoptions "kubesphere.io/schedule/pkg/client/schedule"
	"kubesphere.io/schedule/pkg/constants"
	"kubesphere.io/schedule/pkg/informers"
	"kubesphere.io/schedule/pkg/kapis"
	"kubesphere.io/schedule/pkg/service/model"
	"kubesphere.io/schedule/pkg/service/schedule"
)

var (
	ErrScheduleClientNil = fmt.Errorf("schedule client is nil")
	ErrScheduleTypeNil   = fmt.Errorf("analysis task type error")
)

type scheduleHandler struct {
	schedule schedule.Operator
}

func NewScheduleClient(ksInformers informers.InformerFactory,
	k8sClient kubernetes.Interface,
	scheduleClient versioned.Interface,
	resourcesClient ext.Interface,
	dynamicClient dynamic.Interface, option *scheduleoptions.Options, stopCh <-chan struct{}) schedule.Operator {

	return schedule.NewScheduleOperator(ksInformers,
		k8sClient,
		scheduleClient,
		resourcesClient,
		dynamicClient, stopCh)
}

func (h *scheduleHandler) ListScheduler(request *restful.Request, response *restful.Response) {
	//TODO implement me
	panic("implement me")
}

func (h *scheduleHandler) ModifyScheduler(request *restful.Request, response *restful.Response) {
	ctx := request.Request.Context()
	namespaceID := request.PathParameter("namespace")
	var analysisTask *v1alpha1.AnalysisTask
	err := request.ReadEntity(&analysisTask)
	if err != nil {
		klog.V(4).Infoln(err)
		api.HandleBadRequest(response, nil, err)
		return
	}

	if analysisTask.Spec.Type != v1alpha1.ResourceTypeDeployment {
		klog.V(4).Infoln(ErrScheduleTypeNil)
		api.HandleBadRequest(response, nil, ErrScheduleTypeNil)
		return
	}

	Result(h.schedule.CreateAnalysisTask(ctx, namespaceID, analysisTask)).
		Output(request, response, "create deployment analysis task %s", analysisTask.Name)
}
func (h *scheduleHandler) ListAnalysisTask(request *restful.Request, response *restful.Response) {
	//analysis := request.PathParameter("analysis")
	ctx := request.Request.Context()
	query := query.ParseQueryParameter(request)
	if h.Ready(request, response) {
		objs, err := h.schedule.ListAnalysisTask(ctx, query)
		kapis.ErrorHandle(request, response, objs, err)
	}
}

func (h *scheduleHandler) CreateAnalysis(request *restful.Request, response *restful.Response) {
	ctx := request.Request.Context()
	namespaceID := request.PathParameter("namespace")
	var analysisTask *v1alpha1.AnalysisTask
	err := request.ReadEntity(&analysisTask)
	if err != nil {
		klog.V(4).Infoln(err)
		api.HandleBadRequest(response, nil, err)
		return
	}

	if analysisTask.Spec.Type != v1alpha1.ResourceTypeDeployment {
		klog.V(4).Infoln(ErrScheduleTypeNil)
		api.HandleBadRequest(response, nil, ErrScheduleTypeNil)
		return
	}

	Result(h.schedule.CreateAnalysisTask(ctx, namespaceID, analysisTask)).
		Output(request, response, "create deployment analysis task %s", analysisTask.Name)
}

func (h *scheduleHandler) CreateNamespaceAnalysis(request *restful.Request, response *restful.Response) {
	ctx := request.Request.Context()
	var analysisTask *v1alpha1.AnalysisTask
	err := request.ReadEntity(&analysisTask)
	if err != nil {
		klog.V(4).Infoln(err)
		api.HandleBadRequest(response, nil, err)
		return
	}

	if analysisTask.Spec.Type != v1alpha1.ResourceTypeNamespace {
		klog.V(4).Infoln(ErrScheduleTypeNil)
		api.HandleBadRequest(response, nil, ErrScheduleTypeNil)
		return
	}

	Result(h.schedule.CreateAnalysisTask(ctx, constants.KubesphereScheduleNamespace, analysisTask)).
		Output(request, response, "create deployment analysis task %s", analysisTask.Name)
}

func (h *scheduleHandler) ModifyAnalysisTask(request *restful.Request, response *restful.Response) {
	ctx := request.Request.Context()
	analysisID := request.PathParameter("analysis")
	namespaceID := request.PathParameter("namespace")
	var analysisTask *v1alpha1.AnalysisTask
	err := request.ReadEntity(&analysisTask)
	if err != nil {
		klog.V(4).Infoln(err)
		api.HandleBadRequest(response, nil, err)
		return
	}

	Result(h.schedule.ModifyAnalysisTask(ctx, namespaceID, analysisID, analysisTask)).
		Output(request, response, "modify analysis task: ", analysisID)
}

func (h *scheduleHandler) DescribeAnalysisTask(request *restful.Request, response *restful.Response) {
	ctx := request.Request.Context()
	analysisID := request.PathParameter("analysis")
	namespaceID := request.PathParameter("namespace")

	Result(h.schedule.DescribeAnalysisTask(ctx, namespaceID, analysisID)).
		Output(request, response, "describe analysis task: ", analysisID)
}

func (h *scheduleHandler) DeleteAnalysisTask(request *restful.Request, response *restful.Response) {
	ctx := request.Request.Context()
	analysisID := request.PathParameter("analysis")
	namespace := request.PathParameter("namespace")

	Result(h.schedule.DeleteAnalysisTask(ctx, namespace, analysisID)).
		Output(request, response, "delete analysis task: ", analysisID)
}

func (h *scheduleHandler) ModifyAnalysisTaskConfig(request *restful.Request, response *restful.Response) {
	ctx := request.Request.Context()
	var analysisConfig *model.SchedulerConfig
	err := request.ReadEntity(&analysisConfig)
	if err != nil {
		klog.V(4).Infoln(err)
		api.HandleBadRequest(response, nil, err)
		return
	}

	Result(h.schedule.ModifyAnalysisTaskConfig(ctx, analysisConfig)).
		Output(request, response, "modify analysis task config: ", analysisConfig)
}

func (h *scheduleHandler) Ready(request *restful.Request, response *restful.Response) bool {
	if h.schedule != nil {
		return true
	}
	kapis.HandleBadRequest(response, request, ErrScheduleClientNil)
	return false
}

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
	restful "github.com/emicklei/go-restful"
	restfulspec "github.com/emicklei/go-restful-openapi"
	"github.com/go-openapi/spec"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"kubesphere.io/schedule/api"
	"kubesphere.io/schedule/api/schedule/v1alpha1"
	"kubesphere.io/schedule/pkg/constants"
	"kubesphere.io/schedule/pkg/server/params"
	"kubesphere.io/schedule/pkg/service"
	"kubesphere.io/schedule/pkg/service/model"
	"kubesphere.io/schedule/pkg/service/schedule"
	"net/http"

	"kubesphere.io/schedule/pkg/apiserver/runtime"
	"kubesphere.io/schedule/pkg/client/clientset/versioned"
	scheduleoptions "kubesphere.io/schedule/pkg/client/schedule"
	"kubesphere.io/schedule/pkg/informers"
	"kubesphere.io/schedule/pkg/server/errors"
)

const (
	GroupName = "schedule.kubesphere.io"
)

var GroupVersion = schema.GroupVersion{Group: GroupName, Version: "v1"}

func SwaggerObject(swo *spec.Swagger) {
	swo.Info = &spec.Info{
		InfoProps: spec.InfoProps{
			Title:       "UserService",
			Description: "Resource for managing Users",
			Contact: &spec.ContactInfo{
				Name:  "john",
				Email: "john@doe.rp",
				URL:   "http://johndoe.org",
			},
			License: &spec.License{
				Name: "MIT",
				URL:  "http://mit.org",
			},
			Version: "1.0.0",
		},
	}
	swo.Tags = []spec.Tag{spec.Tag{TagProps: spec.TagProps{
		Name:        "users",
		Description: "Managing users"}}}
}

func AddToContainer(c *restful.Container, ksInfomrers informers.InformerFactory, ksClient versioned.Interface, options *scheduleoptions.Options, scheduleClient schedule.Operator) error {
	webservice := runtime.NewWebService(GroupVersion)

	mimePatch := []string{restful.MIME_JSON, runtime.MimeJsonPatchJson, runtime.MimeMergePatchJson}
	handler := &scheduleHandler{
		scheduleClient,
	}

	//获取调度器列表 GET /scheduler @TODO
	webservice.Route(webservice.GET("/scheduler").
		To(handler.ListScheduler).
		Returns(http.StatusOK, api.StatusOK, model.SchedulerConfig{}).
		Metadata(restfulspec.KeyOpenAPITags, []string{constants.ScheduleTag}).
		Doc("List all applications within the specified cluster"))

	//设置默认调度器 POST /scheduler (通过CM存放) @TODO
	webservice.Route(webservice.PATCH("/scheduler").
		Consumes(mimePatch...).
		To(handler.ModifyScheduler).
		Doc("Modify default scheduler").
		Metadata(restfulspec.KeyOpenAPITags, []string{constants.ScheduleTag}).
		Reads(model.SchedulerConfig{}).
		Returns(http.StatusOK, api.StatusOK, model.SchedulerConfig{}))

	//修改分析任务提醒设置 POST /analysis/notify (通过CM存放) @TODO
	webservice.Route(webservice.PATCH("/analysis/config").
		Consumes(mimePatch...).
		To(handler.ModifyAnalysisTaskConfig).
		Doc("Modify analysis config").
		Metadata(restfulspec.KeyOpenAPITags, []string{constants.ScheduleTag}).
		Reads(model.SchedulerConfig{}).
		Returns(http.StatusOK, api.StatusOK, model.SchedulerConfig{}))

	//获取分析任务列表 GET /analysis @TODO
	webservice.Route(webservice.GET("/analysis").
		To(handler.ListAnalysisTask).
		Returns(http.StatusOK, api.StatusOK, service.PageableResponse{}).
		Metadata(restfulspec.KeyOpenAPITags, []string{constants.ScheduleTag}).
		Doc("List all applications within the specified cluster").
		Param(webservice.QueryParameter(params.ConditionsParam, "query conditions, connect multiple conditions with commas, equal symbol for exact query, wave symbol for fuzzy query e.g. name~a").
			Required(false).
			DataFormat("key=value,key~value").
			DefaultValue("")).
		//Param(webservice.PathParameter("workspace", "the workspace of the project.").Required(true)).
		//Param(webservice.PathParameter("cluster", "the cluster of the project.").Required(true)).
		Param(webservice.QueryParameter(params.PagingParam, "paging query, e.g. limit=100,page=1").
			Required(false).
			DataFormat("limit=%d,page=%d").
			DefaultValue("limit=10,page=1")))

	//创建分析任务 POST /analysis @TODO
	webservice.Route(webservice.POST("/analysis").
		Deprecate().
		To(handler.CreateNamespaceAnalysis).
		Doc("Create a new app template version").
		Metadata(restfulspec.KeyOpenAPITags, []string{constants.ScheduleTag}).
		Reads(v1alpha1.AnalysisTask{}).
		Param(webservice.QueryParameter("validate", "Validate format of package(pack by op tool)")).
		Returns(http.StatusOK, api.StatusOK, v1alpha1.AnalysisTask{}))
	//创建分析任务 POST /analysis @TODO
	webservice.Route(webservice.POST("/namespaces/{namespace}/analysis").
		Deprecate().
		To(handler.CreateAnalysis).
		Doc("Create a new app template version").
		Metadata(restfulspec.KeyOpenAPITags, []string{constants.ScheduleTag}).
		Reads(v1alpha1.AnalysisTask{}).
		Param(webservice.QueryParameter("validate", "Validate format of package(pack by op tool)")).
		Returns(http.StatusOK, api.StatusOK, v1alpha1.AnalysisTask{}).
		Param(webservice.PathParameter("namespace", "namespace id").Required(true)))

	//修改分析任务 POST /analysis @TODO
	webservice.Route(webservice.PATCH("/namespaces/{namespace}/analysis/{analysis}").
		Consumes(mimePatch...).
		To(handler.ModifyAnalysisTask).
		Doc("Modify analysis").
		Metadata(restfulspec.KeyOpenAPITags, []string{constants.ScheduleTag}).
		Reads(v1alpha1.AnalysisTask{}).
		Returns(http.StatusOK, api.StatusOK, v1alpha1.AnalysisTask{}).
		Param(webservice.PathParameter("namespace", "namespace id").Required(true)).
		Param(webservice.PathParameter("analysis", "analysis id").Required(true)))

	//获取分析任务详情 GET /analysis/<id> @TODO
	webservice.Route(webservice.GET("/namespaces/{namespace}/analysis/{analysis}").
		To(handler.DescribeAnalysisTask).
		Doc("Create a global repository, which is used to store package of app").
		Metadata(restfulspec.KeyOpenAPITags, []string{constants.ScheduleTag}).
		Returns(http.StatusOK, api.StatusOK, v1alpha1.AnalysisTask{}).
		Param(webservice.PathParameter("analysis", "analysis id")))

	//删除分析任务 DELETE /scheduler @TODO
	webservice.Route(webservice.DELETE("/namespaces/{namespace}/analysis/{analysis}").
		To(handler.DeleteAnalysisTask).
		Doc("Create a global repository, which is used to store package of app").
		Param(webservice.PathParameter("analysis", "analysis id")).
		Returns(http.StatusOK, api.StatusOK, errors.Error{}))

	c.Add(webservice)

	return nil
}

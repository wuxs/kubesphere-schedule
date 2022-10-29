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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"kubesphere.io/schedule/pkg/models/schedule"

	"kubesphere.io/schedule/pkg/apiserver/runtime"
	"kubesphere.io/schedule/pkg/client/clientset/versioned"
	scheduleoptions "kubesphere.io/schedule/pkg/client/schedule"
	"kubesphere.io/schedule/pkg/informers"
)

const (
	GroupName = "schedule.io"
)

var GroupVersion = schema.GroupVersion{Group: GroupName, Version: "v1"}

func AddToContainer(c *restful.Container, ksInfomrers informers.InformerFactory, ksClient versioned.Interface, options *scheduleoptions.Options, scheduleClient schedule.Interface) error {
	webservice := runtime.NewWebService(GroupVersion)

	//mimePatch := []string{restful.MIME_JSON, runtime.MimeJsonPatchJson, runtime.MimeMergePatchJson}
	//handler := &scheduleHandler{
	//	scheduleClient,
	//}

	//webservice.Route(webservice.POST("/repos").
	//	To(handler.CreateRepo).
	//	Doc("Create a global repository, which is used to store package of app").
	//	Metadata(restfulspec.KeyOpenAPITags, []string{constants.ScheduleTag}).
	//	Param(webservice.QueryParameter("validate", "Validate repository")).
	//	Returns(http.StatusOK, api.StatusOK, schedule.CreateRepoResponse{}).
	//	Reads(schedule.CreateRepoRequest{}))
	//webservice.Route(webservice.POST("/workspaces/{workspace}/repos").
	//	To(handler.CreateRepo).
	//	Doc("Create repository in the specified workspace, which is used to store package of app").
	//	Metadata(restfulspec.KeyOpenAPITags, []string{constants.ScheduleTag}).
	//	Param(webservice.QueryParameter("validate", "Validate repository")).
	//	Returns(http.StatusOK, api.StatusOK, schedule.CreateRepoResponse{}).
	//	Reads(schedule.CreateRepoRequest{}))
	//webservice.Route(webservice.DELETE("/repos/{repo}").
	//	To(handler.DeleteRepo).
	//	Doc("Delete the specified global repository").
	//	Metadata(restfulspec.KeyOpenAPITags, []string{constants.ScheduleTag}).
	//	Returns(http.StatusOK, api.StatusOK, errors.Error{}).
	//	Param(webservice.PathParameter("repo", "repo id")))

	c.Add(webservice)

	return nil
}

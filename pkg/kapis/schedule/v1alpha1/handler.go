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
	"kubesphere.io/schedule/pkg/client/clientset/versioned"
	scheduleoptions "kubesphere.io/schedule/pkg/client/schedule"
	"kubesphere.io/schedule/pkg/informers"
	"kubesphere.io/schedule/pkg/models/schedule"
)

type scheduleHandler struct {
	schedule schedule.Interface
}

func NewScheduleClient(ksInformers informers.InformerFactory, ksClient versioned.Interface, option *scheduleoptions.Options, stopCh <-chan struct{}) schedule.Interface {

	return schedule.NewScheduleOperator(ksInformers, ksClient, stopCh)
}

func (h *scheduleHandler) CreateRepo(req *restful.Request, resp *restful.Response) {

	//createRepoRequest := &schedule.CreateRepoRequest{}
	//err := req.ReadEntity(createRepoRequest)
	//if err != nil {
	//	klog.V(4).Infoln(err)
	//	api.HandleBadRequest(resp, nil, err)
	//	return
	//}
	//
	//createRepoRequest.Workspace = new(string)
	//*createRepoRequest.Workspace = req.PathParameter("workspace")
	//
	//user, _ := request.UserFrom(req.Request.Context())
	//creator := ""
	//if user != nil {
	//	creator = user.GetName()
	//}
	//parsedUrl, err := url.Parse(createRepoRequest.URL)
	//if err != nil {
	//	api.HandleBadRequest(resp, nil, err)
	//	return
	//}
	//userInfo := parsedUrl.User
	//// trim credential from url
	//parsedUrl.User = nil
	//
	//syncPeriod := 0
	//// If SyncPeriod is empty, ignore it.
	//if createRepoRequest.SyncPeriod != "" {
	//	duration, err := time.ParseDuration(createRepoRequest.SyncPeriod)
	//	if err != nil {
	//		api.HandleBadRequest(resp, nil, err)
	//		return
	//	} else if duration > 0 {
	//		syncPeriod = int(math.Max(float64(duration/time.Second), constants.HelmRepoMinSyncPeriod))
	//	}
	//}
	//
	//repo := v1alpha1.HelmRepo{
	//	ObjectMeta: metav1.ObjectMeta{
	//		Name: idutils.GetUuid36(v1alpha1.HelmRepoIdPrefix),
	//		Annotations: map[string]string{
	//			constants.CreatorAnnotationKey: creator,
	//		},
	//		Labels: map[string]string{
	//			constants.WorkspaceLabelKey: *createRepoRequest.Workspace,
	//		},
	//	},
	//	Spec: v1alpha1.HelmRepoSpec{
	//		Name:        createRepoRequest.Name,
	//		Url:         parsedUrl.String(),
	//		SyncPeriod:  syncPeriod,
	//		Description: stringutils.ShortenString(createRepoRequest.Description, 512),
	//	},
	//}
	//
	//if syncPeriod > 0 {
	//	repo.Annotations[v1alpha1.RepoSyncPeriod] = createRepoRequest.SyncPeriod
	//}
	//
	//if strings.HasPrefix(createRepoRequest.URL, "https://") || strings.HasPrefix(createRepoRequest.URL, "http://") {
	//	if userInfo != nil {
	//		repo.Spec.Credential.Username = userInfo.Username()
	//		repo.Spec.Credential.Password, _ = userInfo.Password()
	//	}
	//} else if strings.HasPrefix(createRepoRequest.URL, "s3://") {
	//	cfg := v1alpha1.S3Config{}
	//	err := json.Unmarshal([]byte(createRepoRequest.Credential), &cfg)
	//	if err != nil {
	//		api.HandleBadRequest(resp, nil, err)
	//		return
	//	}
	//	repo.Spec.Credential.S3Config = cfg
	//}
	//
	//var result interface{}
	//// 1. validate repo
	//result, err = h.schedule.ValidateRepo(createRepoRequest.URL, &repo.Spec.Credential)
	//if err != nil {
	//	klog.Errorf("validate repo failed, err: %s", err)
	//	api.HandleBadRequest(resp, nil, err)
	//	return
	//}
	//
	//// 2. create repo
	//validate, _ := strconv.ParseBool(req.QueryParameter("validate"))
	//if !validate {
	//	if repo.GetTrueName() == "" {
	//		api.HandleBadRequest(resp, nil, fmt.Errorf("repo name is empty"))
	//		return
	//	}
	//	result, err = h.schedule.CreateRepo(&repo)
	//}
	//
	//if err != nil {
	//	klog.Errorln(err)
	//	handleScheduleError(resp, err)
	//	return
	//}
	//
	//resp.WriteEntity(result)
}

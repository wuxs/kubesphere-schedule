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

package schedule

type SchedulerConfig struct {
	// name of the default scheduler
	Scheduler        *string `json:"scheduler,omitempty"`
	MemNotifyPresent *int64  `json:"mem_notify_present,omitempty"`
	CPUNotifyPresent *int64  `json:"cpu_notify_present,omitempty"`
}

const (
	CreateTime = "create_time"
	StatusTime = "status_time"

	VersionId       = "version_id"
	AnalysisId      = "analysis_id"
	Status          = "status"
	Type            = "type"
	Visibility      = "visibility"
	AppId           = "app_id"
	Keyword         = "keyword"
	ISV             = "isv"
	WorkspaceLabel  = "workspace"
	BuiltinRepoId   = "repo-helm"
	StatusActive    = "active"
	StatusSuspended = "suspended"
	ActionRecover   = "recover"
	ActionSuspend   = "suspend"
	ActionCancel    = "cancel"
	ActionPass      = "pass"
	ActionReject    = "reject"
	ActionSubmit    = "submit"
	ActionRelease   = "release"
	Ascending       = "ascending"
	ActionIndex     = "index"
)

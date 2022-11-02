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

package v1alpha1

import (
	"github.com/emicklei/go-restful"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog"
	"kubesphere.io/schedule/api"
	"kubesphere.io/schedule/pkg/server/errors"
)

type object struct {
	Type   int
	Object interface{}
	Err    error
}

var _emptyObject = object{Type: 0}

func Result(args ...interface{}) *object {
	if len(args) < 1 {
		return &_emptyObject
	}
	e1 := args[0]
	switch e1 := e1.(type) {
	case error:
		return &object{Type: 1, Err: e1}
	default:
		if len(args) >= 2 {
			if e2, ok := args[1].(error); ok {
				return &object{Type: 2, Object: e1, Err: e2}
			}
		}
	}
	return &_emptyObject
}

func (o *object) Output(request *restful.Request, response *restful.Response, args ...interface{}) {
	err := o.Err
	if err != nil {
		klog.Error(err)
		if apierrors.IsNotFound(err) {
			api.HandleNotFound(response, nil, err)
			return
		}
		api.HandleBadRequest(response, nil, err)
		return
	} else {
		klog.V(4).Info(args...)
	}
	switch o.Type {
	case 0:
		response.WriteEntity(errors.None)
	case 1:
		response.WriteEntity(errors.None)
	case 2:
		response.WriteEntity(o.Object)

	}
}

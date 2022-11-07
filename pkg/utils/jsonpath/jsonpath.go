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

package jsonpath

import (
	"encoding/json"
	"fmt"
	"github.com/mdaverde/jsonpath"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"strconv"
)

type Object struct {
	data interface{}
}

func New(object interface{}) *Object {
	switch object := object.(type) {
	case *unstructured.Unstructured:
		return &Object{data: object.Object}
	default:
		return &Object{data: object}
	}
}

func (o *Object) Set(path string, value interface{}) error {
	return jsonpath.Set(o.data, path, value)
}

func (o *Object) Get(path string) (interface{}, error) {
	return jsonpath.Get(o.data, path)
}

func (o *Object) GetString(path string) (string, error) {
	ret, err := jsonpath.Get(o.data, path)
	if err != nil {
		return "", err
	}
	return fmt.Sprint(ret), nil
}

func (o *Object) GetInt64(path string) (int64, error) {
	val, err := o.GetString(path)
	if err != nil {
		return 0, err
	}
	ret, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return 0, err
	}
	return ret, nil
}

func (o *Object) GetFloat64(path string) (float64, error) {
	val, err := o.GetString(path)
	if err != nil {
		return 0, err
	}
	ret, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return 0, err
	}
	return ret, nil
}

//func (o *Object) GetStringList(path string) ([]string, error) {
//	ret, err := jsonpath.Get(o.data, path)
//	if err != nil {
//		return []string{}, err
//	}
//	switch ret := ret.(type) {
//	case []interface{}:
//		fmt.Println(ret)
//	case Object:
//	default:
//
//	}
//	//return fmt.Sprint(ret), nil
//}

func (o *Object) DataAs(path string, entry interface{}) error {
	elem, err := o.Get(path)
	if err != nil {
		return err
	}
	data, err := json.Marshal(elem)
	if err != nil {
		return err
	}

	err = json.Unmarshal(data, &entry)
	if err != nil {
		err = fmt.Errorf("unmarshal  <%s> to data error: %w", path, err)
		return err
	}
	return nil
}

/*
Copyright 2021.

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

//+kubebuilder:validation:Required

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// WorkerSpec defines the desired state of Worker
type WorkerSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	//+kubebuilder:validation:Minimum=0
	// Replicas is the number of the worker replicas
	Replicas int32 `json:"replicas"`

	//+kubebuilder:validation:MinLength=10
	//KaldiImage is the url of worker image
	KalidImage string `json:"kaldiImage"`

	//+kubebuilder:validation:MinLength=10
	//PythonImage is the url of worker image
	PythonImage string `json:"pythonImage"`

	//+kubebuilder:validation:MinLength=4
	//ModelName is the modelname of the worker
	ModelName string `json:"modelName"`
}

// WorkerStatus defines the observed state of Worker
type WorkerStatus struct {
	// Pods contain the list of worker pod name
	Pods []string `json:"pods"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Worker is the Schema for the workers API
type Worker struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WorkerSpec   `json:"spec,omitempty"`
	Status WorkerStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// WorkerList contains a list of Worker
type WorkerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Worker `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Worker{}, &WorkerList{})
}

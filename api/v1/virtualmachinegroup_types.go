/*
Copyright 2024.

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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// VirtualMachineGroupSpec defines the desired state of VirtualMachineGroup
type VirtualMachineGroupSpec struct {
	Replicas         int32  `json:"replicas,omitempty"`         // Mutable
	AvailabilityZone string `json:"availabilityZone,omitempty"` // Immutable
	DevAdmin         string `json:"devAdmin,omitempty"`         // Mutable
	Flavor           string `json:"flavor,omitempty"`           // Immutable
	Image            string `json:"image,omitempty"`            // Immutable
}

// VirtualMachineGroupStatus defines the observed state of VirtualMachineGroup
type VirtualMachineGroupStatus struct {
	Replicas        int32                  `json:"replicas,omitempty"`
	DevAdmin        string                 `json:"devAdmin,omitempty"`
	VirtualMachines []VirtualMachineStatus `json:"virtualMachines,omitempty"`

	// Conditions store the status conditions of the Memcached instances
	// +operator-sdk:csv:customresourcedefinitions:type=status
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// VirtualMachineGroup is the Schema for the virtualmachinegroups API
type VirtualMachineGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VirtualMachineGroupSpec   `json:"spec,omitempty"`
	Status VirtualMachineGroupStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// VirtualMachineGroupList contains a list of VirtualMachineGroup
type VirtualMachineGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VirtualMachineGroup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VirtualMachineGroup{}, &VirtualMachineGroupList{})
}

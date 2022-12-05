/*
Copyright 2022.

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
	"github.com/RHEcosystemAppEng/dbaas-operator/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// RDSInventory is the Schema for the rdsinventories API
type RDSInventory struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   v1beta1.DBaaSInventorySpec   `json:"spec,omitempty"`
	Status v1beta1.DBaaSInventoryStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// RDSInventoryList contains a list of RDSInventory
type RDSInventoryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RDSInventory `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RDSInventory{}, &RDSInventoryList{})
}

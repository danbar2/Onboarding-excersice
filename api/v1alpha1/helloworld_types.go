package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type HelloworldSpec struct {
	DefaultContent string `json:"defaultContent,omitempty"`

	ReplicaCount int32 `json:"replicaCount,omitempty"`
}

type HelloworldStatus struct {
	AvailableReplicas int32 `json:"availableReplicas,omitempty"`

	ReadyReplicas int32 `json:"readyReplicas,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Namespaced

type Helloworld struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HelloworldSpec   `json:"spec,omitempty"`
	Status HelloworldStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

type HelloworldList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Helloworld `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Helloworld{}, &HelloworldList{})
}

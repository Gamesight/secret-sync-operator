package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html

// RemoteSecretSpec defines the desired state of RemoteSecret
// +k8s:openapi-gen=true
type RemoteSecretSpec struct {
	// +kubebuilder:validation:MinLength=1
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
}

// SynchronizedSecretSpec defines the desired state of SynchronizedSecret
// +k8s:openapi-gen=true
type SynchronizedSecretSpec struct {
	// Desired state of cluster
	RemoteSecret RemoteSecretSpec `json:"remoteSecret,omitempty"`
}

// SynchronizedSecretStatus defines the observed state of SynchronizedSecret
// +k8s:openapi-gen=true
type SynchronizedSecretStatus struct {
	// Define observed state of cluster
	Status   string `json:"status"`
	LastSync string `json:"last_sync"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SynchronizedSecret is the Schema for the synchronizedsecrets API
// +k8s:openapi-gen=true
type SynchronizedSecret struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SynchronizedSecretSpec   `json:"spec,omitempty"`
	Status SynchronizedSecretStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SynchronizedSecretList contains a list of SynchronizedSecret
type SynchronizedSecretList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SynchronizedSecret `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SynchronizedSecret{}, &SynchronizedSecretList{})
}

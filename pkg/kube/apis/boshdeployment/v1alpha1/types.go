package v1alpha1

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"code.cloudfoundry.org/cf-operator/pkg/kube/apis"
)

// This file is safe to edit
// It's used as input for the Kube code generator
// Run "make generate" after modifying this file

// ReferenceType lists all the types of Reference we can supports
type ReferenceType = string

// Valid values for ref types
const (
	// ConfigMapReference represents ConfigMap reference
	ConfigMapReference ReferenceType = "configmap"
	// SecretReference represents Secret reference
	SecretReference ReferenceType = "secret"
	// URLReference represents URL reference
	URLReference ReferenceType = "url"

	ManifestSpecName        string = "manifest"
	OpsSpecName             string = "ops"
	ImplicitVariableKeyName string = "value"
)

var (
	// LabelDeploymentName is the label key for manifest name
	LabelDeploymentName = fmt.Sprintf("%s/deployment-name", apis.GroupName)
	// LabelDeploymentSecretType is the label key for secret type
	LabelDeploymentSecretType = fmt.Sprintf("%s/secret-type", apis.GroupName)
	// AnnotationLinkProvidesKey is the key for the quarks links 'provides' JSON
	AnnotationLinkProvidesKey = fmt.Sprintf("%s/provides", apis.GroupName)
	// AnnotationLinkProviderService is the annotation key used on services to identify the link provider
	AnnotationLinkProviderService = fmt.Sprintf("%s/link-provider-name", apis.GroupName)
)

// BOSHDeploymentSpec defines the desired state of BOSHDeployment
type BOSHDeploymentSpec struct {
	Manifest ResourceReference   `json:"manifest"`
	Ops      []ResourceReference `json:"ops,omitempty"`
}

// ResourceReference defines the resource reference type and location
type ResourceReference struct {
	Name string        `json:"name"`
	Type ReferenceType `json:"type"`
}

// BOSHDeploymentStatus defines the observed state of BOSHDeployment
type BOSHDeploymentStatus struct {
	// Timestamp for the last reconcile
	LastReconcile *metav1.Time `json:"lastReconcile"`
	// State of the BoshDeployment
	State string `json:"status"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BOSHDeployment is the Schema for the boshdeployments API
// +k8s:openapi-gen=true
type BOSHDeployment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BOSHDeploymentSpec   `json:"spec,omitempty"`
	Status BOSHDeploymentStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BOSHDeploymentList contains a list of BOSHDeployment
type BOSHDeploymentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BOSHDeployment `json:"items"`
}

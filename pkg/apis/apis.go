package apis

import (
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

// AddToSchemes may be used to add all resources defined in the project to a Scheme
var AddToSchemes runtime.SchemeBuilder

// AddToScheme adds all Resources to the Scheme
func AddToScheme(s *runtime.Scheme) error {
	return AddToSchemes.AddToScheme(s)
}

// GetScheme creates and returns a new runtime.Scheme with all custom resources registered
func GetScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	// Add the core Kubernetes types
	_ = clientgoscheme.AddToScheme(scheme)
	// Add our custom types
	_ = AddToScheme(scheme)
	return scheme
}

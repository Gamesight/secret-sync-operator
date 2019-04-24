package controller

import (
	"github.com/Innervate/secret-sync-operator/pkg/controller/synchronizedsecret"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, synchronizedsecret.Add)
}

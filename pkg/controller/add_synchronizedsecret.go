// Package controller contains controllers for custom resources
package controller

import (
	"github.com/Gamesight/secret-sync-operator/pkg/controller/synchronizedsecret"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, synchronizedsecret.Add)
}

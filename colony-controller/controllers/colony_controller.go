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

package controllers

import (
	"context"
	"fmt"
	"reflect"
	"time"

	abcoptimizerv1 "abc-optimizer/api/v1"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
)

// ColonyReconciler reconciles a Colony object
type ColonyReconciler struct {
	client.Client
	Log      logr.Logger
	Recorder record.EventRecorder
	Scheme   *runtime.Scheme
}

//+kubebuilder:rbac:groups=abc-optimizer.innoventestech.com,resources=colonies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=abc-optimizer.innoventestech.com,resources=colonies/status,verbs=get;list;update;patch
//+kubebuilder:rbac:groups=abc-optimizer.innoventestech.com,resources=colonies/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Colony object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.2/pkg/reconcile

func (r *ColonyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLogger := log.FromContext(ctx)
	reqLogger.Info("Reconciling on Colony resource")

	colonyInstance := &abcoptimizerv1.Colony{}
	err := r.Get(ctx, req.NamespacedName, colonyInstance)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("object already deleted")
			return ctrl.Result{}, nil
		}
	} else {
		reqLogger.Info("Namespace: " + req.Namespace + "Name: " + req.Name)
	}

	instance := colonyInstance.DeepCopy()

	reqLogger.Info("reconciling: " + fmt.Sprint(instance))

	if err := r.cleanupOwnedResources(ctx, reqLogger, instance); err != nil {
		reqLogger.Error(err, "failed to clean up old Deployment resources for this Colony")
		return ctrl.Result{}, err
	}

	reqLogger.Info("Instance status: " + fmt.Sprint(instance.Status))

	if instance.Status.EmployeeBeeCycles > instance.Spec.TotalCycles || instance.Status.OnlookerBeeCycles > instance.Spec.TotalCycles {
		reqLogger.Info("Completed Processing, skipping colony processor")

		// Employee Bee
		reqLogger.Info("checking if an existing Employee Bee Deployment exists for this Colony")
		employeeBeeDeployment := apps.Deployment{}
		err := r.Client.Get(ctx, client.ObjectKey{Namespace: instance.Namespace, Name: employeeBeeName}, &employeeBeeDeployment)
		if errors.IsNotFound(err) {
			reqLogger.Info("could not find existing Employee Bee Deployment for Colony")
		} else {
			reqLogger.Info("103: Deleting employee deploymnet")
			if err := r.Client.Delete(ctx, &employeeBeeDeployment, &client.DeleteOptions{}); err != nil {
				reqLogger.Error(err, "failed to delete Employee Bee Deployment resource")
				return ctrl.Result{}, err
			}
		}

		// Onlooker Bee
		onlookerBeeDeployment := apps.Deployment{}
		err = r.Client.Get(ctx, client.ObjectKey{Namespace: instance.Namespace, Name: onlookerBeeName}, &onlookerBeeDeployment)
		if errors.IsNotFound(err) {
			reqLogger.Info("could not find existing Onlooker Bee Deployment for Colony")
		} else {
			reqLogger.Info("116: Deleting onlooker deploymnet")
			if err := r.Client.Delete(ctx, &onlookerBeeDeployment, &client.DeleteOptions{}); err != nil {
				reqLogger.Error(err, "failed to delete Onlooker Bee Deployment resource")
				return ctrl.Result{}, err
			}
		}

		// Foodsource
		foodsourceDeployment := apps.Deployment{}
		err = r.Client.Get(ctx, client.ObjectKey{Namespace: instance.Namespace, Name: foodsourceName}, &foodsourceDeployment)
		if errors.IsNotFound(err) {
			reqLogger.Info("could not find existing Foodsource Deployment for Colony")
		} else {
			reqLogger.Info("116: Deleting foodsource deploymnet")
			if err := r.Client.Delete(ctx, &foodsourceDeployment, &client.DeleteOptions{}); err != nil {
				reqLogger.Error(err, "failed to delete Foodsource Deployment resource")
				return ctrl.Result{}, err
			}
		}

		return ctrl.Result{}, nil
	}

	// Foodsource

	if result, err := r.foodsourceController(ctx, reqLogger, instance, req); err != nil {
		reqLogger.Error(err, "failed to create foodsource deployment")
		return result, err
	}

	// Employee Bees

	if result, err := r.employeeBeeController(ctx, reqLogger, instance); err != nil {
		reqLogger.Error(err, "failed to create employee bee deployment")
		return result, err
	}

	// Onlooker Bees

	if result, err := r.onlookerBeeController(ctx, reqLogger, instance); err != nil {
		reqLogger.Error(err, "failed to create onlooker bee deployment")
		return result, err
	}

	reqLogger.Info("resource status synced")

	if !reflect.DeepEqual(instance.Status, colonyInstance.Status) {
		reqLogger.Info("Colony status before update: " + fmt.Sprint(instance.Status))
		tempInstance := &abcoptimizerv1.Colony{}
		err = r.Client.Get(ctx, req.NamespacedName, tempInstance)
		if err != nil {
			if errors.IsNotFound(err) {
				reqLogger.Info("object already deleted")
				return ctrl.Result{}, nil
			}
		}
		if err := r.Client.Status().Update(ctx, instance); err != nil {
			reqLogger.Error(err, "failed to update colony status")
			return ctrl.Result{}, err
		}

		for count := 1; count < 20; count++ {
			tempInstance := &abcoptimizerv1.Colony{}
			err = r.Client.Get(ctx, req.NamespacedName, tempInstance)
			if err != nil {
				if errors.IsNotFound(err) {
					reqLogger.Info("object already deleted")
					return ctrl.Result{}, nil
				}
			} else {
				reqLogger.Info("Namespace: " + req.Namespace + "Name: " + req.Name)
			}
			reqLogger.Info(fmt.Sprint(count) + ": Colony status after update: " + fmt.Sprint(tempInstance.Status))
			if reflect.DeepEqual(instance.Status, tempInstance.Status) {
				break
			}
			time.Sleep(2 * time.Second)
		}
	}

	return ctrl.Result{}, nil
}

// cleanupOwnedResources will Delete any existing Colonys that were created
func (r *ColonyReconciler) cleanupOwnedResources(ctx context.Context, log logr.Logger, Colony *abcoptimizerv1.Colony) error {
	log.Info("finding existing Colony deployments")

	// List all deployments owned by this Colony
	var deployments apps.DeploymentList
	if err := r.List(ctx, &deployments, client.InNamespace(Colony.Namespace), client.MatchingFields{deploymentOwnerKey: Colony.Name}); err != nil {
		return err
	}

	deleted := 0
	for _, depl := range deployments.Items {
		if depl.Name == employeeBeeName || depl.Name == foodsourceName || depl.Name == onlookerBeeName {
			// If this deployment's name matches the one on the MyKind resource
			// then do not delete it.
			continue
		}
		log.Info("396: Deleting unknown deploymnet")
		if err := r.Client.Delete(ctx, &depl); err != nil {
			log.Error(err, "failed to delete Colony")
			return err
		}

		r.Recorder.Eventf(Colony, core.EventTypeNormal, "Deleted", "Deleted bee %q", depl.Name)
		deleted++
	}

	log.Info("finished cleaning up old Colonys", "number_deleted", deleted)

	return nil
}

var (
	deploymentOwnerKey = ".metadata.controller"
)

func (r *ColonyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.TODO(), &apps.Deployment{}, deploymentOwnerKey, func(rawObj client.Object) []string {
		// grab the Deployment object, extract the owner...
		depl := rawObj.(*apps.Deployment)
		owner := metav1.GetControllerOf(depl)
		if owner == nil {
			return nil
		}
		// ...make sure it's a Colony...
		if owner.APIVersion != abcoptimizerv1.GroupVersion.String() || owner.Kind != "Colony" {
			return nil
		}

		// ...and if so, return it
		return []string{owner.Name}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&abcoptimizerv1.Colony{}).
		Owns(&apps.Deployment{}).
		Complete(r)
}

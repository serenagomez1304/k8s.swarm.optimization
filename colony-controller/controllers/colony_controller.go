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

//+kubebuilder:rbac:groups=abc-optimizer.pesu.edu,resources=colonies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=abc-optimizer.pesu.edu,resources=colonies/status,verbs=get;list;update;patch
//+kubebuilder:rbac:groups=abc-optimizer.pesu.edu,resources=colonies/finalizers,verbs=update

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
	reqLogger.V(4).Info("CCX:====================================== ==========================================")
	reqLogger.V(4).Info("CCX:Reconciling on Colony resource")

	colonyInstance := &abcoptimizerv1.Colony{}
	err := r.Get(ctx, req.NamespacedName, colonyInstance)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.V(4).Info("CCX:object already deleted")
			return ctrl.Result{}, nil
		}
	} else {
		reqLogger.V(8).Info("CCX:Namespace: " + req.Namespace + "Name: " + req.Name)
	}

	instance := colonyInstance.DeepCopy()

	reqLogger.V(9).Info("CCX:reconciling: " + fmt.Sprint(instance))

	if err := r.cleanupOwnedResources(ctx, reqLogger, instance); err != nil {
		reqLogger.Error(err, "failed to clean up old Deployment resources for this Colony")
		return ctrl.Result{}, err
	}

	reqLogger.V(7).Info("CCX:Instance status: " + colonyStatusToString(instance.Status))

	if instance.Status.EmployeeBeeCycles > instance.Spec.TotalCycles || instance.Status.OnlookerBeeCycles > instance.Spec.TotalCycles {
		reqLogger.V(8).Info("CCX:Completed Processing, skipping colony processor")

		// Employee Bee
		reqLogger.V(8).Info("CCX:checking if an existing Employee Bee Deployment exists for this Colony")
		employeeBeeDeployment := apps.Deployment{}
		err := r.Client.Get(ctx, client.ObjectKey{Namespace: instance.Namespace, Name: employeeBeeName}, &employeeBeeDeployment)
		if errors.IsNotFound(err) {
			reqLogger.V(4).Info("CCX:could not find existing Employee Bee Deployment for Colony")
		} else {
			reqLogger.V(8).Info("CCX:103: Deleting employee deployment")
			if err := r.Client.Delete(ctx, &employeeBeeDeployment, &client.DeleteOptions{}); err != nil {
				reqLogger.Error(err, "failed to delete Employee Bee Deployment resource")
				return ctrl.Result{}, err
			}
		}

		// Onlooker Bee
		onlookerBeeDeployment := apps.Deployment{}
		err = r.Client.Get(ctx, client.ObjectKey{Namespace: instance.Namespace, Name: onlookerBeeName}, &onlookerBeeDeployment)
		if errors.IsNotFound(err) {
			reqLogger.V(4).Info("CCX:could not find existing Onlooker Bee Deployment for Colony")
		} else {
			reqLogger.V(8).Info("CCX:116: Deleting onlooker deployment")
			if err := r.Client.Delete(ctx, &onlookerBeeDeployment, &client.DeleteOptions{}); err != nil {
				reqLogger.Error(err, "failed to delete Onlooker Bee Deployment resource")
				return ctrl.Result{}, err
			}
		}

		// Foodsource
		foodsourceDeployment := apps.Deployment{}
		err = r.Client.Get(ctx, client.ObjectKey{Namespace: instance.Namespace, Name: foodsourceName}, &foodsourceDeployment)
		if errors.IsNotFound(err) {
			reqLogger.V(4).Info("CCX:could not find existing Foodsource Deployment for Colony")
		} else {
			reqLogger.V(8).Info("CCX:116: Deleting foodsource deployment")
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

	// if !reflect.DeepEqual(instance.Status, colonyInstance.Status) {
	reqLogger.V(8).Info("CCX:resource status synced")
	reqLogger.V(7).Info("CCX:Colony status before update: " + colonyStatusToString(instance.Status))
	tempInstance := &abcoptimizerv1.Colony{}
	err = r.Client.Get(ctx, req.NamespacedName, tempInstance)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.V(4).Info("CCX:object already deleted")
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
				reqLogger.V(4).Info("CCX:object already deleted")
				return ctrl.Result{}, nil
			}
		} else {
			reqLogger.V(8).Info("CCX:Namespace: " + req.Namespace + "Name: " + req.Name)
		}
		reqLogger.V(7).Info(fmt.Sprint(count) + ": Colony status after update: " + colonyStatusToString(tempInstance.Status))
		if reflect.DeepEqual(instance.Status, tempInstance.Status) {
			break
		}
		time.Sleep(2 * time.Second)
		mergedInstance := mergeInstances(instance, tempInstance)
		reqLogger.V(7).Info("CCX:Trying merged instance: " + colonyStatusToString(mergedInstance.Status))
		if err := r.Client.Status().Update(ctx, mergedInstance); err != nil {
			reqLogger.Error(err, "failed to update colony status, retrying")
			// return ctrl.Result{}, err
		}
	}
	// } else {
	// 	reqLogger.V(X).Info("CCX:Did not sync, no change")
	// 	reqLogger.V(X).Info("CCX:Reconcilation input: " + fmt.Sprint(colonyInstance.Status))
	// 	reqLogger.V(X).Info("CCX:Reconcilation output: " + fmt.Sprint(instance.Status))
	// }
	reqLogger.V(4).Info("CCX:====================================== ==========================================")
	return ctrl.Result{}, nil
}

func mergeInstances(instance1 *abcoptimizerv1.Colony, instance2 *abcoptimizerv1.Colony) *abcoptimizerv1.Colony {
	for bee, _ := range instance1.Status.EmployeeBees {
		newEmpStatus := abcoptimizerv1.BeeStatus{}
		newEmpStatus.Status = instance1.Status.EmployeeBees[bee].Status
		newEmpStatus.FoodsourceId = instance1.Status.EmployeeBees[bee].FoodsourceId
		newEmpStatus.ObjectiveFunction = instance2.Status.EmployeeBees[bee].ObjectiveFunction
		newEmpStatus.FoodsourceVector = instance1.Status.EmployeeBees[bee].FoodsourceVector
		newEmpStatus.FoodsourceTrialCount = instance1.Status.EmployeeBees[bee].FoodsourceTrialCount
		newEmpStatus.ObjFuncStatus = instance2.Status.EmployeeBees[bee].ObjFuncStatus
		instance1.Status.EmployeeBees[bee] = newEmpStatus
	}
	for bee, _ := range instance1.Status.OnlookerBees {
		newOnlStatus := abcoptimizerv1.BeeStatus{}
		newOnlStatus.Status = instance1.Status.OnlookerBees[bee].Status
		newOnlStatus.FoodsourceId = instance1.Status.OnlookerBees[bee].FoodsourceId
		newOnlStatus.ObjectiveFunction = instance2.Status.OnlookerBees[bee].ObjectiveFunction
		newOnlStatus.FoodsourceVector = instance1.Status.OnlookerBees[bee].FoodsourceVector
		newOnlStatus.FoodsourceTrialCount = instance1.Status.OnlookerBees[bee].FoodsourceTrialCount
		newOnlStatus.ObjFuncStatus = instance2.Status.OnlookerBees[bee].ObjFuncStatus
		instance1.Status.OnlookerBees[bee] = newOnlStatus
	}
	return instance1
}

// cleanupOwnedResources will Delete any existing Colonys that were created
func (r *ColonyReconciler) cleanupOwnedResources(ctx context.Context, log logr.Logger, Colony *abcoptimizerv1.Colony) error {
	log.Info("CCX:cleanupOwnedResources: finding existing Colony deployments")

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
		log.Info("CCX:cleanupOwnedResources: Deleting unknown deployment")
		if err := r.Client.Delete(ctx, &depl); err != nil {
			log.Error(err, "CCX:cleanupOwnedResources: failed to delete Colony")
			return err
		}

		r.Recorder.Eventf(Colony, core.EventTypeNormal, "Deleted", "Deleted bee %q", depl.Name)
		deleted++
	}

	log.Info("CCX:cleanupOwnedResources: Finished cleaning up old Colonys", "number_deleted", deleted)

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

func colonyStatusToString(status abcoptimizerv1.ColonyStatus) string {
	colonyLogStatus := ""
	colonyLogStatus += "emp:["
	for bee, status := range status.EmployeeBees {
		colonyLogStatus += fmt.Sprint(bee, ": ", status.Status, ", ")
	}
	colonyLogStatus += "], onl:["
	for bee, status := range status.OnlookerBees {
		colonyLogStatus += fmt.Sprint(bee, ": ", status.Status, ", ")
	}
	colonyLogStatus += "], fs:["
	for fs, status := range status.FoodSources {
		colonyLogStatus += fmt.Sprint(fs, ": {occupied:", status.OccupiedBy, ", reserved:", status.ReservedBy, "}, ")
	}
	colonyLogStatus += "]"
	return colonyLogStatus
}

func colonyDataToString(status abcoptimizerv1.ColonyStatus) string {
	colonyLogStatus := ""
	colonyLogStatus += fmt.Sprint("empCycle: ", status.EmployeeBeeCycles, ", empCycStatus: ", status.EmployeeBeeCycleStatus, " onlCycle: ", status.OnlookerBeeCycles, ", onlCycStatus: ", status.OnlookerBeeCycleStatus)
	for bee, status := range status.EmployeeBees {
		colonyLogStatus += fmt.Sprint(bee, ": ", status.Status, ", ")
	}
	colonyLogStatus += "], onl:["
	for bee, status := range status.OnlookerBees {
		colonyLogStatus += fmt.Sprint(bee, ": ", status.Status, ", ")
	}
	colonyLogStatus += "], fs:["
	for fs, status := range status.FoodSources {
		colonyLogStatus += fmt.Sprint(fs, ": {occupied:", status.OccupiedBy, ", reserved:", status.ReservedBy, "}, ")
	}
	colonyLogStatus += "]"
	return colonyLogStatus
}

// {0 InProgress map[employee-bee-5877686888-ct6xf:{Done 0 12.280087870595414 [0.016171038 0.22691888 0.51726204] 0 true} employee-bee-5877686888-pxgvp:{Running 2  [0.5725438 0.88774645 0.8662279] 0 false} employee-bee-5877686888-znl7w:{Running 1  [0.7184246 0.2501613 0.678774] 0 false} onlooker-bee-6fc6fdbc44-7lbqg:{   [] 0 false} onlooker-bee-6fc6fdbc44-dvd4b:{   [] 0 false} onlooker-bee-6fc6fdbc44-nzv4t:{   [] 0 false}] 0 Started map[onlooker-bee-6fc6fdbc44-7lbqg:{New   [] 0 false} onlooker-bee-6fc6fdbc44-dvd4b:{New   [] 0 false} onlooker-bee-6fc6fdbc44-nzv4t:{New   [] 0 false}] map[] map[:{[] 0   } 0:{[0.016171038 0.22691888 0.51726204] 0 employee-bee-5877686888-ct6xf  } 1:{[0.7184246 0.2501613 0.678774] 0 employee-bee-5877686888-znl7w  } 2:{[0.5725438 0.88774645 0.8662279] 0 employee-bee-5877686888-pxgvp  }] [] []}	{"reconciler group": "abc-optimizer.pesu.edu", "reconciler kind": "Colony", "name": "colony-sample", "namespace": "default"}
// cycle: 0, cycStatus: InProgress, emp:[beename: done, ...], onl:[beename: done, ...], fs:[0: {occ:beename,res:beename}, 1:{occ:beename,res:beename}, 2:{occ:beename,res:beename}]

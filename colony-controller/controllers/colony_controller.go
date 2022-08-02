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

const (
	employeeBeeName          = "employee-bee"
	employeeBeeContainerName = "employee-bee-container"
	onlookerBeeName          = "onlooker-bee"
	onlookerBeeContainerName = "onlooker-bee-container"
	foodsourceName           = "foodsource"
	foodsourceContainerName  = "foodsource-container"
)

// ColonyReconciler reconciles a Colony object
type ColonyReconciler struct {
	client.Client
	Log logr.Logger
	// Log      *zap.Logger
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
	instance := &abcoptimizerv1.Colony{}
	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("object already deleted")
			return ctrl.Result{}, nil
		}
	} else {
		reqLogger.Info("Namespace: " + req.Namespace + "Name: " + req.Name)
	}

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

		return ctrl.Result{}, nil
	}

	// Foodsource

	if result, err := r.foodsourceController(ctx, reqLogger, instance); err != nil {
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

	// reqLogger.Info("updating Colony resource status for Food Source")
	// instance.Status.AvailableReplicas = foodSourceDeployment.Status.AvailableReplicas
	// if r.Client.Status().Update(ctx, instance); err != nil {
	// 	reqLogger.Error(err, "failed to update Colony status for Food Source")
	// 	return ctrl.Result{}, err
	// }

	// instance.Status.Cycles += 1

	reqLogger.Info("resource status synced")

	return ctrl.Result{}, nil
}

func (r *ColonyReconciler) foodsourceController(ctx context.Context, reqLogger logr.Logger, instance *abcoptimizerv1.Colony) (ctrl.Result, error) {
	reqLogger.Info("checking if an existing Food Source Deployment exists for this Colony")
	foodSourceDeployment := apps.Deployment{}
	err := r.Client.Get(ctx, client.ObjectKey{Namespace: instance.Namespace, Name: foodsourceName}, &foodSourceDeployment)
	if errors.IsNotFound(err) {
		reqLogger.Info("could not find existing Food Source Deployment for Colony, creating one...")

		foodSourceDeployment = *buildFoodSourceDeployment(*instance)
		if err := r.Client.Create(ctx, &foodSourceDeployment); err != nil {
			reqLogger.Error(err, "failed to create Food Source Deployment resource")
			return ctrl.Result{}, err
		}

		r.Recorder.Eventf(instance, core.EventTypeNormal, "Created", "Created Food Source deployment %q", foodSourceDeployment.Name)
		reqLogger.Info("created Food Source Deployment resource for Colony")
		return ctrl.Result{}, nil
	}
	if err != nil {
		reqLogger.Error(err, "failed to get Food Source Deployment for Colony resource")
		return ctrl.Result{}, err
	}

	reqLogger.Info("existing Food Source Deployment resource already exists for Colony, checking replica count")

	expectedFoodSource := int32(1)

	if *foodSourceDeployment.Spec.Replicas != expectedFoodSource {
		reqLogger.Info("updating replica count", "old_count", *foodSourceDeployment.Spec.Replicas, "new_count", expectedFoodSource)

		foodSourceDeployment.Spec.Replicas = &expectedFoodSource
		if err := r.Client.Update(ctx, &foodSourceDeployment); err != nil {
			reqLogger.Error(err, "failed to Food Source Deployment update replica count")
			return ctrl.Result{}, err
		}

		r.Recorder.Eventf(instance, core.EventTypeNormal, "Scaled", "Scaled Food Source deployment %q to %d replicas", foodSourceDeployment.Name, expectedFoodSource)

		return ctrl.Result{}, nil
	}

	reqLogger.Info("replica count up to date", "replica_count", *foodSourceDeployment.Spec.Replicas)
	return ctrl.Result{}, nil
}

func (r *ColonyReconciler) employeeBeeController(ctx context.Context, reqLogger logr.Logger, instance *abcoptimizerv1.Colony) (ctrl.Result, error) {
	reqLogger.Info("checking if an existing Employee Bee Deployment exists for this Colony")
	employeeBeeDeployment := apps.Deployment{}
	err := r.Client.Get(ctx, client.ObjectKey{Namespace: instance.Namespace, Name: employeeBeeName}, &employeeBeeDeployment)
	if errors.IsNotFound(err) {
		reqLogger.Info("could not find existing Employee Bee Deployment for Colony, creating one...")

		employeeBeeDeployment = *buildEmployeeBeeDeployment(*instance)
		if err := r.Client.Create(ctx, &employeeBeeDeployment); err != nil {
			reqLogger.Error(err, "failed to create Employee Bee Deployment resource")
			return ctrl.Result{}, err
		}

		r.Recorder.Eventf(instance, core.EventTypeNormal, "Created", "Created Employee Bee deployment %q", employeeBeeDeployment.Name)
		reqLogger.Info("created Employee Bee Deployment resource for Colony")
		return ctrl.Result{}, nil
	}
	if err != nil {
		reqLogger.Error(err, "failed to get Employee Bee Deployment for Colony resource")
		return ctrl.Result{}, err
	}

	reqLogger.Info("existing Employee Bee Deployment resource already exists forColony, checking replica count")

	expectedEmployeeColonys := instance.Spec.FoodSourceNumber

	if *employeeBeeDeployment.Spec.Replicas != expectedEmployeeColonys {
		reqLogger.Info("updating replica count", "old_count", *employeeBeeDeployment.Spec.Replicas, "new_count", expectedEmployeeColonys)

		employeeBeeDeployment.Spec.Replicas = &expectedEmployeeColonys
		if err := r.Client.Update(ctx, &employeeBeeDeployment); err != nil {
			reqLogger.Error(err, "failed to update Employee Bee Deployment replica count")
			return ctrl.Result{}, err
		}

		r.Recorder.Eventf(instance, core.EventTypeNormal, "Scaled", "Scaled Employee Bee deployment %q to %d replicas", employeeBeeDeployment.Name, expectedEmployeeColonys)

		return ctrl.Result{}, nil
	}

	reqLogger.Info("replica count up to date", "replica_count", *employeeBeeDeployment.Spec.Replicas)

	employeeBeeStatus := instance.Status.EmployeeBeeCycleStatus
	reqLogger.Info(fmt.Sprint(employeeBeeStatus))

	emp_done_count := 0

	for pod, value := range employeeBeeStatus {
		employeeBeePod := core.Pod{}
		err = r.Client.Get(ctx, client.ObjectKey{Namespace: instance.Namespace, Name: pod}, &employeeBeePod)
		if errors.IsNotFound(err) {
			reqLogger.Info("could not find existing Employee Bee Pod for Colony")
		}
		if err != nil {
			reqLogger.Info("failed to get Employee Bee Pod for Colony resource")
			continue
		}
		if employeeBeePod.Status.Phase == "Running" && value == "Done" {
			emp_done_count += 1
		}
	}

	if emp_done_count >= int(instance.Spec.FoodSourceNumber) {
		reqLogger.Info("Re-Initializing Employees in the Colony")
		// for pod, value := range employeeBeeStatus {
		// 	employeeBeePod := core.Pod{}
		// 	err = r.Client.Get(ctx, client.ObjectKey{Namespace: instance.Namespace, Name: pod}, &employeeBeePod)
		// 	if errors.IsNotFound(err) {
		// 		reqLogger.Info("could not find existing Employee Bee Pod for Colony")
		// 		delete(instance.Status.EmployeeBeeCycleStatus, pod)
		// 	}
		// 	if err != nil {
		// 		reqLogger.Info("failed to get Employee Bee Pod for Colony resource")
		// 		delete(instance.Status.EmployeeBeeCycleStatus, pod)
		// 		continue
		// 	}
		// 	if employeeBeePod.Status.Phase == "Running" && value == "Done" {
		// 		// instance.Status.EmployeeBeeCycleStatus = map[string]string{pod: "Terminating"}
		// 		delete(instance.Status.EmployeeBeeCycleStatus, pod)
		// 	}
		// }

		instance.Status.EmployeeBeeCycleStatus = map[string]string{}
		instance.Status.EmployeeBeeCycles += 1

		for i, foodsource := range instance.Status.FoodSources {
			foodsource.EmployeeBee = ""
			instance.Status.FoodSources[string(i)] = foodsource
		}

		// patch_instance := &abcoptimizerv1.Colony{}
		// patch_instance.Status.EmployeeBeeCycleStatus = map[string]string{}
		// patch_instance.Status.EmployeeBeeCycles = instance.Status.EmployeeBeeCycles + 1

		reqLogger.Info("Before Update:" + fmt.Sprint(instance.Status))
		// patch := client.MergeFrom(instance.DeepCopy())

		if err := r.Client.Status().Update(ctx, instance); err != nil {
			reqLogger.Error(err, "failed to update Employee Bee Deployment resource")
			return ctrl.Result{}, err
		}

		// r.Client.Status().Update(ctx, instance)
		reqLogger.Info("279: Deleting employee deploymnet")
		if err := r.Client.Delete(ctx, &employeeBeeDeployment, &client.DeleteOptions{}); err != nil {
			reqLogger.Error(err, "failed to delete Employee Bee Deployment resource")
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *ColonyReconciler) onlookerBeeController(ctx context.Context, reqLogger logr.Logger, instance *abcoptimizerv1.Colony) (ctrl.Result, error) {
	reqLogger.Info("checking if an existing Onlooker Bee Deployment exists for this Colony")
	onlookerBeeDeployment := apps.Deployment{}
	err := r.Client.Get(ctx, client.ObjectKey{Namespace: instance.Namespace, Name: onlookerBeeName}, &onlookerBeeDeployment)
	if errors.IsNotFound(err) {
		reqLogger.Info("could not find existing Onlooker Bee Deployment for Colony, creating one...")

		onlookerBeeDeployment = *buildOnlookerBeeDeployment(*instance)
		if err := r.Client.Create(ctx, &onlookerBeeDeployment); err != nil {
			reqLogger.Error(err, "failed to create Onlooker Bee Deployment resource")
			return ctrl.Result{}, err
		}

		r.Recorder.Eventf(instance, core.EventTypeNormal, "Created", "Created Onlooker Bee deployment %q", onlookerBeeDeployment.Name)
		reqLogger.Info("created Onlooker Bee Deployment resource for Colony")
		return ctrl.Result{}, nil
	}
	if err != nil {
		reqLogger.Error(err, "failed to get Onlooker Bee Deployment for Colony resource")
		return ctrl.Result{}, err
	}

	reqLogger.Info("existing Onlooker Bee Deployment resource already exists forColony, checking replica count")

	expectedOnlookerColonys := instance.Spec.FoodSourceNumber

	if *onlookerBeeDeployment.Spec.Replicas != expectedOnlookerColonys {
		reqLogger.Info("updating replica count", "old_count", *onlookerBeeDeployment.Spec.Replicas, "new_count", expectedOnlookerColonys)

		onlookerBeeDeployment.Spec.Replicas = &expectedOnlookerColonys
		if err := r.Client.Update(ctx, &onlookerBeeDeployment); err != nil {
			reqLogger.Error(err, "failed to update Onlooker Bee Deployment replica count")
			return ctrl.Result{}, err
		}

		r.Recorder.Eventf(instance, core.EventTypeNormal, "Scaled", "Scaled Onlooker Bee deployment %q to %d replicas", onlookerBeeDeployment.Name, expectedOnlookerColonys)

		return ctrl.Result{}, nil
	}

	reqLogger.Info("replica count up to date", "replica_count", *onlookerBeeDeployment.Spec.Replicas)

	onlookerBeeStatus := instance.Status.OnlookerBeeCycleStatus
	reqLogger.Info(fmt.Sprint(onlookerBeeStatus))

	emp_done_count := 0

	for pod, value := range onlookerBeeStatus {
		onlookerBeePod := core.Pod{}
		err = r.Client.Get(ctx, client.ObjectKey{Namespace: instance.Namespace, Name: pod}, &onlookerBeePod)
		if errors.IsNotFound(err) {
			reqLogger.Info("could not find existing Onlooker Bee Pod for Colony")
		}
		if err != nil {
			reqLogger.Info("failed to get Onlooker Bee Pod for Colony resource")
			continue
		}
		if onlookerBeePod.Status.Phase == "Running" && value == "Done" {
			emp_done_count += 1
		}
	}

	if emp_done_count >= int(instance.Spec.FoodSourceNumber) {
		reqLogger.Info("Re-Initializing Onlookers in the Colony")
		// for pod, value := range onlookerBeeStatus {
		// 	onlookerBeePod := core.Pod{}
		// 	err = r.Client.Get(ctx, client.ObjectKey{Namespace: instance.Namespace, Name: pod}, &onlookerBeePod)
		// 	if errors.IsNotFound(err) {
		// 		reqLogger.Info("could not find existing Onlooker Bee Pod for Colony")
		// 		delete(instance.Status.OnlookerBeeCycleStatus, pod)
		// 	}
		// 	if err != nil {
		// 		reqLogger.Info("failed to get Onlooker Bee Pod for Colony resource")
		// 		delete(instance.Status.OnlookerBeeCycleStatus, pod)
		// 		continue
		// 	}
		// 	if onlookerBeePod.Status.Phase == "Running" && value == "Done" {
		// 		// instance.Status.OnlookerBeeCycleStatus = map[string]string{pod: "Terminating"}
		// 		delete(instance.Status.OnlookerBeeCycleStatus, pod)
		// 	}
		// }
		instance.Status.OnlookerBeeCycleStatus = map[string]string{}
		instance.Status.OnlookerBeeCycles += 1

		for i, foodsource := range instance.Status.FoodSources {
			foodsource.OnlookerBee = ""
			instance.Status.FoodSources[string(i)] = foodsource
		}

		// patch_instance := &abcoptimizerv1.Colony{}
		// patch_instance.Status.OnlookerBeeCycleStatus = map[string]string{}
		// patch_instance.Status.OnlookerBeeCycles = instance.Status.OnlookerBeeCycles + 1

		reqLogger.Info("Before Update:" + fmt.Sprint(instance.Status))
		// patch := client.MergeFrom(instance.DeepCopy())

		if err := r.Client.Status().Update(ctx, instance); err != nil {
			reqLogger.Error(err, "failed to update Onlooker Bee Deployment resource")
			return ctrl.Result{}, err
		}

		// patch := client.MergeFrom(instance.DeepCopy())
		// r.Status().Update(ctx, instance)
		reqLogger.Info("411: Deleting onlooker deploymnet")
		if err := r.Client.Delete(ctx, &onlookerBeeDeployment, &client.DeleteOptions{}); err != nil {
			reqLogger.Error(err, "failed to delete Onlooker Bee Deployment resource")
			return ctrl.Result{}, err
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

func buildEmployeeBeeDeployment(Colony abcoptimizerv1.Colony) *apps.Deployment {
	labels := map[string]string{
		"app":        Colony.Name,
		"controller": Colony.Name,
	}
	deployment := apps.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:            employeeBeeName,
			Namespace:       Colony.Namespace,
			OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(&Colony, abcoptimizerv1.GroupVersion.WithKind("Colony"))},
		},
		Spec: apps.DeploymentSpec{
			Replicas: &Colony.Spec.FoodSourceNumber,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: core.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: core.PodSpec{
					Volumes: []core.Volume{
						{
							Name: "log-volume",
							// VolumeSource: core.VolumeSource{
							// 	PersistentVolumeClaim: &core.PersistentVolumeClaimVolumeSource{
							// 		ClaimName: "colony-pvc",
							// 	},
							// },
							VolumeSource: core.VolumeSource{
								HostPath: &core.HostPathVolumeSource{
									Path: "/mycolony",
								},
							},
						},
					},
					Containers: []core.Container{
						{
							Name:  employeeBeeContainerName,
							Image: Colony.Spec.EmployeeBeeImage,
							// Args:  []string{"/bin/sh", "-c", "test -e /var/log && rm -rf /var/log/..?* /var/log/.[!.]* /var/log/*  && test -z \"$(ls -A /var/log)\" || exit 1"},
							Env: []core.EnvVar{
								{
									Name: "BEE_NAME",
									ValueFrom: &core.EnvVarSource{
										FieldRef: &core.ObjectFieldSelector{
											FieldPath: "metadata.name",
										},
									},
								},
								{
									Name: "BEE_NAMESPACE",
									ValueFrom: &core.EnvVarSource{
										FieldRef: &core.ObjectFieldSelector{
											FieldPath: "metadata.namespace",
										},
									},
								},
							},
							VolumeMounts: []core.VolumeMount{
								{
									Name:      "log-volume",
									MountPath: "/var/log/mycolony",
								},
							},
						},
					},
				},
			},
		},
	}
	return &deployment
}

func buildOnlookerBeeDeployment(Colony abcoptimizerv1.Colony) *apps.Deployment {
	labels := map[string]string{
		"app":        Colony.Name,
		"controller": Colony.Name,
	}
	deployment := apps.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:            onlookerBeeName,
			Namespace:       Colony.Namespace,
			OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(&Colony, abcoptimizerv1.GroupVersion.WithKind("Colony"))},
		},
		Spec: apps.DeploymentSpec{
			Replicas: &Colony.Spec.FoodSourceNumber,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: core.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: core.PodSpec{
					Volumes: []core.Volume{
						{
							Name: "log-volume",
							// VolumeSource: core.VolumeSource{
							// 	PersistentVolumeClaim: &core.PersistentVolumeClaimVolumeSource{
							// 		ClaimName: "colony-pvc",
							// 	},
							// },
							VolumeSource: core.VolumeSource{
								HostPath: &core.HostPathVolumeSource{
									Path: "/mycolony",
								},
							},
						},
					},
					Containers: []core.Container{
						{
							Name:  onlookerBeeContainerName,
							Image: Colony.Spec.OnlookerBeeImage,
							Env: []core.EnvVar{
								{
									Name: "BEE_NAME",
									ValueFrom: &core.EnvVarSource{
										FieldRef: &core.ObjectFieldSelector{
											FieldPath: "metadata.name",
										},
									},
								},
								{
									Name: "BEE_NAMESPACE",
									ValueFrom: &core.EnvVarSource{
										FieldRef: &core.ObjectFieldSelector{
											FieldPath: "metadata.namespace",
										},
									},
								},
							},
							VolumeMounts: []core.VolumeMount{
								{
									Name:      "log-volume",
									MountPath: "/var/log/mycolony",
								},
							},
						},
					},
				},
			},
		},
	}
	return &deployment
}

func buildFoodSourceDeployment(Colony abcoptimizerv1.Colony) *apps.Deployment {
	labels := map[string]string{
		"app":        Colony.Name,
		"controller": Colony.Name,
	}
	replica_count := int32(1)
	deployment := apps.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:            foodsourceName,
			Namespace:       Colony.Namespace,
			OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(&Colony, abcoptimizerv1.GroupVersion.WithKind("Colony"))},
		},
		Spec: apps.DeploymentSpec{
			Replicas: &replica_count,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: core.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: core.PodSpec{
					Volumes: []core.Volume{
						{
							Name: "log-volume",
							// VolumeSource: core.VolumeSource{
							// 	PersistentVolumeClaim: &core.PersistentVolumeClaimVolumeSource{
							// 		ClaimName: "colony-pvc",
							// 	},
							// },
							VolumeSource: core.VolumeSource{
								HostPath: &core.HostPathVolumeSource{
									Path: "/mycolony",
								},
							},
						},
					},
					Containers: []core.Container{
						{
							Name:  foodsourceContainerName,
							Image: Colony.Spec.FoodSourceImage,
							Env: []core.EnvVar{
								{
									Name: "BEE_NAME",
									ValueFrom: &core.EnvVarSource{
										FieldRef: &core.ObjectFieldSelector{
											FieldPath: "metadata.name",
										},
									},
								},
								{
									Name: "BEE_NAMESPACE",
									ValueFrom: &core.EnvVarSource{
										FieldRef: &core.ObjectFieldSelector{
											FieldPath: "metadata.namespace",
										},
									},
								},
							},
							VolumeMounts: []core.VolumeMount{
								{
									Name:      "log-volume",
									MountPath: "/var/log/mycolony",
								},
							},
						},
					},
				},
			},
		},
	}
	return &deployment
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

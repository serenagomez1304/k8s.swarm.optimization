package controllers

import (
	abcoptimizerv1 "abc-optimizer/api/v1"
	"context"
	"fmt"
	"math/rand"
	"strconv"

	"github.com/go-logr/logr"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	foodsourceName          = "foodsource"
	foodsourceContainerName = "foodsource-container"
)

// type FoodSourceDeployment struct {
// 	Metatdata metav1.ObjectMeta
// 	Spec      apps.DeploymentSpec
// }

func (r *ColonyReconciler) foodsourceController(ctx context.Context, reqLogger logr.Logger, instance *abcoptimizerv1.Colony, req ctrl.Request) (ctrl.Result, error) {
	reqLogger.Info("foodsourceController: checking if an existing Food Source Deployment exists for this Colony")

	// 1. Initialize foodsource status in colony
	if len(instance.Status.FoodSources) == 0 {
		_, err := r.initFoodsource(ctx, reqLogger, instance)
		if err != nil {
			reqLogger.Error(err, "foodsourceController: foodsource initialization failed")
		}
	}

	// 2. check existence and update foodsource deployment in colony
	foodSourceDeployment := apps.Deployment{}
	err := r.Client.Get(ctx, client.ObjectKey{Namespace: instance.Namespace, Name: foodsourceName}, &foodSourceDeployment)
	if errors.IsNotFound(err) {
		reqLogger.Info("foodsourceController: could not find existing Food Source Deployment for Colony, creating one...")

		foodSourceDeployment = *buildFoodSourceDeployment(*instance)
		if err := r.Client.Create(ctx, &foodSourceDeployment); err != nil {
			reqLogger.Error(err, "foodsourceController: failed to create Food Source Deployment resource")
			return ctrl.Result{}, err
		}

		r.Recorder.Eventf(instance, core.EventTypeNormal, "Created", "Created Food Source deployment %q", foodSourceDeployment.Name)
		reqLogger.Info("foodsourceController: created Food Source Deployment resource for Colony")
	}
	// if err != nil {
	// 	reqLogger.Error(err, "failed to get Food Source Deployment for Colony resource")
	// 	return ctrl.Result{}, err
	// }

	reqLogger.Info("foodsourceController: existing Food Source Deployment resource already exists for Colony, checking replica count")

	if instance.Status.EmployeeBeeCycleStatus == "Started" || instance.Status.OnlookerBeeCycleStatus == "Started" {
		result, err := generateProbabilityMap(instance, reqLogger)
		if err != nil {
			reqLogger.Info("foodsourceController: error in generating probabilty map")
			return result, err
		}
	}

	// 3. reconcile in progress employee bee status with foodsource status
	if instance.Status.EmployeeBeeCycleStatus == "InProgress" {
		reconcileWithEmployeeInProgress(instance, reqLogger)
	}

	// 4. reconcile completed employee bee status data with foodsource status data
	if instance.Status.EmployeeBeeCycleStatus == "Completed" {
		reconcileWithEmployeeCompleted(instance, reqLogger)
	}

	// 5. reconcile in progress onlooker bee status with foodsource status
	if instance.Status.OnlookerBeeCycleStatus == "InProgress" {
		reconcileWithOnlookerInProgress(instance, reqLogger)
	}

	// 6. reconcile completed onlooker bee status data with foodsource status data
	if instance.Status.OnlookerBeeCycleStatus == "Completed" {
		reconcileWithOnlookerCompleted(instance, reqLogger)
	}

	reqLogger.Info("Foodsource instance status: " + fsLogString(instance.Status.FoodSources))

	// 7. Save potential solution by comparing with max trial count in colony
	_, err = r.saveSolution(ctx, instance, req, reqLogger)
	if err != nil {
		reqLogger.Error(err, "saveSolution failed")
	}
	return ctrl.Result{}, nil
}

func (r *ColonyReconciler) initFoodsource(ctx context.Context, reqLogger logr.Logger, instance *abcoptimizerv1.Colony) (ctrl.Result, error) {
	reqLogger.Info("initFoodsource: start foodsource initialization")
	// 1. Check length of upper and lower value vectors
	if len(instance.Spec.UpperValues) != int(instance.Spec.FoodsourceVectorLength) || len(instance.Spec.LowerValues) != int(instance.Spec.FoodsourceVectorLength) {
		return ctrl.Result{}, fmt.Errorf("initFoodsource: invalid colony spec: length of upper or lower vector")
	}

	// 2. Use upper and lower to generate random foodsource vector
	// foodsourceStatus := new(map[string]abcoptimizerv1.FoodsourceStatus)
	reqLogger.Info("initFoodsource: generate random foodsources")
	foodsourceStatus := map[string]abcoptimizerv1.FoodsourceStatus{}
	for id := 0; id < int(instance.Spec.FoodSourceNumber); id++ {
		fs_vector, err := newFsVector(instance, reqLogger)
		if err != nil {
			return ctrl.Result{}, err
		}
		newFoodsource := new(abcoptimizerv1.FoodsourceStatus)
		newFoodsource.Foodsource = fs_vector
		newFoodsource.TrialCount = 0
		newFoodsource.OccupiedBy = ""
		newFoodsource.ReservedBy = ""
		newFoodsource.ObjectiveFunction = ""
		// *foodsourceStatus = map[string]abcoptimizerv1.FoodsourceStatus{fmt.Sprint(id): *newFoodsource}
		foodsourceStatus[fmt.Sprint(id)] = *newFoodsource
	}
	instance.Status.FoodSources = foodsourceStatus

	// 3. Saved foodsources = []
	reqLogger.Info("initFoodsource: init saved foodsources")
	instance.Status.SavedFoodSources = []abcoptimizerv1.FoodsourceStatus{}

	reqLogger.Info("initFoodsource: exit foodsource initialization")
	return ctrl.Result{}, nil
}

func generateProbabilityMap(instance *abcoptimizerv1.Colony, reqLogger logr.Logger) (ctrl.Result, error) {
	reqLogger.Info("generateProbabilityMap: enter")
	maxFit := float32(0)
	fitMap := map[string]float32{}
	for id, value := range instance.Status.FoodSources {
		reqLogger.Info("generateProbabilityMap: value: " + fmt.Sprint(value))
		if value.ObjectiveFunction == "" {
			continue
		}
		objFunc, err := strconv.ParseFloat(value.ObjectiveFunction, 32)
		if err != nil {
			reqLogger.Info("generateProbabilityMap: could not convert objective function value to float")
			return ctrl.Result{}, err
		}
		fit := evaluateFitness(float32(objFunc))
		fitMap[id] = fit
		if fit > maxFit {
			maxFit = fit
		}
	}

	if len(fitMap) == 0 {
		reqLogger.Info("generateProbabilityMap: exit, skip")
		return ctrl.Result{}, nil
	}

	// commenting - trying to create object in heap, failed
	// creating in stack
	// probabilityMap := new(map[string]string)
	probabilityMap := map[string]string{}
	for id := range instance.Status.FoodSources {
		if maxFit == 0 {
			probabilityMap[id] = fmt.Sprint(0.0)
		} else {
			probability := (0.9 * (fitMap[id] / maxFit)) + 0.1
			probabilityMap[id] = fmt.Sprint(probability)
		}
	}

	instance.Status.ProbabilityMap = probabilityMap
	reqLogger.V(4).Info("generateProbabilityMap: probability map: " + fmt.Sprint(instance.Status.ProbabilityMap))
	reqLogger.Info("generateProbabilityMap: exit")
	return ctrl.Result{}, nil
}

func reconcileWithEmployeeInProgress(instance *abcoptimizerv1.Colony, reqLogger logr.Logger) (ctrl.Result, error) {
	reqLogger.Info("reconcileWithEmployeeInProgress: enter")
	for bee, value := range instance.Status.EmployeeBees {
		newFoodsource := new(abcoptimizerv1.FoodsourceStatus)
		newFoodsource.Foodsource = instance.Status.FoodSources[value.FoodsourceId].Foodsource
		newFoodsource.TrialCount = instance.Status.FoodSources[value.FoodsourceId].TrialCount
		newFoodsource.ObjectiveFunction = instance.Status.FoodSources[value.FoodsourceId].ObjectiveFunction
		newFoodsource.OccupiedBy = instance.Status.FoodSources[value.FoodsourceId].OccupiedBy
		newFoodsource.ReservedBy = instance.Status.FoodSources[value.FoodsourceId].ReservedBy

		switch value.Status {
		case "New":
			// dont copy to foodsource
			reqLogger.Info("reconcileWithEmployeeInProgress: new, do nothing")
		case "Waiting":
			// occupied by onlooker, reserve
			reqLogger.Info("reconcileWithEmployeeInProgress: waiting for onlooker, " + bee + " reserving " + fmt.Sprint(value.FoodsourceId))
			newFoodsource.ReservedBy = bee
		case "Running":
			// occupy
			reqLogger.Info("reconcileWithEmployeeInProgress: running, " + bee + " occupying " + fmt.Sprint(value.FoodsourceId))
			newFoodsource.OccupiedBy = bee
			if newFoodsource.ReservedBy == bee {
				newFoodsource.ReservedBy = ""
			}
		case "Done":
			// vacate
			reqLogger.Info("reconcileWithEmployeeInProgress: done, vacate " + bee + " fs " + fmt.Sprint(value.FoodsourceId))
			newFoodsource.Foodsource = value.FoodsourceVector
			newFoodsource.ObjectiveFunction = value.ObjectiveFunction
			newFoodsource.TrialCount = value.FoodsourceTrialCount
			if newFoodsource.OccupiedBy == bee {
				newFoodsource.OccupiedBy = ""
			}
		}

		reqLogger.Info("reconcileWithEmployeeInProgress: update colony status")
		instance.Status.FoodSources[value.FoodsourceId] = *newFoodsource
		reqLogger.V(4).Info("reconcileWithEmployeeInProgress: fs status: " + fmt.Sprint(instance.Status.FoodSources))
	}
	reqLogger.Info("reconcileWithEmployeeInProgress: exit")
	return ctrl.Result{}, nil
}

func compareFitness(beeObj string, fsObj string, reqLogger logr.Logger) (bool, error) {
	if len(beeObj) == 0 {
		beeObj = "0.0"
	}
	if len(fsObj) == 0 {
		fsObj = "0.0"
	}
	beeObjVal, err := strconv.ParseFloat(beeObj, 32)
	if err != nil {
		return false, fmt.Errorf("compareFitness: cannot convert beeObj value to float")
	}
	fsObjVal, err := strconv.ParseFloat(fsObj, 32)
	if err != nil {
		return false, fmt.Errorf("compareFitness: cannot convert fsObj value to float")
	}
	beeFit := evaluateFitness(float32(beeObjVal))
	fsFit := evaluateFitness(float32(fsObjVal))
	if beeFit > fsFit {
		return true, nil
	} else {
		return false, nil
	}
}

func reconcileWithEmployeeCompleted(instance *abcoptimizerv1.Colony, reqLogger logr.Logger) (ctrl.Result, error) {
	reqLogger.Info("reconcileWithEmployeeCompleted: enter")
	for bee, value := range instance.Status.EmployeeBees {
		newFoodsource := new(abcoptimizerv1.FoodsourceStatus)
		newFoodsource.Foodsource = value.FoodsourceVector
		newFoodsource.TrialCount = value.FoodsourceTrialCount
		newFoodsource.OccupiedBy = ""
		newFoodsource.ReservedBy = instance.Status.FoodSources[value.FoodsourceId].ReservedBy
		newFoodsource.ObjectiveFunction = value.ObjectiveFunction
		instance.Status.FoodSources[value.FoodsourceId] = *newFoodsource

		instance.Status.DeadBees = append(instance.Status.DeadBees, bee)
	}
	instance.Status.EmployeeBeeCycleStatus = ""
	instance.Status.EmployeeBees = map[string]abcoptimizerv1.BeeStatus{}
	reqLogger.Info("reconcileWithEmployeeCompleted: exit")
	return ctrl.Result{}, nil
}

func reconcileWithOnlookerInProgress(instance *abcoptimizerv1.Colony, reqLogger logr.Logger) (ctrl.Result, error) {
	reqLogger.Info("reconcileWithOnlookerInProgress: enter")
	for bee, value := range instance.Status.OnlookerBees {
		newFoodsource := new(abcoptimizerv1.FoodsourceStatus)
		newFoodsource.Foodsource = instance.Status.FoodSources[value.FoodsourceId].Foodsource
		newFoodsource.TrialCount = instance.Status.FoodSources[value.FoodsourceId].TrialCount
		newFoodsource.ObjectiveFunction = instance.Status.FoodSources[value.FoodsourceId].ObjectiveFunction
		newFoodsource.OccupiedBy = instance.Status.FoodSources[value.FoodsourceId].OccupiedBy
		newFoodsource.ReservedBy = instance.Status.FoodSources[value.FoodsourceId].ReservedBy

		switch value.Status {
		case "New":
			// dont copy to foodsource
			reqLogger.Info("reconcileWithOnlookerInProgress: new, do nothing")
		case "Waiting":
			// occupied by employee, reserve
			reqLogger.Info("reconcileWithOnlookerInProgress: waiting for employee, " + bee + " reserving " + fmt.Sprint(value.FoodsourceId))
			newFoodsource.ReservedBy = bee
		case "Running":
			// occupy
			reqLogger.Info("reconcileWithOnlookerInProgress: running, " + bee + " occupying " + fmt.Sprint(value.FoodsourceId))
			newFoodsource.OccupiedBy = bee
			if newFoodsource.ReservedBy == bee {
				newFoodsource.ReservedBy = ""
			}

		case "Done":
			// TODO: if multiple onlooker bees work on same foodsoure, overwriting ???
			// vacate and copy to foodsource
			reqLogger.Info("reconcileWithOnlookerInProgress: done, vacate " + bee + " fs " + fmt.Sprint(value.FoodsourceId))
			newFoodsource.Foodsource = value.FoodsourceVector
			newFoodsource.ObjectiveFunction = value.ObjectiveFunction
			newFoodsource.TrialCount = value.FoodsourceTrialCount
			if newFoodsource.OccupiedBy == bee {
				newFoodsource.OccupiedBy = ""
			}
		}
		reqLogger.Info("reconcileWithOnlookerInProgress: update colony status")
		instance.Status.FoodSources[value.FoodsourceId] = *newFoodsource
		reqLogger.V(4).Info("reconcileWithOnlookerInProgress: fs status: " + fmt.Sprint(instance.Status.FoodSources))
	}
	reqLogger.Info("reconcileWithOnlookerInProgress: exit")
	return ctrl.Result{}, nil
}

func reconcileWithOnlookerCompleted(instance *abcoptimizerv1.Colony, reqLogger logr.Logger) (ctrl.Result, error) {
	reqLogger.Info("reconcileWithOnlookerCompleted: enter")
	for bee, value := range instance.Status.OnlookerBees {
		newFoodsource := new(abcoptimizerv1.FoodsourceStatus)
		newFoodsource.Foodsource = instance.Status.FoodSources[value.FoodsourceId].Foodsource
		newFoodsource.TrialCount = instance.Status.FoodSources[value.FoodsourceId].TrialCount
		newFoodsource.ObjectiveFunction = instance.Status.FoodSources[value.FoodsourceId].ObjectiveFunction
		newFoodsource.OccupiedBy = instance.Status.FoodSources[value.FoodsourceId].OccupiedBy
		newFoodsource.ReservedBy = instance.Status.FoodSources[value.FoodsourceId].ReservedBy

		good, err := compareFitness(value.ObjectiveFunction, instance.Status.FoodSources[value.FoodsourceId].ObjectiveFunction, reqLogger)
		if err != nil {
			reqLogger.Info("reconcileWithOnlookerCompleted: error in comparing fitness, skipping")
			reqLogger.Info(err.Error())
			if newFoodsource.OccupiedBy == bee {
				newFoodsource.OccupiedBy = ""
			}
			instance.Status.FoodSources[value.FoodsourceId] = *newFoodsource
			instance.Status.DeadBees = append(instance.Status.DeadBees, bee) // TODO: check later
			continue
		}
		// onlooker bee did not process this vector, hence retaining previous vector
		if good && len(value.FoodsourceVector) != 0 {
			newFoodsource.Foodsource = value.FoodsourceVector
			newFoodsource.TrialCount = value.FoodsourceTrialCount
			newFoodsource.ObjectiveFunction = value.ObjectiveFunction
		}
		if newFoodsource.OccupiedBy == bee {
			newFoodsource.OccupiedBy = ""
		}
		instance.Status.FoodSources[value.FoodsourceId] = *newFoodsource

		instance.Status.DeadBees = append(instance.Status.DeadBees, bee)
	}
	instance.Status.OnlookerBeeCycleStatus = ""
	instance.Status.OnlookerBees = map[string]abcoptimizerv1.BeeStatus{}
	reqLogger.Info("reconcileWithOnlookerCompleted: exit")
	return ctrl.Result{}, nil
}

func (r *ColonyReconciler) saveSolution(ctx context.Context, instance *abcoptimizerv1.Colony, req ctrl.Request, reqLogger logr.Logger) (ctrl.Result, error) {
	reqLogger.Info("saveSolution: enter")

	max_trial_count := instance.Spec.MaxTrialCount
	for id, foodsource := range instance.Status.FoodSources {
		if foodsource.TrialCount >= max_trial_count {
			// append foodsource to saved foodsource
			reqLogger.Info("saveSolution: saving foodsource, and reset")
			instance.Status.SavedFoodSources = append(instance.Status.SavedFoodSources, instance.Status.FoodSources[id])
			fs_vector, err := newFsVector(instance, reqLogger)
			if err != nil {
				return ctrl.Result{}, err
			}
			newFoodsource := new(abcoptimizerv1.FoodsourceStatus)
			newFoodsource.Foodsource = fs_vector
			newFoodsource.TrialCount = 0
			newFoodsource.OccupiedBy = ""
			newFoodsource.ReservedBy = ""
			newFoodsource.ObjectiveFunction = ""
			instance.Status.FoodSources[string(id)] = *newFoodsource
		}
	}
	reqLogger.Info("saveSolution: exit")
	return ctrl.Result{}, nil
}

func newFsVector(instance *abcoptimizerv1.Colony, reqLogger logr.Logger) ([]string, error) {
	fs_vector := make([]string, instance.Spec.FoodsourceVectorLength)
	for i := 0; i < int(instance.Spec.FoodsourceVectorLength); i++ {
		upper, err := strconv.ParseFloat(instance.Spec.UpperValues[i], 32)
		if err != nil {
			return nil, fmt.Errorf("newFsVector: cannot convert upper value to int")
		}
		lower, err := strconv.ParseFloat(instance.Spec.LowerValues[i], 32)
		if err != nil {
			return nil, fmt.Errorf("newFsVector: cannot convert lower value to int")
		}
		fs_vector[i] = fmt.Sprint(float32(rand.Float64() * ((upper - lower) + lower)))
	}
	return fs_vector, nil
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

func fsLogString(fsStatus map[string]abcoptimizerv1.FoodsourceStatus) string {
	fsLogStatus := ""
	for id, status := range fsStatus {
		fsLogStatus += fmt.Sprint(id, ": {len: ", len(status.Foodsource), ", occupied: ", status.OccupiedBy, ", reserved: ", status.ReservedBy, ", trialCount: ", status.TrialCount, "}, ")
	}
	return fsLogStatus
}

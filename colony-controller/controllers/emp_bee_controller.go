package controllers

import (
	abcoptimizerv1 "abc-optimizer/api/v1"
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/strings/slices"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	employeeBeeName          = "employee-bee"
	employeeBeeContainerName = "employee-bee-container"
)

func (r *ColonyReconciler) employeeBeeController(ctx context.Context, reqLogger logr.Logger, instance *abcoptimizerv1.Colony) (ctrl.Result, error) {

	reqLogger.V(4).Info("EBC:-------------------------------------- ------------------------------------------")
	reqLogger.V(4).Info("EPC: Employee Bee Controller")
	reqLogger.V(4).Info("EBC:-------------------------------------- ------------------------------------------")

	reqLogger.V(8).Info("EBC:checking if an existing Employee Bee Deployment exists for this Colony")

	// 1. check existence and update employee-bee deployment in colony

	employeeBeeDeployment := apps.Deployment{}
	result, err := r.deployEmployeeBees(ctx, &employeeBeeDeployment, instance, reqLogger)
	if err != nil {
		reqLogger.V(4).Info("EBC:employeeBeeController: error in employee bee deployment")
		return result, err
	}

	// 2. Get employee pod list
	empPodList, err := r.getEmpPodList(ctx, instance, reqLogger)
	if err != nil {
		reqLogger.V(4).Info("EBC:employeeBeeController: pods not found")
		return ctrl.Result{}, nil
	}

	// 3. Initialize employee bee status in colony
	if instance.Status.EmployeeBeeCycleStatus == "" && len(instance.Status.EmployeeBees) == 0 {
		initEmployeeBees(instance, empPodList, reqLogger)
	}

	// 4. register bee and generate new fs vector when cycle is new
	if instance.Status.EmployeeBeeCycleStatus == "Started" && len(instance.Status.FoodSources["0"].Foodsource) != 0 {
		result, err := r.registerAndAssign(ctx, instance, empPodList, reqLogger)
		if err != nil {
			reqLogger.V(4).Info("EBC:employeeBeeController: error in registering bee to foodsource")
			return result, err
		}
		reqLogger.V(5).Info("EBC:employeeBeeController: cycle-status: Started " + empStatusToString(instance.Status.EmployeeBees))

		// all foodsources need to be filled before entering here ?????
		result, err = generateNewEmpFsVector(instance, reqLogger)
		if err != nil {
			reqLogger.V(4).Info("EBC:employeeBeeController: error in generating new fs vector")
			return result, err
		}
		reqLogger.V(7).Info("EBC:employeeBeeController:  cycle-status: Started: " + empFoodSourceDataToString(instance.Status.EmployeeBees))
	}

	// 5. reassign
	if instance.Status.EmployeeBeeCycleStatus == "InProgress" {
		result, err := r.reassignEmpBeeStatus(ctx, instance, reqLogger)
		if err != nil {
			reqLogger.V(4).Info("EBC:employeeBeeController: error in reassigning bee to foodsource")
			return result, err
		}
		reqLogger.V(5).Info("EBC:employeeBeeController:  cycle-status: InProgress: " + empStatusToString(instance.Status.EmployeeBees))
	}

	// 6. update fs vector
	if instance.Status.EmployeeBeeCycleStatus == "Completed" {
		result, err := updateEmpFoodsources(instance, reqLogger)
		if err != nil {
			reqLogger.V(4).Info("EBC:employeeBeeController: error in updating fs vector")
			return result, err
		}
		reqLogger.V(5).Info("EBC:employeeBeeController:  cycle-status: Completed: " + empStatusToString(instance.Status.EmployeeBees))
	}

	// 7. end of employee cycle
	result, err = r.endOfEmpCycle(ctx, instance, &employeeBeeDeployment, reqLogger)
	if err != nil {
		return result, err
	}
	reqLogger.V(5).Info("EBC:employeeBeeController:  endOfEmpCycle: " + empStatusToString(instance.Status.EmployeeBees))

	reqLogger.V(4).Info("EBC:-------------------------------------- ------------------------------------------")
	return ctrl.Result{}, nil
}

func (r *ColonyReconciler) getEmpPodList(ctx context.Context, instance *abcoptimizerv1.Colony, reqLogger logr.Logger) ([]core.Pod, error) {
	employeeBeeList := &core.PodList{}
	err := r.Client.List(ctx, employeeBeeList, client.MatchingLabels{"bee": "employee"})
	if err != nil {
		return []core.Pod{}, err
	}
	reqLogger.V(10).Info("EBC:getEmpPodList: pod list: " + fmt.Sprint(employeeBeeList.Items))
	employeeList := employeeBeeList.Items
	returnList := []core.Pod{}
	for _, bee := range employeeList {
		if !slices.Contains(instance.Status.DeadBees, bee.GetName()) {
			returnList = append(returnList, bee)
		}
	}
	return returnList, nil
}

func initEmployeeBees(instance *abcoptimizerv1.Colony, employeeList []core.Pod, reqLogger logr.Logger) (ctrl.Result, error) {
	reqLogger.V(8).Info("EBC:initEmployeeBees: enter")
	// 1. if all bees have not been created, wait
	reqLogger.V(10).Info("EBC:initEmployeeBees: number of employee pods: " + fmt.Sprint(len(employeeList)))

	empBeeStatus := map[string]abcoptimizerv1.BeeStatus{}
	for _, bee := range employeeList {
		if slices.Contains(instance.Status.DeadBees, bee.GetName()) {
			continue
		}
		newEmpStatus := abcoptimizerv1.BeeStatus{}
		newEmpStatus.Status = "New"
		newEmpStatus.FoodsourceId = ""
		newEmpStatus.ObjectiveFunction = ""
		newEmpStatus.FoodsourceVector = []string{}
		newEmpStatus.FoodsourceTrialCount = 0
		newEmpStatus.ObjFuncStatus = false
		empBeeStatus[bee.GetName()] = newEmpStatus
	}

	if len(empBeeStatus) < int(instance.Spec.FoodSourceNumber) {
		reqLogger.V(4).Info("EBC:initEmployeeBees: exit with warning")
		return ctrl.Result{}, nil
	}

	// 2. init bee status
	instance.Status.EmployeeBees = empBeeStatus
	instance.Status.EmployeeBeeCycleStatus = "Started"

	reqLogger.V(5).Info("EBC:initEmployeeBees: employee bees: " + empStatusToString(instance.Status.EmployeeBees))
	reqLogger.V(8).Info("EBC:initEmployeeBees: exit")
	return ctrl.Result{}, nil
}

func (r *ColonyReconciler) registerAndAssign(ctx context.Context, instance *abcoptimizerv1.Colony, employeeList []core.Pod, reqLogger logr.Logger) (ctrl.Result, error) {
	reqLogger.V(8).Info("EBC:registerAndAssign: enter")
	// 1. if all bees have not been created, wait
	if len(employeeList) < int(instance.Spec.FoodSourceNumber) {
		return ctrl.Result{}, nil
	}

	// 2. register and assign foodsource to bee
	reqLogger.V(8).Info("EBC:registerAndAssign: register and assign foodsource to bee")
	instance.Status.EmployeeBeeCycleStatus = "InProgress"
	// something wrong here
	for _, bee := range employeeList {
		reqLogger.V(8).Info("EBC:registerAndAssign: iterating: " + bee.GetName())
		if instance.Status.EmployeeBees[bee.GetName()].Status == "New" {
			availableId, availableFoodsource, err := empFindAvailableFoodsource(instance, reqLogger)
			if err != nil {
				// TODO: vacate again

				return ctrl.Result{}, err
			}
			empStatus := abcoptimizerv1.BeeStatus{}
			if strings.Contains(availableFoodsource.OccupiedBy, "onlooker") {
				empStatus.Status = "Waiting"
			} else {
				empStatus.Status = "Running"
			}
			// empStatus.BeeName = bee.GetName()
			empStatus.FoodsourceId = availableId
			empStatus.FoodsourceVector = availableFoodsource.Foodsource
			empStatus.ObjectiveFunction = availableFoodsource.ObjectiveFunction
			empStatus.FoodsourceTrialCount = availableFoodsource.TrialCount
			empStatus.ObjFuncStatus = false
			instance.Status.EmployeeBees[bee.GetName()] = empStatus
			// reqLogger.V(7).Info("EBC:registerAndAssign: update colony status : " + fmt.Sprint(empStatus))
		}
	}
	reqLogger.V(5).Info("EBC:registerAndAssign: registering: " + empStatusToString(instance.Status.EmployeeBees))
	reqLogger.V(8).Info("EBC:registerAndAssign: exit")
	return ctrl.Result{}, nil
}

func (r *ColonyReconciler) reassignEmpBeeStatus(ctx context.Context, instance *abcoptimizerv1.Colony, reqLogger logr.Logger) (ctrl.Result, error) {
	reqLogger.V(8).Info("EBC:reassignEmpBeeStatus: enter")
	// rassign bee status
	for bee, value := range instance.Status.EmployeeBees {
		// if bee waiting, bee reserved fs, fs occupied by none
		foodsource := instance.Status.FoodSources[value.FoodsourceId]
		if value.Status == "Waiting" && foodsource.ReservedBy == bee && foodsource.OccupiedBy == "" {
			value.Status = "Running"
		} else {
			// if bee running, objective function value computed

			if value.Status == "Running" && value.ObjFuncStatus {
				value.Status = "Done"
			}
		}
		instance.Status.EmployeeBees[bee] = value
	}
	reqLogger.V(5).Info("EBC:reassignEmpBeeStatus: status: " + empStatusToString(instance.Status.EmployeeBees))
	reqLogger.V(8).Info("EBC:reassignEmpBeeStatus: exit")
	return ctrl.Result{}, nil
}

func empFindAvailableFoodsource(instance *abcoptimizerv1.Colony, reqLogger logr.Logger) (string, abcoptimizerv1.FoodsourceStatus, error) {
	reqLogger.V(8).Info("EBC:finaAvailableFoodsource: enter")
	foodsources := instance.Status.FoodSources
	empBeeStatus := instance.Status.EmployeeBees
	for id, value := range foodsources {
		if strings.Contains(foodsources[id].ReservedBy, "employee") || strings.Contains(foodsources[id].OccupiedBy, "employee") {
			continue
		} else {
			// skip if foodsource already assigned
			confirm := false
			for _, status := range empBeeStatus {
				if status.FoodsourceId == id {
					confirm = true
					break
				}
			}
			if confirm {
				continue
			}
			reqLogger.V(8).Info("EBC:finaAvailableFoodsource: exit")
			return id, value, nil
		}
	}
	reqLogger.V(8).Info("EBC:finaAvailableFoodsource: exit with error")
	return "", abcoptimizerv1.FoodsourceStatus{}, fmt.Errorf("all foodsources occupied/reserved by employee")
}

func (r *ColonyReconciler) endOfEmpCycle(ctx context.Context, instance *abcoptimizerv1.Colony, employeeBeeDeployment *apps.Deployment, reqLogger logr.Logger) (ctrl.Result, error) {
	reqLogger.V(8).Info("EBC:endOfEmpCycle: enter")
	employeeBees := instance.Status.EmployeeBees

	doneCount := 0

	for bee, value := range employeeBees {
		employeeBeePod := core.Pod{}
		err := r.Client.Get(ctx, client.ObjectKey{Namespace: instance.Namespace, Name: bee}, &employeeBeePod)
		if errors.IsNotFound(err) {
			reqLogger.V(4).Info("EBC:endOfEmpCycle: could not find existing Employee Bee Pod for Colony")
			value.Status = "Done"
			doneCount += 1
			continue
		}
		if err != nil {
			reqLogger.V(4).Info("EBC:endOfEmpCycle: failed to get Employee Bee Pod for Colony resource")
			value.Status = "Done"
			doneCount += 1
			continue
		}
		if employeeBeePod.Status.Phase == "Running" && value.Status == "Done" { // TODO: done??
			doneCount += 1
		}
	}

	if doneCount >= int(instance.Spec.FoodSourceNumber) {
		reqLogger.V(8).Info("EBC:endOfEmpCycle: Re-Initializing Employees in the Colony")

		instance.Status.EmployeeBeeCycles += 1

		reqLogger.V(8).Info("EBC:endOfEmpCycle: Attempting to delete employee-bee deployment")
		employeeBeeDeployment := apps.Deployment{}
		err := r.Client.Get(ctx, client.ObjectKey{Namespace: instance.Namespace, Name: employeeBeeName}, &employeeBeeDeployment)
		if errors.IsNotFound(err) {
			reqLogger.V(4).Info("EBC:endOfEmpCycle: could not find existing Employee Bee Deployment for Colony")
		} else {
			reqLogger.V(8).Info("EBC:endOfEmpCycle: Deleting employee deployment")
			if err := r.Client.Delete(ctx, &employeeBeeDeployment, &client.DeleteOptions{}); err != nil {
				reqLogger.Error(err, "EBC:failed to delete Employee Bee Deployment resource")
				return ctrl.Result{}, err
			}
		}

		// if err := r.Client.Delete(ctx, employeeBeeDeployment, &client.DeleteOptions{}); err != nil {
		// 	reqLogger.Error(err, "EBC:endOfEmpCycle: failed to delete Employee Bee Deployment resource")

		// 	return ctrl.Result{}, err
		// }
		instance.Status.EmployeeBeeCycleStatus = "Completed"
	}
	reqLogger.V(8).Info("EBC:endOfEmpCycle: exit")
	return ctrl.Result{}, nil
}

func checkFs(instance *abcoptimizerv1.Colony, reqLogger logr.Logger) bool {
	reqLogger.V(8).Info("EBC:checkFs: enter")
	for _, val := range instance.Status.FoodSources {
		if len(val.Foodsource) == 0 {
			reqLogger.V(8).Info("EBC:checkFs: exit with false")
			return false
		}
	}
	reqLogger.V(8).Info("EBC:checkFs: exit with true")
	return true
}

func generateNewEmpFsVector(instance *abcoptimizerv1.Colony, reqLogger logr.Logger) (ctrl.Result, error) {
	reqLogger.V(8).Info("EBC:generateNewEmpFsVector: enter")
	reqLogger.V(7).Info("EBC:generateNewEmpFsVector: instance status: " + empFoodSourceDataToString(instance.Status.EmployeeBees))
	if !checkFs(instance, reqLogger) {
		reqLogger.V(8).Info("EBC:generateNewEmpFsVector: foodsource vector is empty")
	}
	for bee, value := range instance.Status.EmployeeBees {
		id := value.FoodsourceId
		if id == "" {
			continue
		}
		currentVector := instance.Status.FoodSources[id].Foodsource
		reqLogger.V(8).Info("EBC:food source vector for " + id + " : " + fmt.Sprint(currentVector))
		partnerId := fmt.Sprint(rand.Intn(int(instance.Spec.FoodSourceNumber)))
		for partnerId == id {
			partnerId = fmt.Sprint(rand.Intn(int(instance.Spec.FoodSourceNumber)))
		}
		partnerVector := instance.Status.FoodSources[partnerId].Foodsource
		reqLogger.V(8).Info("EBC:partner vector for " + partnerId + " : " + fmt.Sprint(partnerVector))
		j := rand.Intn(int(instance.Spec.FoodsourceVectorLength))
		max := float32(1)
		min := float32(-1)
		phi := rand.Float32()*(max-min) + min
		newVector := make([]string, len(currentVector))
		copy(newVector, currentVector)
		curPosVal, err := strconv.ParseFloat(currentVector[j], 32)
		if err != nil {
			reqLogger.V(4).Info("EBC:generateNewEmpFsVector: " + err.Error())
			return ctrl.Result{}, err
		}
		partnerPosVal, err := strconv.ParseFloat(partnerVector[j], 32)
		if err != nil {
			reqLogger.V(4).Info("EBC:generateNewEmpFsVector: failed to convert partner vector to float")
			reqLogger.V(4).Info("EBC:generateNewEmpFsVector: " + err.Error())
			return ctrl.Result{}, err
		}
		newVector[j] = fmt.Sprint(float32(curPosVal) + phi*(float32(curPosVal)-float32(partnerPosVal)))
		employeeBeeStatus := new(abcoptimizerv1.BeeStatus)
		employeeBeeStatus.FoodsourceId = value.FoodsourceId
		employeeBeeStatus.FoodsourceTrialCount = value.FoodsourceTrialCount
		employeeBeeStatus.FoodsourceVector = newVector
		employeeBeeStatus.ObjFuncStatus = value.ObjFuncStatus
		employeeBeeStatus.ObjectiveFunction = value.ObjectiveFunction
		employeeBeeStatus.Status = value.Status
		instance.Status.EmployeeBees[bee] = *employeeBeeStatus
		// reqLogger.V(7).Info("EBC:generateNewEmpFsVector: Fs Vector " + id + ": " + fmt.Sprint(instance.Status.EmployeeBees[bee].FoodsourceVector))
	}
	reqLogger.V(7).Info("EBC:generateNewEmpFsVector: instance status: " + empFoodSourceDataToString(instance.Status.EmployeeBees))
	reqLogger.V(8).Info("EBC:generateNewEmpFsVector: exit")
	return ctrl.Result{}, nil
}

func evaluateFitness(obj_func_val float32) float32 {
	if obj_func_val >= 0 {
		return 1 / (1 + obj_func_val)
	} else {
		return 1 - obj_func_val
	}
}

func updateEmpFoodsources(instance *abcoptimizerv1.Colony, reqLogger logr.Logger) (ctrl.Result, error) {
	reqLogger.V(8).Info("EBC:updateEmpFoodsources: enter")
	for bee, value := range instance.Status.EmployeeBees {
		if !value.ObjFuncStatus {
			continue
		}
		newObjFunc, err := strconv.ParseFloat(value.ObjectiveFunction, 32)
		if err != nil {
			reqLogger.Error(err, "EBC:updateEmpFoodsources: cannot convert new obj func to float")
			return ctrl.Result{}, err
		}
		newFitness := evaluateFitness(float32(newObjFunc))
		curObjFunc, err := strconv.ParseFloat(instance.Status.FoodSources[value.FoodsourceId].ObjectiveFunction, 32)
		if err != nil {
			reqLogger.Error(err, "EBC:updateEmpFoodsources: cannot convert cur obj func to float")
			return ctrl.Result{}, err
		}
		curFitness := evaluateFitness(float32(curObjFunc))

		employeeBeeStatus := new(abcoptimizerv1.BeeStatus)

		if newFitness >= curFitness {
			employeeBeeStatus.FoodsourceTrialCount = 0
			employeeBeeStatus.ObjectiveFunction = fmt.Sprint(newObjFunc)
			reqLogger.V(8).Info("EBC:updateEmpFoodsources: updated to new the foodsource vector; ObjectiveFunction = " + fmt.Sprint(newObjFunc))
		} else {
			employeeBeeStatus.FoodsourceVector = instance.Status.FoodSources[value.FoodsourceId].Foodsource
			employeeBeeStatus.FoodsourceTrialCount = employeeBeeStatus.FoodsourceTrialCount + 1
			reqLogger.V(8).Info("EBC:updateEmpFoodsources: retain old foodsource vector, trialcount updated to " + fmt.Sprint(employeeBeeStatus.FoodsourceTrialCount))
		}
		instance.Status.EmployeeBees[bee] = *employeeBeeStatus
	}
	reqLogger.V(8).Info("EBC:updateEmpFoodsources: exit")
	return ctrl.Result{}, nil
}

func (r *ColonyReconciler) deployEmployeeBees(ctx context.Context, employeeBeeDeployment *apps.Deployment, instance *abcoptimizerv1.Colony, reqLogger logr.Logger) (ctrl.Result, error) {
	reqLogger.V(8).Info("EBC:deployEmployeeBees: enter")
	err := r.Client.Get(ctx, client.ObjectKey{Namespace: instance.Namespace, Name: employeeBeeName}, employeeBeeDeployment)
	if errors.IsNotFound(err) {
		reqLogger.V(8).Info("EBC:deployEmployeeBees: could not find existing Employee Bee Deployment for Colony, creating one...")

		employeeBeeDeployment = buildEmployeeBeeDeployment(*instance)
		if err := r.Client.Create(ctx, employeeBeeDeployment); err != nil {
			reqLogger.Error(err, "EBC:deployEmployeeBees: failed to create Employee Bee Deployment resource")
			reqLogger.V(8).Info("EBC:deployEmployeeBees: exit")
			return ctrl.Result{}, err
		}

		r.Recorder.Eventf(instance, core.EventTypeNormal, "Created", "Created Employee Bee deployment %q", employeeBeeDeployment.Name)
		reqLogger.V(8).Info("EBC:deployEmployeeBees: created Employee Bee Deployment resource for Colony")
		reqLogger.V(8).Info("EBC:deployEmployeeBees: exit")
		return ctrl.Result{}, nil
	}
	if err != nil {
		reqLogger.Error(err, "EBC:deployEmployeeBees: failed to get Employee Bee Deployment for Colony resource")
		reqLogger.V(8).Info("EBC:deployEmployeeBees: exit")
		return ctrl.Result{}, err
	}
	reqLogger.V(8).Info("EBC:existing Employee Bee Deployment resource already exists forColony, checking replica count")

	expectedEmployeeColonys := instance.Spec.FoodSourceNumber

	if *employeeBeeDeployment.Spec.Replicas != expectedEmployeeColonys {
		reqLogger.V(8).Info("EBC:deployEmployeeBees: updating replica count", "old_count", *employeeBeeDeployment.Spec.Replicas, "new_count", expectedEmployeeColonys)

		employeeBeeDeployment.Spec.Replicas = &expectedEmployeeColonys
		if err := r.Client.Update(ctx, employeeBeeDeployment); err != nil {
			reqLogger.Error(err, "EBC:deployEmployeeBees: failed to update Employee Bee Deployment replica count")
			reqLogger.V(8).Info("EBC:deployEmployeeBees: exit")
			return ctrl.Result{}, err
		}

		r.Recorder.Eventf(instance, core.EventTypeNormal, "Scaled", "Scaled Employee Bee deployment %q to %d replicas", employeeBeeDeployment.Name, expectedEmployeeColonys)
		reqLogger.V(8).Info("EBC:deployEmployeeBees: exit")
		return ctrl.Result{}, nil
	}

	reqLogger.V(8).Info("EBC:deployEmployeeBees: replica count up to date", "replica_count", *employeeBeeDeployment.Spec.Replicas)
	reqLogger.V(8).Info("EBC:deployEmployeeBees: exit")
	return ctrl.Result{}, nil
}

func buildEmployeeBeeDeployment(Colony abcoptimizerv1.Colony) *apps.Deployment {
	labels := map[string]string{
		"app":        Colony.Name,
		"controller": Colony.Name,
		"bee":        "employee",
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

func empStatusToString(beeStatus map[string]abcoptimizerv1.BeeStatus) string {
	empLogStatus := ""

	empLogStatus += "emp:["
	for bee, status := range beeStatus {
		empLogStatus += fmt.Sprint(bee, ": ", status.Status, ", ")
	}
	return empLogStatus
}

func empObjectiveFunctionStatusToString(beeStatus map[string]abcoptimizerv1.BeeStatus) string {
	empLogStatus := ""

	empLogStatus += "emp:["
	for bee, status := range beeStatus {
		empLogStatus += fmt.Sprint(bee, ": ", status.ObjFuncStatus, ", ")
	}
	return empLogStatus
}

func empFoodSourceDataToString(beeStatus map[string]abcoptimizerv1.BeeStatus) string {
	empLogStatus := ""

	empLogStatus += "emp:["
	for bee, status := range beeStatus {
		empLogStatus += fmt.Sprint(bee, ": ", status.FoodsourceVector, ", ", status.FoodsourceTrialCount, "; ")
	}
	return empLogStatus
}

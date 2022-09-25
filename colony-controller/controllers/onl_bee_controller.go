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
	onlookerBeeName          = "onlooker-bee"
	onlookerBeeContainerName = "onlooker-bee-container"
)

func (r *ColonyReconciler) onlookerBeeController(ctx context.Context, reqLogger logr.Logger, instance *abcoptimizerv1.Colony) (ctrl.Result, error) {
	reqLogger.Info("checking if an existing Onlooker Bee Deployment exists for this Colony")

	// 1. check existence and update onlooker-bee deployment in colony
	onlookerBeeDeployment := apps.Deployment{}
	result, err := r.deployOnlookerBees(ctx, &onlookerBeeDeployment, instance, reqLogger)
	if err != nil {
		reqLogger.Info("onlookerBeeController: error in onlooker bee deployment")
		return result, err
	}

	// 2. Get onlooker pod list
	onlPodList, err := r.getOnlPodList(ctx, instance, reqLogger)
	if err != nil {
		reqLogger.Info("onlookerBeeController: pods not found")
		return ctrl.Result{}, nil
	}

	// 3. Initialize onlooker bee status in colony
	if instance.Status.OnlookerBeeCycleStatus == "" && len(instance.Status.OnlookerBees) == 0 {
		initOnlookerBees(instance, onlPodList, reqLogger)
	}

	// 4. register bee and generate new fs vector when cycle is new
	reqLogger.Info("onlookerBeeController: cycle status: " + fmt.Sprint(instance.Status.OnlookerBeeCycleStatus) + ", foodsource len: " + fmt.Sprint(len(instance.Status.FoodSources["0"].Foodsource)))
	if instance.Status.OnlookerBeeCycleStatus == "Started" && len(instance.Status.FoodSources["0"].Foodsource) != 0 {

		// if enitre map not generated, wait until complete
		if len(instance.Status.ProbabilityMap) < int(instance.Spec.FoodSourceNumber) {
			return result, nil
		}
		result, err = r.registerAndAssignOnlooker(ctx, instance, onlPodList, reqLogger)
		if err != nil {
			reqLogger.Info("onlookerBeeController: error in registering bee to foodsource")
			return result, err
		}
		// all foodsources need to be filled before entering here ???
		result, err = generateNewOnlFsVector(instance, reqLogger)
		if err != nil {
			reqLogger.Info("onlookerBeeController: error in generating new fs vector")
			return result, err
		}
		reqLogger.Info("onlookerBeeController: bee: " + fmt.Sprint(instance.Status.OnlookerBees))
	}

	// 5. reassign
	if instance.Status.OnlookerBeeCycleStatus == "InProgress" {
		result, err := r.reassignOnlBeeStatus(ctx, instance, reqLogger)
		if err != nil {
			reqLogger.Info("onlookerBeeController: error in reassigning bee to foodsource")
			return result, err
		}
	}

	// 6. update fs vector
	if instance.Status.OnlookerBeeCycleStatus == "Completed" {
		result, err := onlUpdateFoodsources(instance, reqLogger)
		if err != nil {
			reqLogger.Info("onlookerBeeController: error in updating fs vector")
			return result, err
		}
	}

	// 7. end of onlooker cycle
	result, err = r.endOfOnlCycle(ctx, instance, &onlookerBeeDeployment, reqLogger)
	if err != nil {
		return result, err
	}

	return ctrl.Result{}, nil
}

func (r *ColonyReconciler) getOnlPodList(ctx context.Context, instance *abcoptimizerv1.Colony, reqLogger logr.Logger) ([]core.Pod, error) {
	onlookerBeeList := &core.PodList{}
	err := r.Client.List(ctx, onlookerBeeList, client.MatchingLabels{"bee": "onlooker"})
	if err != nil {
		return []core.Pod{}, err
	}
	reqLogger.Info("getOnlPodList: pod list: " + fmt.Sprint(onlookerBeeList.Items))
	onlookerList := onlookerBeeList.Items
	returnList := []core.Pod{}
	for _, bee := range onlookerList {
		if !slices.Contains(instance.Status.DeadBees, bee.GetName()) {
			returnList = append(returnList, bee)
		}
	}
	return returnList, nil
}

func initOnlookerBees(instance *abcoptimizerv1.Colony, onlookerList []core.Pod, reqLogger logr.Logger) (ctrl.Result, error) {
	reqLogger.Info("initOnlookerBees: enter")
	// 1. if all bees have not been created, wait
	onlBeeStatus := map[string]abcoptimizerv1.BeeStatus{}
	for _, bee := range onlookerList {
		if slices.Contains(instance.Status.DeadBees, bee.GetName()) {
			continue
		}
		newOnlStatus := abcoptimizerv1.BeeStatus{}
		newOnlStatus.Status = "New"
		newOnlStatus.FoodsourceId = ""
		newOnlStatus.ObjectiveFunction = ""
		newOnlStatus.FoodsourceVector = []string{}
		newOnlStatus.FoodsourceTrialCount = 0
		newOnlStatus.ObjFuncStatus = false
		onlBeeStatus[bee.GetName()] = newOnlStatus
	}

	if len(onlBeeStatus) < int(instance.Spec.FoodSourceNumber) {
		reqLogger.Info("initOnlookerBees: exit with warning")
		return ctrl.Result{}, nil
	}

	// 2. init bee status
	instance.Status.OnlookerBees = onlBeeStatus
	instance.Status.OnlookerBeeCycleStatus = "Started"

	reqLogger.Info("initEmployeeBees: onlooker bees: " + empLogString(instance.Status.OnlookerBees))
	reqLogger.Info("initOnlookerBees: exit")
	return ctrl.Result{}, nil
}

func (r *ColonyReconciler) registerAndAssignOnlooker(ctx context.Context, instance *abcoptimizerv1.Colony, onlookerList []core.Pod, reqLogger logr.Logger) (ctrl.Result, error) {
	reqLogger.Info("registerAndAssignOnlooker: enter")
	// 1. if all bees have not been created, wait
	if len(onlookerList) < int(instance.Spec.FoodSourceNumber) {
		return ctrl.Result{}, nil
	}

	// 2. register and assign foodsource to bee
	reqLogger.Info("registerAndAssignOnlooker: register ans assign foodsource to bee")
	instance.Status.OnlookerBeeCycleStatus = "InProgress"
	for _, bee := range onlookerList {
		reqLogger.Info("registerAndAssignOnlooker: iterating: " + bee.GetName())
		if instance.Status.OnlookerBees[bee.GetName()].Status == "New" {
			availableId, availableFoodsource, err := onlFindAvailableFoodsource(instance, reqLogger)
			for len(availableId) == 0 {
				availableId, availableFoodsource, err = onlFindAvailableFoodsource(instance, reqLogger)
			}
			if err != nil {
				// TODO: vacate again
				return ctrl.Result{}, err
			}
			onlStatus := abcoptimizerv1.BeeStatus{}
			if strings.Contains(availableFoodsource.OccupiedBy, "employee") {
				onlStatus.Status = "Waiting"
			} else {
				onlStatus.Status = "Running"
			}
			onlStatus.FoodsourceId = availableId
			onlStatus.FoodsourceVector = availableFoodsource.Foodsource
			onlStatus.ObjectiveFunction = availableFoodsource.ObjectiveFunction
			onlStatus.FoodsourceTrialCount = availableFoodsource.TrialCount
			onlStatus.ObjFuncStatus = false
			instance.Status.OnlookerBees[bee.GetName()] = onlStatus
			reqLogger.Info("registerAndAssignOnlooker: update colony status : " + fmt.Sprint(onlStatus))
		}
	}
	reqLogger.Info("registerAndAssignOnlooker: exit")
	return ctrl.Result{}, nil
}

func (r *ColonyReconciler) reassignOnlBeeStatus(ctx context.Context, instance *abcoptimizerv1.Colony, reqLogger logr.Logger) (ctrl.Result, error) {
	reqLogger.Info("reassignOnlBeeStatus: enter")
	// rassign bee status
	for bee, value := range instance.Status.OnlookerBees {
		// if bee waiting, bee reserved fs, fs occupied by none
		foodsource := instance.Status.FoodSources[value.FoodsourceId]
		if value.Status == "Waiting" && foodsource.ReservedBy == bee && foodsource.OccupiedBy == "" {
			value.Status = "Running"
		} else {
			// if bee running, ojective function value computed
			if value.Status == "Running" && value.ObjFuncStatus {
				value.Status = "Done"
			}
		}
		instance.Status.OnlookerBees[bee] = value
	}
	reqLogger.Info("reassignOnlBeeStatus: status: " + onlLogString(instance.Status.OnlookerBees))
	reqLogger.Info("reassignOnlBeeStatus: exit")
	return ctrl.Result{}, nil
}

func onlFindAvailableFoodsource(instance *abcoptimizerv1.Colony, reqLogger logr.Logger) (string, abcoptimizerv1.FoodsourceStatus, error) {
	reqLogger.Info("onlFindAvailableFoodsource: enter")
	if !checkFs(instance, reqLogger) {
		reqLogger.Info("onlFindAvailableFoodsource: foodsource vector is empty")
	}
	foodsources := instance.Status.FoodSources
	probabilityMap := instance.Status.ProbabilityMap

	for id, value := range foodsources {
		// not checking for occupied or reserved by onlooker
		// find foodsource where random val is less than its probability
		randomVal := rand.Float32()
		probability, err := strconv.ParseFloat(probabilityMap[id], 32)
		if err != nil {
			reqLogger.Info("onlFindAvailableFoodsource: cannot conert probability map value to float")
			return "", abcoptimizerv1.FoodsourceStatus{}, err
		}
		if randomVal < float32(probability) {
			reqLogger.Info("onlFindAvailableFoodsource: exit")
			return id, value, nil
		}
	}
	reqLogger.Info("onlFindAvailableFoodsource: exit with error")
	return "", abcoptimizerv1.FoodsourceStatus{}, fmt.Errorf("all foodsources occupied/reserved by onlooker")
}

func generateNewOnlFsVector(instance *abcoptimizerv1.Colony, reqLogger logr.Logger) (ctrl.Result, error) {
	reqLogger.Info("generateNewOnlFsVector: enter")
	reqLogger.Info("generateNewOnlFsVector: instance status: " + onlFsLogString(instance.Status.OnlookerBees))
	for bee, value := range instance.Status.OnlookerBees {
		id := value.FoodsourceId
		if id == "" {
			continue
		}
		currentVector := instance.Status.FoodSources[id].Foodsource
		reqLogger.Info("food source vector for " + id + " : " + fmt.Sprint(currentVector))
		partnerId := fmt.Sprint(rand.Intn(int(instance.Spec.FoodSourceNumber))) // TODO: cannot be equal to current id ??
		for partnerId == id {
			partnerId = fmt.Sprint(rand.Intn(int(instance.Spec.FoodSourceNumber)))
		}
		partnerVector := instance.Status.FoodSources[partnerId].Foodsource
		reqLogger.Info("partner vector for " + partnerId + " : " + fmt.Sprint(partnerVector))
		j := rand.Intn(int(instance.Spec.FoodsourceVectorLength))
		max := float32(1)
		min := float32(-1)
		phi := rand.Float32()*(max-min) + min
		newVector := make([]string, len(currentVector))
		copy(newVector, currentVector)
		curPosVal, err := strconv.ParseFloat(currentVector[j], 32)
		if err != nil {
			return ctrl.Result{}, err
		}
		partnerPosVal, err := strconv.ParseFloat(partnerVector[j], 32)
		if err != nil {
			reqLogger.Info("generateNewOnlFsVector: failed to convert partner vector to float")
			return ctrl.Result{}, err
		}
		newVector[j] = fmt.Sprint(float32(curPosVal) + phi*(float32(curPosVal)-float32(partnerPosVal)))
		onlookerBeeStatus := new(abcoptimizerv1.BeeStatus)
		onlookerBeeStatus.FoodsourceId = value.FoodsourceId
		onlookerBeeStatus.FoodsourceTrialCount = value.FoodsourceTrialCount
		onlookerBeeStatus.FoodsourceVector = newVector
		onlookerBeeStatus.ObjFuncStatus = value.ObjFuncStatus
		onlookerBeeStatus.ObjectiveFunction = value.ObjectiveFunction
		onlookerBeeStatus.Status = value.Status
		instance.Status.OnlookerBees[bee] = *onlookerBeeStatus
		// value.FoodsourceVector = newVector
		reqLogger.Info("generateNewOnlFsVector: Fs Vector " + id + ": " + fmt.Sprint(instance.Status.OnlookerBees[bee].FoodsourceVector))
	}
	reqLogger.Info("generateNewOnlFsVector: fs " + fmt.Sprint(instance.Status.OnlookerBees))
	reqLogger.Info("generateNewOnlFsVector: exit")
	return ctrl.Result{}, nil
}

func (r *ColonyReconciler) endOfOnlCycle(ctx context.Context, instance *abcoptimizerv1.Colony, onlookerBeeDeployment *apps.Deployment, reqLogger logr.Logger) (ctrl.Result, error) {
	reqLogger.Info("endOfOnlCycle: enter")
	onlookerBees := instance.Status.OnlookerBees

	doneCount := 0

	for bee, value := range onlookerBees {
		onlookerBeePod := core.Pod{}
		err := r.Client.Get(ctx, client.ObjectKey{Namespace: instance.Namespace, Name: bee}, &onlookerBeePod)
		if errors.IsNotFound(err) {
			reqLogger.Info("endOfOnlCycle: could not find existing Onlooker Bee Pod for Colony")
			value.Status = "Done"
			doneCount += 1
			continue
		}
		if err != nil {
			reqLogger.Info("endOfOnlCycle: failed to get Onlooker Bee Pod for Colony resource")
			value.Status = "Done"
			doneCount += 1
			continue
		}
		if onlookerBeePod.Status.Phase == "Running" && value.Status == "Done" { // TODO: done??
			doneCount += 1
		}
	}

	if doneCount >= int(instance.Spec.FoodSourceNumber) {
		reqLogger.Info("endOfOnlCycle: Re-Initializing Onlookers in the Colony")

		instance.Status.OnlookerBeeCycles += 1

		reqLogger.Info("endOfOnlCycle: Attempting to delete onlooker-bee deploymnet")
		onlookerBeeDeployment := apps.Deployment{}
		err := r.Client.Get(ctx, client.ObjectKey{Namespace: instance.Namespace, Name: onlookerBeeName}, &onlookerBeeDeployment)
		if errors.IsNotFound(err) {
			reqLogger.Info("endOfOnlCycle: could not find existing Onlooker Bee Deployment for Colony")
		} else {
			reqLogger.Info("endOfOnlCycle: Deleting onloooker-bee deploymnet")
			if err := r.Client.Delete(ctx, &onlookerBeeDeployment, &client.DeleteOptions{}); err != nil {
				reqLogger.Error(err, "failed to delete Onlooker Bee Deployment resource")
				return ctrl.Result{}, err
			}
		}

		// reqLogger.Info("endOfOnlCycle: Deleting onlooker deploymnet")
		// if err := r.Client.Delete(ctx, onlookerBeeDeployment, &client.DeleteOptions{}); err != nil {
		// 	reqLogger.Error(err, "endOfOnlCycle: failed to delete Onlooker Bee Deployment resource")
		// 	return ctrl.Result{}, err
		// }
		instance.Status.OnlookerBeeCycleStatus = "Completed"
	}
	reqLogger.Info("endOfOnlCycle: exit")
	return ctrl.Result{}, nil
}

func onlUpdateFoodsources(instance *abcoptimizerv1.Colony, reqLogger logr.Logger) (ctrl.Result, error) {
	for bee, value := range instance.Status.OnlookerBees {
		if len(value.ObjectiveFunction) == 0 {
			continue
		}
		newObjFunc, err := strconv.ParseFloat(value.ObjectiveFunction, 32)
		if err != nil {
			reqLogger.Error(err, "onlUpdateFoodsources: cannot convert new obj func to int")
			return ctrl.Result{}, err
		}
		newFitness := evaluateFitness(float32(newObjFunc))
		curObjFunc, err := strconv.ParseFloat(instance.Status.FoodSources[value.FoodsourceId].ObjectiveFunction, 32)
		if err != nil {
			reqLogger.Error(err, "onlUpdateFoodsources: cannot convert cur obj func to int")
			return ctrl.Result{}, err
		}
		curFitness := evaluateFitness(float32(curObjFunc))

		onlookerBeeStatus := new(abcoptimizerv1.BeeStatus)

		if newFitness >= curFitness {
			onlookerBeeStatus.FoodsourceTrialCount = 0
			onlookerBeeStatus.ObjectiveFunction = fmt.Sprint(newObjFunc)
		} else {
			onlookerBeeStatus.FoodsourceVector = instance.Status.FoodSources[value.FoodsourceId].Foodsource
			onlookerBeeStatus.FoodsourceTrialCount = onlookerBeeStatus.FoodsourceTrialCount + 1
		}
		instance.Status.OnlookerBees[bee] = *onlookerBeeStatus
	}
	return ctrl.Result{}, nil
}

func (r *ColonyReconciler) deployOnlookerBees(ctx context.Context, onlookerBeeDeployment *apps.Deployment, instance *abcoptimizerv1.Colony, reqLogger logr.Logger) (ctrl.Result, error) {
	err := r.Client.Get(ctx, client.ObjectKey{Namespace: instance.Namespace, Name: onlookerBeeName}, onlookerBeeDeployment)
	if errors.IsNotFound(err) {
		reqLogger.Info("could not find existing Onlooker Bee Deployment for Colony, creating one...")

		onlookerBeeDeployment = buildOnlookerBeeDeployment(*instance)
		if err := r.Client.Create(ctx, onlookerBeeDeployment); err != nil {
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
	reqLogger.Info("existing Onlooker Bee Deployment resource already exists for Colony, checking replica count")

	expectedOnlookerColonys := instance.Spec.FoodSourceNumber

	if *onlookerBeeDeployment.Spec.Replicas != expectedOnlookerColonys {
		reqLogger.Info("updating replica count", "old_count", *onlookerBeeDeployment.Spec.Replicas, "new_count", expectedOnlookerColonys)

		onlookerBeeDeployment.Spec.Replicas = &expectedOnlookerColonys
		if err := r.Client.Update(ctx, onlookerBeeDeployment); err != nil {
			reqLogger.Error(err, "failed to update Onlooker Bee Deployment replica count")
			return ctrl.Result{}, err
		}

		r.Recorder.Eventf(instance, core.EventTypeNormal, "Scaled", "Scaled Onlooker Bee deployment %q to %d replicas", onlookerBeeDeployment.Name, expectedOnlookerColonys)

		return ctrl.Result{}, nil
	}

	reqLogger.Info("replica count up to date", "replica_count", *onlookerBeeDeployment.Spec.Replicas)
	return ctrl.Result{}, nil
}

func buildOnlookerBeeDeployment(Colony abcoptimizerv1.Colony) *apps.Deployment {
	labels := map[string]string{
		"app":        Colony.Name,
		"controller": Colony.Name,
		"bee":        "onlooker",
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

func onlLogString(beeStatus map[string]abcoptimizerv1.BeeStatus) string {
	onlLogStatus := ""
	for bee, status := range beeStatus {
		onlLogStatus += fmt.Sprint(bee, ": {", status.Status, ", ", status.ObjFuncStatus, ", ", status.FoodsourceTrialCount, "}, ")
	}
	return onlLogStatus
}

func onlFsLogString(beeStatus map[string]abcoptimizerv1.BeeStatus) string {
	onlLogStatus := ""
	for bee, status := range beeStatus {
		onlLogStatus += fmt.Sprint(bee, ": ", status.FoodsourceVector, ", ", status.FoodsourceTrialCount, "; ")
	}
	return onlLogStatus
}

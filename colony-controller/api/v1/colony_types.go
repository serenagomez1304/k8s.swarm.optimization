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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type FoodsourceStatus struct {
	Foodsource []string `json:"fs_vector,omitempty"`
	TrialCount int32    `json:"trial_count,omitempty"`
	// EmployeeBee       string   `json:"employee_bee,omitempty"`
	// OnlookerBee       string   `json:"onlooker_bee,omitempty"`
	OccupiedBy        string `json:"occupied_by,omitempty"`
	ReservedBy        string `json:"reserved_by,omitempty"`
	ObjectiveFunction string `json:"objective_function,omitempty"`
}

type BeeStatus struct {
	// BeeName              string   `json:"bee_name,omitempty"`
	Status               string   `json:"bee_status,omitempty"`
	FoodsourceId         string   `json:"foodsource_id,omitempty"`
	ObjectiveFunction    string   `json:"bee_objective_function,omitempty"`
	FoodsourceVector     []string `json:"bee_fs_vector,omitempty"`
	FoodsourceTrialCount int32    `json:"bee_fs_trial_count,omitempty"`
	ObjFuncStatus        bool     `json:"bee_obj_func_status,omitempty"`
}

// ColonySpec defines the desired state of Colony
type ColonySpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of Colony. Edit colony_types.go to remove/update
	// EmployeeBees     int32             `json:"employee_bee_num,omitempty"`
	// OnlookerBees     int32             `json:"onlooker_bee_num,omitempty"`
	ColonyJob              metav1.ObjectMeta `json:"template,omitempty"`
	EmployeeBeeImage       string            `json:"employee_bee_image,omitempty"`
	OnlookerBeeImage       string            `json:"onlooker_bee_image,omitempty"`
	MaxTrialCount          int32             `json:"max_trial_count,omitempty"`
	FoodSourceNumber       int32             `json:"foodsource_num,omitempty"`
	FoodSourceImage        string            `json:"foodsource_image,omitempty"`
	TotalCycles            int32             `json:"number_of_cycles,omitempty"`
	FoodsourceVectorLength int32             `json:"fs_vec_len,omitempty"`
	UpperValues            []string          `json:"upper,omitempty"`
	LowerValues            []string          `json:"lower,omitempty"`
}

// ColonyStatus defines the observed state of Colony
type ColonyStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// AvailableReplicas      int32             `json:"availableReplicas"`
	EmployeeBeeCycles      int32                       `json:"completedEmployeeCycles,omitempty"`
	EmployeeBeeCycleStatus string                      `json:"completedEmployeeCycleStatus,omitempty"`
	EmployeeBees           map[string]BeeStatus        `json:"empBeeStatus,omitempty"`
	OnlookerBeeCycles      int32                       `json:"completedOnlookerCycles,omitempty"`
	OnlookerBeeCycleStatus string                      `json:"completedOnlookerCycleStatus,omitempty"`
	OnlookerBees           map[string]BeeStatus        `json:"onlBeeStatus,omitempty"`
	ProbabilityMap         map[string]string           `json:"probabilityMap,omitempty"`
	FoodSources            map[string]FoodsourceStatus `json:"foodsources,omitempty"`
	SavedFoodSources       []FoodsourceStatus          `json:"saved_fs_vector,omitempty"`
	DeadBees               []string                    `json:"dead_bees,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Colony is the Schema for the colonies API
type Colony struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ColonySpec   `json:"spec,omitempty"`
	Status ColonyStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ColonyList contains a list of Colony
type ColonyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Colony `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Colony{}, &ColonyList{})
}

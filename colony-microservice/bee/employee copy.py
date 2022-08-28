import time
import os

from kubernetes.client.rest import ApiException

import random

from kubernetes import client, config

import logr
import bee.linear as linear



class EmployeeBee:
	def __init__(self, bee):
		self.bee = bee
		self.foodsource_id = ""
		self.foodsources = None # to maintain a cache of foodsources read from colony

	def patch_colony_status(self, patch_body):
		api = client.CustomObjectsApi()
		try:
			
			logr.logr_info("patch_colony_status: patch body")
			logr.logr_info(str(patch_body))
			api_response = api.patch_namespaced_custom_object_status(
				group="abc-optimizer.innoventestech.com",
				version="v1",
				name="colony-sample",
				namespace="default",
				plural="colonies",
				body=patch_body,
				)
			logr.logr_info("patch_colony_status: response")
			logr.logr_info(str(api_response))
			if 'foodsources' in api_response['status']:
				in_foodsources = api_response['status']['foodsources']
				self.foodsources = self.rewrite_foodsources(in_foodsources)
		except ApiException as e:
			print("Exception when updating fs_vector %s\n" % e)
			logr.logr_err("patch_colony_status: Exception when updating fs_vector", e)
	
	def get_colony_status(self):
		api = client.CustomObjectsApi()
		try:
			api_response = api.get_namespaced_custom_object_status(
				group="abc-optimizer.innoventestech.com",
				version="v1",
				name="colony-sample",
				namespace="default",
				plural="colonies",
			)
			in_foodsources = api_response['status']['foodsources']
			self.foodsources = self.rewrite_foodsources(in_foodsources)
			return api_response['status']
		except ApiException as e:
			print("Exception when calling set_bee_status->get_cluster_custom_object_status or patch_namespaced_custom_object_status: %s\n" % e)
			logr.logr_info("get_colony_status: Exception when calling set_bee_status->get_cluster_custom_object_status or patch_namespaced_custom_object_status")
			return None
		

	def wait_for_termination(self):
		while True:
			print("wait to die...")
			time.sleep(5)

	def evaluate_fitness(self, obj_func_val):
		if obj_func_val >= 0:
			return 1/(1+obj_func_val)
		else:
			return 1 - obj_func_val	

	def generate_new_fs_vector(self, current_vector, partner_vector):
		j = random.randrange(0,3)
		phi = random.randrange(-1,1)
		new_vector = current_vector.copy()
		new_vector[j] = current_vector[j] + phi*(current_vector[j] - partner_vector[j])
		logr.logr_info("generate_new_fs_vector: generate new fs_vector " + str(new_vector))
		return new_vector

	def update_foodsources(self):
		# change fs_vector value
		# randomly increment trial count
		logr.logr_info("update_foodsources: update fs_vector for bee: " +  str(self.bee))
		self.foodsources = self.get_foodsources()
		
		vector = self.foodsources[self.foodsource_id]['fs_vector']
		
		# 1. generate new fs_vector
		partner_id = random.randrange(0, len(self.foodsources))
		new_vector = self.generate_new_fs_vector(vector, self.foodsources[str(partner_id)]['fs_vector'])
		
		# 2. evaluate new fitness
		new_obj_func = linear.Application.evaluate_obj_func(new_vector)
		logr.logr_info("update_foodsources: new objective function "+ str(new_obj_func))
		new_fitness = self.evaluate_fitness(new_obj_func)
		logr.logr_info("update_foodsources: new fitness " + str(new_fitness))

		# 3. evaluate current fitness
		cur_obj_func = linear.Application.evaluate_obj_func(vector)
		cur_fitness = self.evaluate_fitness(cur_obj_func)


		# 4. if new fitness better than current fitness ->  replace fs_vector
		if new_fitness > cur_fitness:
			# TODO: check for upper and lower bounds for new vector
			logr.logr_info("update_foodsources: new foodsource better than current, replace fs_vector")
			self.foodsources[self.foodsource_id]['fs_vector'] = new_vector
			self.foodsources[self.foodsource_id]['trial_count'] = 0
			self.foodsources[self.foodsource_id]['objetcive_function'] = str(new_obj_func)
			
		# 5. else increment trial count of current fs
		else:
			logr.logr_info("update_foodsources: current foodsource better than new, increment trial_count")
			if 'trail_count' not in self.foodsources[self.foodsource_id]:
				self.foodsources[self.foodsource_id]['trial_count'] = 1
			else:
				self.foodsources[self.foodsource_id]['trial_count'] = int(self.foodsources[self.foodsource_id]['trial_count']) + 1

		print("before update:", self.foodsources)
		logr.logr_info("update_foodsources: before update:" + str(self.foodsources))
		patch_body = {
			"status": {
				"foodsources": self.foodsources 
				}
			}
		self.patch_colony_status(patch_body)

	def register_bee(self):
		patch_body = {
			"status": {
				"completedEmployeeCycleStatus": {self.bee: "Running"}
				}
			}
		self.patch_colony_status(patch_body)

	def vacate_foodsources(self):
		self.foodsources = self.get_foodsources()
		self.foodsources[self.foodsource_id]['employee_bee'] = self.bee
		self.foodsources[self.foodsource_id]['occupied_by'] = ""
		patch_body = {
			"status": {
				"foodsources": self.foodsources
				}
			}
		logr.logr_info("vacate_foodsources: patch body")
		self.patch_colony_status(patch_body)

	def set_bee_status(self, state):
		
		colony_status = self.get_colony_status()

		if colony_status == None:
			logr.logr_warn("set_bee_status: colony status is None")
			return 

		registered_bees = colony_status['completedEmployeeCycleStatus']
		if self.bee in registered_bees:
			print("Bee found")
			logr.logr_info("Bee found")

			patch_body = {
			"status": {
				"completedEmployeeCycleStatus": {self.bee: state}
				}
			}

			logr.logr_info("set_bee_status: patch body")

			self.patch_colony_status(patch_body)

		else:
			print("Not found")
			logr.logr_warn("set_bee_status: Not found")

	def rewrite_foodsources(self, foodsources):
		for id, value in foodsources.items():
			if 'fs_vector' not in value:
				value['fs_vector'] = [int(random.randrange(1,10)), int(random.randrange(1,10)), int(random.randrange(1,10))]
			if 'trial_count' not in value:
				value['trial_count'] = 0
			if 'employee_bee' not in value:
				value['employee_bee'] = ""
			if 'onlooker_bee' not in value:
				value['onlooker_bee'] = ""
			if 'occupied_by' not in value:
				value['occupied_by'] = ""
			if 'reserved_by' not in value:
				value['reserved_by'] = ""
			if 'objective_function' not in value:
				value['objective_function'] = str(linear.Application.evaluate_obj_func(value['fs_vector']))
			else:
				value['objective_function'] = str(value['objective_function'])
			foodsources[id] = value
		return foodsources

	def get_foodsources(self):
		try:
			colony_status = self.get_colony_status()
			# pprint(api_response)
			if colony_status == None:
				logr.logr_warn("get_foodsources: colony status is None")
				return None

			self.foodsources = self.rewrite_foodsources(colony_status['foodsources'])
			logr.logr_info("get_foodsources: foodsources")
			logr.logr_info(str(self.foodsources))
			return self.foodsources
		except KeyError as e:
			print("Food source not initialized %s\n" % e)
			logr.logr_info("get_foodsources: food source not initialized")
			return None

	

	def reserve_foodsources(self, id):
		self.foodsources = self.get_foodsources()
		logr.logr_info("reserve_foodsources: reserving foodsource " + str(self.foodsources[id]))

		self.foodsources[id]['reserved_by'] = self.bee
		patch_body = {
			"status": {
				"foodsources": self.foodsources
				}
			}
		logr.logr_info("reserve_foodsources: patch body")
		self.patch_colony_status(patch_body)

	def wait_to_occupy(self, id):
		logr.logr_info("wait_to_occupy: Waiting to occupy")
		self.reserve_foodsources(id)
		while True:
			self.foodsources = self.get_foodsources()
			if "onlooker" in self.foodsources[id]['occupied_by']:
				logr.logr_info(str(self.bee) + " waiting for " + self.foodsources[id]['occupied_by'])
				time.sleep(2)
				continue
			else:
				return self.foodsources
		
	def cleanup_onlookers(self):
		self.foodsources = self.get_foodsources()
		print(self.foodsources)
		logr.logr_info("cleanup_onlookers: "+str(self.foodsources))

		for id, _ in self.foodsources.items():
			if self.foodsources[id]['occupied_by'] == "":
				self.foodsources[id]['onlooker_bee'] = ""
		patch_body = {
			"status": {
				"foodsources": self.foodsources
				}
			}
		self.patch_colony_status(patch_body)
		logr.logr_info("cleanup_onlookers:: completed")


	def assign_to_foodsources(self):
		self.foodsources = self.get_foodsources()
		print(self.foodsources)
		logr.logr_info("assign_to_foodsources: "+str(self.foodsources))

		for id, value in self.foodsources.items():
			print(value)
			if 'employee_bee' not in value:
				value['employee_bee'] = ""
			if value['employee_bee'] == "":
				if "employee" in self.foodsources[id]['reserved_by'] and self.foodsources[id]['reserved_by'] != self.bee:
					logr.logr_info( str(self.bee) + " skipping foodsource reserved by " + str(self.foodsources[id]['reserved_by']))
					continue
				self.foodsources = self.wait_to_occupy(id)
				self.foodsources[id]['employee_bee'] = self.bee
				self.foodsources[id]['occupied_by'] = self.bee
				self.foodsources[id]['reserved_by'] = ""
				patch_body = {
					"status": {
						"foodsources": self.foodsources
						}
					}
				self.patch_colony_status(patch_body)
				self.foodsource_id = id
				break
		if self.foodsource_id == "":
			logr.logr_warn("assign_to_foodsources: unable to assign foodsource")
		else:
			logr.logr_info("assign_to_foodsources: assign to foodsource_id"+str(self.foodsource_id))

	def wait_for_foodsources(self):
		while True:
			print("wait...")
			time.sleep(5)

			# change fs_vector value
			# randomly increment trial count

			self.foodsources = self.get_foodsources()
			self.rewrite_foodsources(self.foodsources)
			if self.foodsources != None:
				break

def main():
	# config.load_kube_config()
	config.load_incluster_config()

	bee = str(os.getenv("BEE_NAME"))
	print("BEE_NAME: " + bee)

	employee = EmployeeBee(bee)

	if bee == "None":
		employee.wait_for_termination()

	# 1. Register bee in colony
	# 2. Set status as running
	logr.logr_info("1. Register bee in colony")
	logr.logr_info("2. Set status as running")
	print("Registering bee", bee)
	employee.register_bee()

	# 3. Wait for fs_vector to be ready
	logr.logr_info("3. Wait for fs_vector to be ready")
	employee.wait_for_foodsources()

	employee.cleanup_onlookers()

	# 4. Wait and Assign to foodsources
	logr.logr_info("4. Assign to foodsources")
	employee.assign_to_foodsources()

	# 5. Update food source
	logr.logr_info("5. Update food source")
	employee.update_foodsources()

	# 6. Verify if bee is still registed, if true update status to done
	logr.logr_info("6. Verify if bee is still registed, if true update status to done")
	logr.logr_info("setting"+str(bee)+"bee status to done")
	print("setting", bee, "bee status to done")
	employee.set_bee_status("Done")

	# 7. Vacate foodsource
	employee.vacate_foodsources()
	logr.logr_info("7. Vacate foodsources")

	# 8. Wait for termination
	logr.logr_info("8. Wait for termination")
	employee.wait_for_termination()

if __name__ == '__main__':
	main()


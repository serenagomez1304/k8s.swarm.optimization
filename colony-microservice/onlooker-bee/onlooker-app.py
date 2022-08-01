from datetime import datetime
import time
import os

import random

from pprint import pprint
from kubernetes.client.rest import ApiException

from kubernetes import client, config

# importing module
import logging
 
bee = str(os.getenv("BEE_NAME"))
print("BEE_NAME: " + bee)

# Create and configure logger
logging.basicConfig(filename="/var/log/mycolony/onl"+str(datetime.now())+".log",
                    format='%(asctime)s %(message)s',
                    filemode='w')

 
# Creating an object
logger = logging.getLogger()

# Setting the threshold of logger to DEBUG
logger.setLevel(logging.DEBUG)

def logr_info(message):
	logger.info(bee + ": " + message) 


class Framework:
	def patch_fs_vector(foodsources):
		api = client.CustomObjectsApi()
		try:
			patch_body = {
				"status": {
					"foodsources": foodsources 
					}
				}
			logr_info("patch_fs_vector: patch body")
			logr_info(str(patch_body))
			response = api.patch_namespaced_custom_object_status(
				group="abc-optimizer.innoventestech.com",
				version="v1",
				name="colony-sample",
				namespace="default",
				plural="colonies",
				body=patch_body,
				)
			logr_info("patch_fs_vector: response")
			logr_info(str(response))
		except ApiException as e:
			print("Exception when updating fs_vector %s\n" % e)
			logr_info("Exception when updating fs_vector")


	def wait_for_termination():
		while True:
			print("wait to die...")
			time.sleep(5)

	def evaluate_fitness(obj_func_val):
		if obj_func_val >= 0:
			return 1/(1+obj_func_val)
		else:
			return 1 - obj_func_val

	def evaluate_probability(foodsources):
		max_fit = 0
		fit_map = {}
		for id, value in foodsources.items():
			obj_func = Application.evaluate_obj_func(value['fs_vector'])
			fit = Framework.evaluate_fitness(obj_func)
			fit_map[id] = fit
			if fit > max_fit:
				max_fit = fit

		probability_map = {}
		for id, value in foodsources.items():
			if max_fit == 0:
				probability_map[id] = 0.0
			else:
				probability = (0.9*(fit_map[id]/max_fit)) + 0.1
				probability_map[id] = probability
		return probability_map

	def generate_new_fs_vector(current_vector, partner_vector):
		j = random.randrange(0,3)
		phi = random.randrange(-1,1)
		new_vector = current_vector.copy()
		new_vector[j] = current_vector[j] + phi*(current_vector[j] - partner_vector[j])
		logr_info("generate new fs_vector: " + str(new_vector))
		return new_vector

	def update_foodsources(bee):
		# change fs_vector value
		# randomly increment trial count
		logr_info("update_foodsources: update fs_vector for bee: " +  str(bee))
		foodsources = Framework.get_foodsources()
		for id, value in foodsources.items():
			if 'onlooker_bee' not in value:
				continue
			if value['onlooker_bee'] == bee:
				vector = value['fs_vector']
				# 1. generate new fs_vector
				partner_id = str(random.randrange(0, len(foodsources)))
				new_vector = Framework.generate_new_fs_vector(vector, foodsources[partner_id]['fs_vector'])
				
				# 2. evaluate new fitness
				new_obj_func = Application.evaluate_obj_func(new_vector)
				logr_info("update_foodsources: new objective function: " +str(new_obj_func))
				new_fitness = Framework.evaluate_fitness(new_obj_func)
				logr_info("update_foodsources: new fitness: " + str(new_fitness))

				# 3. evaluate current fitness
				cur_obj_func = Application.evaluate_obj_func(vector)
				cur_fitness = Framework.evaluate_fitness(cur_obj_func)


				# 4. if new fitness better than current fitness ->  replace fs_vector
				if new_fitness > cur_fitness:
					# TODO: check for upper and lower bounds for new vector
					logr_info("update_foodsources: new foodsource better than current, replace fs_vector")
					value['fs_vector'] = new_vector
					value['trial_count'] = 0
					value['objetcive_function'] = str(new_obj_func)
					
				# 5. else increment trial count of current fs
				else:
					logr_info("update_foodsources: current foodsource better than new, increment trial_count")
					if 'trail_count' not in value:
						value['trial_count'] = 1
					else:
						value['trial_count'] = int(value['trial_count']) + 1
				foodsources[id] = value
				break
		print("before update:", foodsources)
		logr_info("update_foodsources: before update:" + str(foodsources))
		Framework.patch_fs_vector(foodsources)


	def vacate_foodsources(bee):
		api = client.CustomObjectsApi()
		foodsources = Framework.get_foodsources()
		for id, value in foodsources.items():
			if value['onlooker_bee'] != bee:
				continue
			try:
				foodsources[id]['onlooker_bee'] = bee
				foodsources[id]['occupied_by'] = ""
				patch_body = {
					"status": {
						"foodsources": foodsources
						}
					}
				logr_info("vacate_foodsources: patch body")
				response  = api.patch_namespaced_custom_object_status(
					group="abc-optimizer.innoventestech.com",
					version="v1",
					name="colony-sample",
					namespace="default",
					plural="colonies",
					body=patch_body,
					)
				logr_info("vacate_foodsources: response")
				logr_info(str(response))
			except ApiException as e:
				print("Exception when vacating foodsource to onlooker %s\n" % e)
				logr_info("Exception when vacating foodsource to onlooker")

	def register_bee(bee):
		api = client.CustomObjectsApi()
		try:
			patch_body = {
				"status": {
					"completedOnlookerCycleStatus": {bee: "Running"}
					}
				}

			api.patch_namespaced_custom_object_status(
				group="abc-optimizer.innoventestech.com",
				version="v1",
				name="colony-sample",
				namespace="default",
				plural="colonies",
				body=patch_body,
				)
		except ApiException as e:
			print("Exception when calling register_bee->patch_cluster_custom_object_status: %s\n" % e)
			logr_info("register_bee: Exception when calling register_bee->patch_cluster_custom_object_status")


	def set_bee_status(bee, state):
		api = client.CustomObjectsApi()
		try:
			api_response = api.get_namespaced_custom_object_status(
				group="abc-optimizer.innoventestech.com",
				version="v1",
				name="colony-sample",
				namespace="default",
				plural="colonies",
			)
			registered_bees = api_response['status']['completedOnlookerCycleStatus']

			if bee in registered_bees:
				print("Bee found")
				logr_info("Bee found")

				patch_body = {
				"status": {
					"completedOnlookerCycleStatus": {bee: state}
					}
				}
				logr_info("set_bee_status: patch body")

				response = api.patch_namespaced_custom_object_status(
					group="abc-optimizer.innoventestech.com",
					version="v1",
					name="colony-sample",
					namespace="default",
					plural="colonies",
					body=patch_body,
					)
				logr_info("set_bee_status: response")
				logr_info(str(response))

			else:
				print("Not found")
				logr_info("set_bee_status: Not found")

		except ApiException as e:
			print("Exception when calling set_bee_status->get_cluster_custom_object_status or patch_namespaced_custom_object_status: %s\n" % e)
			logr_info("set_bee_status: Exception when calling set_bee_status->get_cluster_custom_object_status or patch_namespaced_custom_object_status")

	def rewrite_foodsources(foodsources):
		for id, value in foodsources.items():
			if 'fs_vector' not in value:
				value['fs_vector'] = [-1,-1,-1]
			if 'trial_count' not in value:
				value['trial_count'] = 0
			if 'onlooker_bee' not in value:
				value['onlooker_bee'] = ""
			if 'onlooker_bee' not in value:
				value['onlooker_bee'] = ""
			if 'occupied_by' not in value:
				value['occupied_by'] = ""
			if 'reserved_by' not in value:
				value['reserved_by'] = ""
			if 'objective_function' not in value:
				value['objective_function'] = str(0.0)
			foodsources[id] = value
		return foodsources

	def get_foodsources():
		try:
			api = client.CustomObjectsApi()

			api_response = api.get_namespaced_custom_object_status(
				group="abc-optimizer.innoventestech.com",
				version="v1",
				name="colony-sample",
				namespace="default",
				plural="colonies",
			)
			# pprint(api_response)
			foodsources = api_response['status']['foodsources']
			foodsources = Framework.rewrite_foodsources(foodsources)
			logr_info("get_foodsources: foodsources")
			logr_info(str(foodsources))
			return foodsources

		except ApiException as e:
			print("Exception when getting fs_vector: %s\n" % e)
			logr_info("get_foodsources: Exception when getting fs_vector")
			return None
		except KeyError as e:
			print("Food source not initialized %s\n" % e)
			logr_info("get_foodsources: food source not initialized")
			return None

	def assign_onlooker(foodsources, id, bee):
		api = client.CustomObjectsApi()
		try:
			foodsources[id]['onlooker_bee'] = bee
			foodsources[id]['occupied_by'] = bee
			foodsources[id]['reserved_by'] = ""
			patch_body = {
				"status": {
					"foodsources": foodsources
					}
				}
			logr_info("assign_onlooker: patch body")
			response  = api.patch_namespaced_custom_object_status(
				group="abc-optimizer.innoventestech.com",
				version="v1",
				name="colony-sample",
				namespace="default",
				plural="colonies",
				body=patch_body,
				)
			logr_info("assign_onlooker: response")
			logr_info(str(response))
		except ApiException as e:
			print("Exception when assigning foodsource to onlooker %s\n" % e)
			logr_info("assign_onlooker: Exception when assigning foodsource to onlooker")

	def reserve_foodsources(id):
		foodsources = Framework.get_foodsources()
		logr_info("reserve_foodsources: reserving foodsource " + str(foodsources[id]))
		api = client.CustomObjectsApi()
		try:
			foodsources[id]['reserved_by'] = bee
			patch_body = {
				"status": {
					"foodsources": foodsources
					}
				}
			logr_info("reserve_foodsources: patch body")
			response  = api.patch_namespaced_custom_object_status(
				group="abc-optimizer.innoventestech.com",
				version="v1",
				name="colony-sample",
				namespace="default",
				plural="colonies",
				body=patch_body,
				)
			logr_info("reserve_foodsources: response")
			logr_info(str(response))
		except ApiException as e:
			print("Exception when reserving foodsource %s\n" % e)
			logr_info("reserve_foodsources: Exception when reserving foodsource")

	def wait_to_occupy(id):
		logr_info("Waiting to occupy")
		Framework.reserve_foodsources(id)
		while True:
			foodsources = Framework.get_foodsources()
			if "employee" in foodsources[id]['occupied_by']:
				logr_info(str(foodsources[id]['onlooker_bee']) + " waiting for " + foodsources[id]['occupied_by'])
				time.sleep(2)
				continue
			elif "onlooker" in foodsources[id]['reserved_by'] and foodsources[id]['reserved_by'] != bee:
				logr_info( str(bee) + " skipping foodsource reserved by " + str(foodsources[id]['reserved_by']))
				continue
			else:
				return foodsources


	def assign_to_foodsources(bee):
		foodsources = Framework.get_foodsources()
		print(foodsources)
		probability_map = Framework.evaluate_probability(foodsources)
		logr_info("assign_to_foodsources: probability map " + str(probability_map))
		logr_info("assign_to_foodsources: "+str(foodsources))

		for id, value in foodsources.items():
			print(value)
			r = random.randrange(0,1)
			logr_info("assign_to_foodsources: probability of foodsource:" + str(probability_map[id]))
			logr_info("assign_to_foodsources: random value r: " + str(r))
			if r < probability_map[id]:
				if 'onlooker_bee' not in value:
					value['onlooker_bee'] = ""
				if value['onlooker_bee'] == "":
					foodsources = Framework.wait_to_occupy(id)
					Framework.assign_onlooker(foodsources, id, bee)
					break


	def wait_for_foodsources():
		while True:
			print("wait...")
			time.sleep(5)

			# change fs_vector value
			# randomly increment trial count

			foodsources = Framework.get_foodsources()
			if foodsources != None:
				break

class Application:
	def evaluate_obj_func(fs_vector):
		# use fs_vector to compute objective function value
		# f = (x-3)^2 + (y-2)^2 + (z-1)^2
		# -10 < x < 10
		# -10 < y < 10
		# -10 < z < 10
		
		x = -10 if (fs_vector[0] < -10) else fs_vector[0]
		x = 10 if (x > 10) else x
		y = -10 if (fs_vector[1] < -10) else fs_vector[1]
		y = 10 if (y > 10) else y
		z = -10 if (fs_vector[2] < -10) else fs_vector[2]
		z = 10 if (z > 10) else z

		f = (x-3)**2 + (y-2)**2 + (z-1)**2

		return f

def main():
	# config.load_kube_config()
	config.load_incluster_config()

	if bee == "None":
		Framework.wait_for_termination()

	# 1. Register bee in colony
	# 2. Set status as running
	logr_info("1. Register bee in colony")
	logr_info("2. Set status as running")
	print("Registering bee", bee)
	Framework.register_bee(bee)

	# 3. Wait for fs_vector to be ready
	logr_info("3. Wait for fs_vector to be ready")
	Framework.wait_for_foodsources()

	# 4. Assign to fs_vector
	logr_info("4. Assign to fs_vector")
	Framework.assign_to_foodsources(bee)

	# 5. Update food source
	logr_info("5. Update food source")
	Framework.update_foodsources(bee)

	# 6. Verify if bee is still registed, if true update status to done
	logr_info("6. Verify if bee is still registed, if true update status to done")
	logr_info("setting"+str(bee)+"bee status to done")
	print("setting", bee, "bee status to done")
	Framework.set_bee_status(bee, "Done")

	# 7. Vacate fs_vector
	Framework.vacate_foodsources(bee)
	logr_info("7. Vacate fs_vector")

	# 8. Wait for termination
	logr_info("8. Wait for termination")
	Framework.wait_for_termination()

if __name__ == '__main__':
	main()


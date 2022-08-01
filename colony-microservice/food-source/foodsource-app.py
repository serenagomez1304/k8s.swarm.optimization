from datetime import datetime
import time
import os

from pprint import pprint

import random
from kubernetes.client.rest import ApiException

from kubernetes import client, config

# importing module
import logging
 
# Create and configure logger
logging.basicConfig(filename="/var/log/mycolony/foodsources.log",
                    format='%(asctime)s %(message)s',
                    filemode='w')
 
# Creating an object
logger = logging.getLogger()

# Setting the threshold of logger to DEBUG
logger.setLevel(logging.DEBUG)

# f = open('fs_vector-output.txt', 'a')

class Framework:
	def get_max_trial_count():
		try:
			api = client.CustomObjectsApi()

			api_response = api.get_namespaced_custom_object_status(
				group="abc-optimizer.innoventestech.com",
				version="v1",
				name="colony-sample",
				namespace="default",
				plural="colonies",
			)
			max_trial_count = api_response['spec']['max_trial_count']
			return max_trial_count

		except ApiException as e:
			print("Exception when calling set_bee_status->get_cluster_custom_object_status or patch_namespaced_custom_object_status: %s\n" % e)
			logger.info("get_max_trial_count: Exception when calling set_bee_status->get_cluster_custom_object_status or patch_namespaced_custom_object_status")
			return -1

	def get_saved_foodsources():
		try:
			api = client.CustomObjectsApi()

			api_response = api.get_namespaced_custom_object_status(
				group="abc-optimizer.innoventestech.com",
				version="v1",
				name="colony-sample",
				namespace="default",
				plural="colonies",
			)
			# if 'saved_fs_vector' not in api_response['status']:
			# 	api_response['status']['saved_fs_vector'] = {}
			# else:
			saved_foodsources = api_response['status']['saved_fs_vector']
			# print("saved food sources:", saved_foodsources)
			# logger.info("saved food sources:" + str(saved_foodsources) + "\n")
			return saved_foodsources

		except ApiException as e:
			print("Exception when getting fs_vector: %s\n" % e)
			logger.info("get_saved_foodsources: Exception when getting saved fs_vector")
			return []
		except KeyError as e:
			print("Exception when getting fs_vector: %s\n" % e)
			logger.info("get_saved_foodsources: Exception when getting saved fs_vector")
			return []

	def get_obj_func():
		val = random.randrange(-2,2)
		logger.info("get objective function "+ str(val))
		return str(val)

	def save_fs_vector(foodsources, id, value):
		print("food source being saved")
		logger.info("save_fs_vector: food source being saved"+ "\n")

		foodsources[str(id)] = {'fs_vector': [int(random.randrange(1,10)), int(random.randrange(1,10)), int(random.randrange(1,10))], 
									'trial_count': 0, 
									'employee_bee': "", 
									'onlooker_bee': "",
									'occupied_by': "",
									'reserved_by': "",
									'objective_function': Framework.get_obj_func()}

		api = client.CustomObjectsApi()
		try:
			saved_foodsources = Framework.get_saved_foodsources()
			logger.info("save_fs_vector: saved foodsources \n" + str(saved_foodsources)+ "\n")

			saved_foodsources.append(value)

			patch_body = {
				"status": {
					"saved_fs_vector": saved_foodsources,
					"foodsources": foodsources 
					}
				}
			logger.info("save_fs_vector: " + str(patch_body) + "\n")

			response = api.patch_namespaced_custom_object_status(
				group="abc-optimizer.innoventestech.com",
				version="v1",
				name="colony-sample",
				namespace="default",
				plural="colonies",
				body=patch_body,
				)
			logger.info("save_fs_vector: "+str(response)+ "\n")

		except ApiException as e:
			print("Exception when saving fs_vector %s\n" % e)
			logger.info("save_fs_vector: Exception when saving fs_vector \n" + str(e)+ "\n")

	def check_fs_vector_trials(foodsources, max_trial_count):
		try:
			if foodsources != None:
				for id, value in foodsources.items():
					if "trial_count" not in value:
						value['trial_count'] = 0
					if int(value['trial_count']) >= int(max_trial_count):
						print("save")
						Framework.save_fs_vector(foodsources, id, value)
					else:
						print("no solution yet")
			else:
				logger.info("check_fs_vector_trials: invalid fs_vector")
		except KeyError as e:
			print("key trial count not found", e)
			logger.info("check_fs_vector_trials: key trial count not found")

	def rewrite_foodsources(foodsources):
		logger.info("Rewrite foodsource")
		for id, value in foodsources.items():
			if 'fs_vector' not in value:
				value['fs_vector'] = [-1,-1,-1]
			if 'trial_count' not in value:
				value['trial_count'] = 0
			if 'employee_bee' not in value:
				value['employee_bee'] = ""
			if 'onlooker_bee' not in value:
				value['onlooker_bee'] = ""
			if 'occupied_by' not in value:
				value['occupied_by'] = ""
			if 'objective_function' not in value:
				value['objective_function'] = 0.0
			if 'reserved_by' not in value:
				value['reserved_by'] = ""
			foodsources[id] = value
		logger.info("rewrite_foodsources: " + str(foodsources))
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
			foodsources = api_response['status']['foodsources']
			foodsources = Framework.rewrite_foodsources(foodsources)
			return foodsources

		except ApiException as e:
			print("Exception when getting fs_vector: %s\n" % e)
			logger.info("get_foodsources: Exception when getting fs_vector:")
			return None
		except KeyError as e:
			print("Key error food source not found:", e)
			logger.info("get_foodsources: Key error food source not found:")
			return None

	def wait_for_termination():
		max_trial_count = Framework.get_max_trial_count()
		while True:
			print("wait...")
			logger.info("wait...")
			time.sleep(2)

			foodsources = Framework.get_foodsources()
			logger.info("wait_for_termination: "+str(foodsources)+ "\n")
			Framework.check_fs_vector_trials(foodsources, max_trial_count)

	def init_fs_vector(fs_vector_num):
		
		foodsources = Framework.get_foodsources()
		if foodsources != None:
			print("food sources not none")
			logger.info("init_fs_vector: fs_vector not none")
			return foodsources

		foodsources = {}
		for id in range(fs_vector_num):
			foodsources[str(id)] = {'fs_vector': [int(random.randrange(1,10)), int(random.randrange(1,10)), int(random.randrange(1,10))], 
									'trial_count': 0, 
									'employee_bee': "", 
									'onlooker_bee': "",
									'occupied_by': "",
									'resereved_by': "",
									'objective_function': Framework.get_obj_func()}

		api = client.CustomObjectsApi()
		try:
			patch_body = {
				"status": {
					"saved_fs_vector": [],
					"foodsources": foodsources
					}
				}

			logger.info("Init:")
			logger.info(str(patch_body))

			api.patch_namespaced_custom_object_status(
				group="abc-optimizer.innoventestech.com",
				version="v1",
				name="colony-sample",
				namespace="default",
				plural="colonies",
				body=patch_body,
				)
			logger.info("init_fs_vector: " + str(foodsources)+ "\n")
			return foodsources
		except ApiException as e:
			print("Exception when initializing fs_vector %s\n" % e)
			logger.info("init_fs_vector: Exception when initializing fs_vector")
			return None


	def get_fs_vector_num():
		try:
			api = client.CustomObjectsApi()

			api_response = api.get_namespaced_custom_object_status(
				group="abc-optimizer.innoventestech.com",
				version="v1",
				name="colony-sample",
				namespace="default",
				plural="colonies",
			)
			fs_vector_num = api_response['spec']['foodsource_num']
			return fs_vector_num

		except ApiException as e:
			print("Exception when getting fs_vector number: %s\n" % e)
			logger.info("get_fs_vector_num: Exception when getting fs_vector number:")
			return -1

def main():
	# config.load_kube_config()
	logger.info("hi")
	config.load_incluster_config()

	# 1. get fs_vector number
	fs_vector_num = Framework.get_fs_vector_num()
	print("Food source number:", fs_vector_num)
	logger.info("Food source number:" + str(fs_vector_num)+ "\n")

	# 2. Initialize foodsources
	req = Framework.init_fs_vector(fs_vector_num)
	print("init completed")
	print(req)
	logger.info("init completed")
	logger.info(str(req)+ "\n")

	# 3. Wait to terminate
	Framework.wait_for_termination()

if __name__ == '__main__':
	main()
	# f.close()
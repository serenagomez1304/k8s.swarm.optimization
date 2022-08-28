import time

from kubernetes.client.rest import ApiException

from kubernetes import client, config

import logr
import sample_app

class Foodsource:
	def __init__(self):
		self.foodsources = None
		self.saved_foodsources = None

		def get_foodsource_spec():
			try:
				api = client.CustomObjectsApi()

				api_response = api.get_namespaced_custom_object_status(
					group="abc-optimizer.innoventestech.com",
					version="v1",
					name="colony-sample",
					namespace="default",
					plural="colonies",
				)
				self.max_trial_count = api_response['spec']['max_trial_count']
				self.foodsource_num = api_response['spec']['foodsource_num']
				return self.max_trial_count, self.foodsource_num

			except ApiException as e:
				print("get_foodsource_spec: Exception when getting foodsource spec: %s\n" % e)
				logr.logger.error("get_foodsource_spec: Exception when getting foodsource spec: ", e, exc_info=1)
				return -1, -1

		self.max_trial_count, self.foodsource_num = get_foodsource_spec()

	def get_saved_foodsources(self):
		try:
			api = client.CustomObjectsApi()

			api_response = api.get_namespaced_custom_object_status(
				group="abc-optimizer.innoventestech.com",
				version="v1",
				name="colony-sample",
				namespace="default",
				plural="colonies",
			)
			
			self.foodsources = self.rewrite_foodsources(self.foodsources)

			if 'saved_fs_vector' not in api_response['status']:
				api_response['status']['saved_fs_vector'] = []

			self.saved_foodsources = api_response['status']['saved_fs_vector']
			# print("saved food sources:", saved_foodsources)
			# logr.logger.info("saved food sources:" + str(saved_foodsources) + "\n")
			return self.saved_foodsources

		except ApiException as e:
			print("Exception when getting fs_vector: %s\n" % e)
			logr.logger.erroe("get_saved_foodsources: Exception when getting saved fs_vector", e, exc_info=1)
			self.saved_foodsources = []
			return self.saved_foodsources
		except KeyError as e:
			print("Exception when getting fs_vector: %s\n" % e)
			logr.logger.error("get_saved_foodsources: Exception when getting saved fs_vector", e, exc_info=1)
			self.saved_foodsources = []
			return self.saved_foodsources

	def rewrite_foodsources(self, in_foodsources):
		logr.logger.info("Rewrite foodsource")
		for id, value in in_foodsources.items():
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
				value['objective_function'] = str(0.0)
			if 'reserved_by' not in value:
				value['reserved_by'] = ""
			self.foodsources[id] = value
		logr.logger.info("rewrite_foodsources: " + str(self.foodsources))
		return self.foodsources

	def patch_colony(self, patch_body):
		try:
			api = client.CustomObjectsApi()
			api_response = api.patch_namespaced_custom_object_status(
					group="abc-optimizer.innoventestech.com",
					version="v1",
					name="colony-sample",
					namespace="default",
					plural="colonies",
					body=patch_body,
					)
			logr.logger.info("patch_colony: "+str(api_response)+ "\n")
			in_foodsources = api_response['status']['foodsources']
			self.foodsources = self.rewrite_foodsources(in_foodsources)

			if 'saved_fs_vector' not in api_response['status']:
				api_response['status']['saved_fs_vector'] = []
			self.saved_foodsources = api_response['status']['saved_fs_vector']
		except ApiException as e:
			print("Exception when saving colony %s\n" % e)
			logr.logger.error("patch_colony: Exception when saving colony \n" + str(e)+ "\n", e, exc_info=1)


	def save_fs_vector(self, id, value):
		print("food source being saved")
		logr.logger.info("save_fs_vector: food source being saved"+ "\n")
		vec = sample_app.init_vector()
		self.foodsources[str(id)] = {'fs_vector': vec, 
									'trial_count': 0, 
									'employee_bee': "", 
									'onlooker_bee': "",
									'occupied_by': "",
									'reserved_by': "",
									'objective_function': str(sample_app.Application.evaluate_obj_func(vec))}

		self.saved_foodsources = self.get_saved_foodsources()
		logr.logger.info("save_fs_vector: saved foodsources \n" + str(self.saved_foodsources)+ "\n")

		self.saved_foodsources.append(value)

		patch_body = {
			"status": {
				"saved_fs_vector": self.saved_foodsources,
				"foodsources": self.foodsources 
				}
			}
		logr.logger.info("save_fs_vector: " + str(patch_body) + "\n")

		self.patch_colony(patch_body)


	def get_foodsources(self):
		try:
			api = client.CustomObjectsApi()

			api_response = api.get_namespaced_custom_object_status(
				group="abc-optimizer.innoventestech.com",
				version="v1",
				name="colony-sample",
				namespace="default",
				plural="colonies",
			)
			in_foodsources = api_response['status']['foodsources']
			self.foodsources = self.rewrite_foodsources(in_foodsources)
			if 'saved_fs_vector' not in api_response['status']:
				api_response['status']['saved_fs_vector'] = []
			self.saved_foodsources = api_response['status']['saved_fs_vector']
			return self.foodsources

		except ApiException as e:
			print("Exception when getting fs_vector: %s\n" % e)
			logr.logger.error("get_foodsources: Exception when getting fs_vector:", e, exc_info=1)
			return None
		except KeyError as e:
			print("Key error food source not found:", e)
			logr.logger.error("get_foodsources: Key error food source not found:", e, exc_info=1)
			return None

	def init_foodsources(self):
		
		# self.foodsources = self.get_foodsources()
		# if self.foodsources != None:
		# 	print("food sources not none")
		# 	logr.logger.info("init_foodsources: fs_vector not none")
		# 	return self.foodsources

		self.foodsources = {}
		for id in range(self.foodsource_num):
			vec = sample_app.Application.init_vector()
			self.foodsources[str(id)] = {'fs_vector': vec, 
									'trial_count': 0, 
									'employee_bee': "", 
									'onlooker_bee': "",
									'occupied_by': "",
									'resereved_by': "",
									'objective_function': str(sample_app.Application.evaluate_obj_func(vec))}

		try:
			patch_body = {
				"status": {
					"saved_fs_vector": [],
					"foodsources": self.foodsources
					}
				}

			logr.logger.info("Init:")
			logr.logger.info(str(patch_body))

			self.patch_colony(patch_body)

			logr.logger.info("init_foodsources: " + str(self.foodsources)+ "\n")
			return self.foodsources
		except ApiException as e:
			print("Exception when initializing fs_vector %s\n" % e)
			logr.logger.error("init_foodsources: Exception when initializing fs_vector", e, exc_info=1)
			return None

	def check_fs_vector_trials(self):
		try:
			if self.foodsources != None:
				for id, value in self.foodsources.items():
					if "trial_count" not in value:
						value['trial_count'] = 0
					if int(value['trial_count']) >= int(self.max_trial_count):
						print("save")
						self.save_fs_vector(id, value)
					else:
						print("no solution yet")
			else:
				logr.logger.info("check_fs_vector_trials: invalid fs_vector")
		except KeyError as e:
			print("key trial count not found", e)
			logr.logger.error("check_fs_vector_trials: key trial count not found", e, exc_info=1)

	def wait_for_termination(self):
		while True:
			print("wait...")
			logr.logger.info("wait...")
			time.sleep(2)

			# self.foodsources = self.get_foodsources()
			logr.logger.info("wait_for_termination: "+str(self.foodsources)+ "\n")
			# self.check_fs_vector_trials()

	

def main():
	# config.load_kube_config()
	config.load_incluster_config()

	foodsource = Foodsource()

	# # 1. Initialize foodsources
	# req = foodsource.init_foodsources()
	# print("init completed")
	# print(req)
	# logr.logger.info("init completed")
	# logr.logger.info(str(req)+ "\n")

	# 2. Wait to terminate
	foodsource.wait_for_termination()

if __name__ == '__main__':
	main()
	# f.close()
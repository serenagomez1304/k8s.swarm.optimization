import time

from kubernetes.client.rest import ApiException

from kubernetes import client

import logr

class Bee:
	def __init__(self, bee_name, obj_func):
		self.bee_name = bee_name
		self.obj_func = obj_func

	def patch_bee_status(self, patch_body):
		api = client.CustomObjectsApi()
		try:
			
			logr.logr_info("patch_bee_status: patch body")
			logr.logr_info(str(patch_body))
			api_response = api.patch_namespaced_custom_object_status(
				group="abc-optimizer.innoventestech.com",
				version="v1",
				name="colony-sample",
				namespace="default",
				plural="colonies",
				body=patch_body,
				)
			logr.logr_info("patch_bee_status: response")
			logr.logr_info(str(api_response))
		except ApiException as e:
			print("Exception when updating fs_vector %s\n" % e)
			logr.logr_err("patch_bee_status: Exception when updating fs_vector", e)
	
	def get_bee_status(self):
		api = client.CustomObjectsApi()
		try:
			api_response = api.get_namespaced_custom_object_status(
				group="abc-optimizer.innoventestech.com",
				version="v1",
				name="colony-sample",
				namespace="default",
				plural="colonies",
			)
			if "employee" in self.bee_name:
				in_bee_status = api_response['status']['empBeeStatus'][self.bee_name]
			else:
				in_bee_status = api_response['status']['onlBeeStatus'][self.bee_name]
			in_bee_status = self.rewrite_emp_status(in_bee_status)
			return in_bee_status

		except ApiException as e:
			print("Exception when calling  %s\n" % e)
			logr.logr_info("get_bee_status: Exception when calling")
			return None
		except KeyError as e:
			print("Key error:  %s\n" % e)
			logr.logr_info("get_bee_status: key error")
			return None

	def wait_for_termination(self):
		while True:
			print("wait to die...")
			time.sleep(5)

	def save_objective_function(self, bee_status, obj_func_val):
		bee_status['bee_objective_function'] = str(obj_func_val)
		bee_status['bee_obj_func_status'] = True
		if "employee" in self.bee_name:
			patch_body = {
				"status": {
					"empBeeStatus": {
						self.bee_name: bee_status
					}
				}
			}
		else:
			patch_body = {
				"status": {
					"onlBeeStatus": {
						self.bee_name: bee_status
					}
				}
			}
		self.patch_bee_status(patch_body)
		
	def rewrite_emp_status(self, emp_status):
		if 'bee_status' not in emp_status:
			emp_status['bee_status'] = ""
		if 'foodsource_id' not in emp_status:
			emp_status['foodsource_id'] = ""
		if 'bee_objective_function' not in emp_status:
			emp_status['bee_objective_function'] = ""
		if 'bee_fs_vector' not in emp_status:
			emp_status['bee_fs_vector'] = []
		if 'bee_fs_trial_count' not in emp_status:
			emp_status['bee_fs_trial_count'] = 0
		return emp_status


	def controller(self):

		if self.bee_name == "None":
			self.wait_for_termination()

		logr.logr_info("controller: bee name: "+ str(self.bee_name))

		# 1. get bee foodsource vector
		while True:
			bee_status = self.get_bee_status()
			if bee_status == None:
				time.sleep(2)
				continue
			logr.logr_info("controller: bee status: " + str(bee_status))
			if len(bee_status['bee_fs_vector']) != 0:
				break
			logr.logr_info("controller: waiting for bee assignment: " + self.bee_name)
			time.sleep(2)

		# 2. compute objective function value
		obj_func_val = self.obj_func(bee_status['bee_fs_vector'])
		logr.logr_info("controller: obj func val: " + str(obj_func_val))

		# 3. update employee status objective function vlaue
		self.save_objective_function(bee_status, obj_func_val)

		# 4. wait for termination
		self.wait_for_termination()



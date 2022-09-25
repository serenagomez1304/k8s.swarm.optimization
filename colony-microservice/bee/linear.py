from hashlib import new
from kubernetes import config
import os
import bee

class Application:
	def evaluate_obj_func(fs_vector):
		# TODO: use fs_vector to compute objective function value
		# f = (x-3)^2 + (y-2)^2 + (z-1)^2
		# -10 < x < 10
		# -10 < y < 10
		# -10 < z < 10

		coefficient = []
		for i in range(len(fs_vector)):
			coefficient.append(float(fs_vector[i]))
		
		x = -10 if (coefficient[0] < -10) else coefficient[0]
		x = 10 if (x > 10) else x
		y = -10 if (coefficient[1] < -10) else coefficient[1]
		y = 10 if (y > 10) else y
		z = -10 if (coefficient[2] < -10) else coefficient[2]
		z = 10 if (z > 10) else z

		f = (x-3)**2 + (y-2)**2 + (z-1)**2
		
		return f

def main():
	# config.load_kube_config()
	config.load_incluster_config()

	bee_name = str(os.getenv("BEE_NAME"))
	print("BEE_NAME: " + bee_name)

	bee_obj = bee.Bee(bee_name, Application.evaluate_obj_func)

	bee_obj.controller()

if __name__ == '__main__':
	main()
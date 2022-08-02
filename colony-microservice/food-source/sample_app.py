import random

class Application:
	
	def init_vector():
		return [int(random.randrange(1,10)), int(random.randrange(1,10)), int(random.randrange(1,10))]

	def evaluate_obj_func(fs_vector):
		# TODO: use fs_vector to compute objective function value
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
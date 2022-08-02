
# importing module
import logging
from datetime import datetime

# Create and configure logger
logging.basicConfig(filename="/var/log/mycolony/foodsource.log",
                    format='%(asctime)s %(message)s',
                    filemode='w')
 
# Creating an object
logger = logging.getLogger()

# Setting the threshold of logger to DEBUG
logger.setLevel(logging.DEBUG)

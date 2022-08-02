
# importing module
import logging
import employee
from datetime import datetime
import os

bee = str(os.getenv("BEE_NAME"))
print("BEE_NAME: " + bee)

# Create and configure logger
logging.basicConfig(filename="/var/log/mycolony/emp"+str(datetime.now())+".log",
                    format='%(asctime)s %(message)s',
                    filemode='w')
 
# Creating an object
logger = logging.getLogger()

# Setting the threshold of logger to DEBUG
logger.setLevel(logging.DEBUG)

def logr_info(message):
	logger.info(bee + ": " + message) 

def logr_warn(message, e = None):
    if e != None:
        logger.warn(bee + ": " + e + "," + message, exc_info=1)
    else:
        logger.warn(bee + ": " + message, exc_info=1)

def logr_err(message, e):
    logger.error(bee + ": " + e + "," + message, exc_info=1)
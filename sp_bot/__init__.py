import logging
import os
import sys
from telegram.ext import Application
from sp_bot.config import Config

from pymongo import MongoClient

# enable logging
logging.basicConfig(
    format="%(asctime)s - %(name)s - %(levelname)s - %(message)s",
    level=logging.INFO)

LOGGER = logging.getLogger(__name__)

# import ENV variables
TOKEN = Config.API_KEY

# spotify secrets
CLIENT_ID = Config.SPOTIFY_CLIENT_ID
CLIENT_SECRET = Config.SPOTIFY_CLIENT_SECRET
REDIRECT_URI = Config.REDIRECT_URI

# MongoDB secrets
MONGO_USR = Config.MONGO_USR
MONGO_PASS = Config.MONGO_PASS
COL = Config.MONGO_COLL

TEMP_CHANNEL = Config.TEMP_CHANNEL

# Create application instance (replaces updater/dispatcher in v20+)
application = Application.builder().token(TOKEN).build()

# SESSION = MongoClient("localhost", 27017)
SESSION = MongoClient(
    f"mongodb://{MONGO_USR}:{MONGO_PASS}@{COL}-shard-00-00.ibx7n.mongodb.net:27017,spotipie-shard-00-01.ibx7n.mongodb.net:27017,{COL}-shard-00-02.ibx7n.mongodb.net:27017/{COL}?ssl=true&replicaSet=atlas-lnda18-shard-0&authSource=admin&retryWrites=true&w=majority")

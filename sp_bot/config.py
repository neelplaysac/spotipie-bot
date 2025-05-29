import os
from dotenv import load_dotenv

# Load environment variables from .env file
load_dotenv()


class Config:

    API_KEY = os.getenv('API_KEY')
    BOT_USERNAME = os.getenv('BOT_USERNAME', 'spotipiebot')
    SPOTIFY_CLIENT_ID = os.getenv('SPOTIFY_CLIENT_ID')
    SPOTIFY_CLIENT_SECRET = os.getenv('SPOTIFY_CLIENT_SECRET')
    MONGO_USR = os.getenv('MONGO_USR')
    MONGO_PASS = os.getenv('MONGO_PASS')
    MONGO_COLL = os.getenv('MONGO_COLL')
    TEMP_CHANNEL = os.getenv('TEMP_CHANNEL')
    REDIRECT_URI = os.getenv('REDIRECT_URI')
    LASTFM_API_KEY = os.getenv('LASTFM_API_KEY')

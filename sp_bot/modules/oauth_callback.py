"""OAuth callback handler for Spotify authentication."""
import requests
from sp_bot.config import Config


class OAuthCallbackHandler:
    def __init__(self):
        self.nodejs_server_url = Config.REDIRECT_URI.rstrip('/callback') if Config.REDIRECT_URI else None

    def get_callback_url(self) -> str:
        return Config.REDIRECT_URI

    def check_auth_code(self, document_id: str) -> str:
        """Check if auth code is available for the given document ID from Node.js server"""
        if not self.nodejs_server_url:
            return None

        try:
            response = requests.get(f"{self.nodejs_server_url}/api/auth/{document_id}")
            if response.status_code == 200:
                data = response.json()
                return data.get('authCode')
        except Exception:
            pass
        return None


oauth_callback_handler = OAuthCallbackHandler()

"""OAuth callback handler for Spotify authentication."""
import secrets
from threading import Thread
from flask import Flask, request, redirect

from sp_bot.modules.db import DATABASE
from sp_bot.config import Config


class OAuthCallbackHandler:
    def __init__(self, host='0.0.0.0', port=6969):
        self.host = host
        self.port = port
        self.pending_states = {}
        self.app = Flask(__name__)
        self._setup_routes()

    def _setup_routes(self):
        @self.app.route('/callback')
        def spotify_callback():
            code = request.args.get('code')
            state = request.args.get('state')

            if not code or not state:
                return "Invalid request", 400

            telegram_user_id = self.pending_states.pop(state, None)
            if not telegram_user_id:
                return "Invalid or expired request", 400

            result = DATABASE.addCode(code, telegram_user_id)
            if result:
                document_id = str(result.inserted_id)
                return redirect(f"https://t.me/{Config.BOT_USERNAME}?start={document_id}")

            return "Database error", 500

    def generate_state(self, telegram_user_id: str) -> str:
        state = secrets.token_urlsafe(32)
        self.pending_states[state] = telegram_user_id
        return state

    def start_server(self):
        def run():
            self.app.run(host=self.host, port=self.port, debug=False)
        Thread(target=run, daemon=True).start()

    def get_callback_url(self) -> str:
        return Config.REDIRECT_URI or f"http://{self.host}:{self.port}/callback"


oauth_callback_handler = OAuthCallbackHandler()

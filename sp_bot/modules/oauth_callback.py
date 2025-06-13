"""
OAuth callback handler for Spotify authentication using Flask.
"""
import secrets
import os
from threading import Thread
import logging
from flask import Flask, request, redirect, jsonify

from sp_bot.modules.db import DATABASE
from sp_bot.config import Config

logger = logging.getLogger(__name__)


class OAuthCallbackHandler:
    def __init__(self, host='0.0.0.0', port=6969):
        self.host = host
        self.port = port
        self.pending_states = {}
        self.app = None
        self.server_thread = None
        self._setup_flask_app()

    def _setup_flask_app(self):
        """Setup Flask application with routes"""
        self.app = Flask(__name__)
        self.app.logger.setLevel(logging.ERROR)  # Suppress Flask logs

        # Disable Flask's default logging to avoid conflicts
        import werkzeug
        werkzeug_logger = logging.getLogger('werkzeug')
        werkzeug_logger.setLevel(logging.ERROR)

        @self.app.route('/callback')
        def spotify_callback():
            try:
                # Extract parameters
                code = request.args.get('code')
                state = request.args.get('state')
                error = request.args.get('error')

                if error:
                    logger.error(f"OAuth error: {error}")
                    return self._render_error_page("OAuth authorization was denied or failed."), 400

                if not code or not state:
                    logger.error("Missing code or state in callback")
                    return self._render_error_page("Invalid callback parameters."), 400

                # Validate state and get telegram user ID
                telegram_user_id = self.validate_state(state)
                if not telegram_user_id:
                    logger.error(f"Invalid state: {state}")
                    return self._render_error_page("Invalid or expired authorization request."), 400

                # Store the auth code in database
                try:
                    result = DATABASE.addCode(code, telegram_user_id)
                    if result:
                        document_id = str(result.inserted_id)
                        # Redirect to Telegram bot with the document ID
                        bot_username = Config.BOT_USERNAME
                        redirect_url = f"https://t.me/{bot_username}?start={document_id}"

                        logger.info(
                            f"OAuth callback successful for user {telegram_user_id}, redirecting to @{bot_username}")
                        return redirect(redirect_url)
                    else:
                        return self._render_error_page("Failed to save authorization code."), 500
                except Exception as e:
                    logger.exception(f"Database error: {e}")
                    return self._render_error_page("Database error occurred."), 500

            except Exception as e:
                logger.exception(f"Callback handler error: {e}")
                return self._render_error_page("An error occurred processing the callback."), 500

        @self.app.route('/health')
        def health_check():
            """Health check endpoint"""
            return jsonify({"status": "healthy", "service": "spotify-oauth-callback"})

    def _render_error_page(self, message: str) -> str:
        """Render an error page"""
        return f"""
        <!DOCTYPE html>
        <html>
        <head>
            <title>Authorization Error</title>
            <style>
                body {{ font-family: Arial, sans-serif; text-align: center; margin: 50px; }}
                .error {{ color: #d32f2f; }}
                .container {{ max-width: 600px; margin: 0 auto; }}
            </style>
        </head>
        <body>
            <div class="container">
                <h1 class="error">Authorization Error</h1>
                <p>{message}</p>
                <p>Please try again by using the /register command in the bot.</p>
            </div>
        </body>
        </html>
        """

    def generate_state(self, telegram_user_id: str) -> str:
        """Generate a unique state parameter for OAuth flow"""
        state = secrets.token_urlsafe(32)
        self.pending_states[state] = telegram_user_id
        return state

    def validate_state(self, state: str) -> str:
        """Validate state and return associated telegram user ID"""
        return self.pending_states.pop(state, None)

    def start_server(self):
        """Start the Flask OAuth callback server"""
        if self.server_thread is not None and self.server_thread.is_alive():
            logger.warning("Server is already running")
            return

        def run_server():
            logger.info(f"Starting Flask OAuth callback server on {self.host}:{self.port}")
            if Config.REDIRECT_URI:
                logger.info(f"Server accessible via: {Config.REDIRECT_URI}")

            # Run Flask app
            self.app.run(
                host=self.host,
                port=self.port,
                debug=False,
                use_reloader=False,
                threaded=True
            )

        self.server_thread = Thread(target=run_server, daemon=True)
        self.server_thread.start()

    def stop_server(self):
        """Stop the Flask OAuth callback server"""
        if self.server_thread and self.server_thread.is_alive():
            logger.info("Stopping Flask OAuth callback server")
            # Flask doesn't have a clean shutdown method when run in thread
            # The daemon thread will be terminated when main process exits
            self.server_thread = None

    def get_callback_url(self) -> str:
        """Get the callback URL for OAuth registration"""
        redirect_uri = Config.REDIRECT_URI
        if redirect_uri:
            return redirect_uri
        return f"http://{self.host}:{self.port}/callback"


# Global instance
oauth_callback_handler = OAuthCallbackHandler()

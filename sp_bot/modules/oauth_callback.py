"""
OAuth callback handler for Spotify authentication with built-in HTTP server.
"""
import secrets
import os
from http.server import BaseHTTPRequestHandler, HTTPServer
from urllib.parse import urlparse, parse_qs
from threading import Thread
import logging

from sp_bot.modules.db import DATABASE
from sp_bot.config import Config

logger = logging.getLogger(__name__)


class OAuthCallbackHandler:
    def __init__(self, host='localhost', port=6969):
        self.host = host
        self.port = port
        self.pending_states = {}
        self.server = None
        self.server_thread = None

    def generate_state(self, telegram_user_id: str) -> str:
        """Generate a unique state parameter for OAuth flow"""
        state = secrets.token_urlsafe(32)
        self.pending_states[state] = telegram_user_id
        return state

    def validate_state(self, state: str) -> str:
        """Validate state and return associated telegram user ID"""
        return self.pending_states.pop(state, None)

    def create_request_handler(self):
        """Create HTTP request handler class with access to this instance"""
        oauth_handler = self

        class CallbackRequestHandler(BaseHTTPRequestHandler):
            def do_GET(self):
                try:
                    # Parse the callback URL
                    parsed_url = urlparse(self.path)
                    query_params = parse_qs(parsed_url.query)

                    # Extract parameters
                    code = query_params.get('code', [None])[0]
                    state = query_params.get('state', [None])[0]
                    error = query_params.get('error', [None])[0]

                    if error:
                        logger.error(f"OAuth error: {error}")
                        self.send_error_response("OAuth authorization was denied or failed.")
                        return

                    if not code or not state:
                        logger.error("Missing code or state in callback")
                        self.send_error_response("Invalid callback parameters.")
                        return

                    # Validate state and get telegram user ID
                    telegram_user_id = oauth_handler.validate_state(state)
                    if not telegram_user_id:
                        logger.error(f"Invalid state: {state}")
                        self.send_error_response("Invalid or expired authorization request.")
                        return
                      # Store the auth code in database
                    try:
                        result = DATABASE.addCode(code, telegram_user_id)
                        if result:
                            document_id = str(result.inserted_id)
                            # Redirect to Telegram bot with the document ID
                            bot_username = Config.BOT_USERNAME
                            redirect_url = f"https://t.me/{bot_username}?start={document_id}"

                            self.send_response(302)
                            self.send_header('Location', redirect_url)
                            self.end_headers()

                            logger.info(
                                f"OAuth callback successful for user {telegram_user_id}, redirecting to @{bot_username}")
                        else:
                            self.send_error_response("Failed to save authorization code.")
                    except Exception as e:
                        logger.exception(f"Database error: {e}")
                        self.send_error_response("Database error occurred.")

                except Exception as e:
                    logger.exception(f"Callback handler error: {e}")
                    self.send_error_response("An error occurred processing the callback.")

            def send_error_response(self, message: str):
                """Send an error response to the user"""
                self.send_response(400)
                self.send_header('Content-type', 'text/html')
                self.end_headers()

                html_response = f"""
                <!DOCTYPE html>
                <html>
                <head>
                    <title>Authorization Error</title>
                    <style>
                        body {{ font-family: Arial, sans-serif; text-align: center; margin: 50px; }}
                        .error {{ color: #d32f2f; }}
                    </style>
                </head>
                <body>
                    <h1 class="error">Authorization Error</h1>
                    <p>{message}</p>
                    <p>Please try again by using the /register command in the bot.</p>
                </body>
                </html>
                """
                self.wfile.write(html_response.encode())

            def log_message(self, format, *args):
                """Override to use our logger instead of printing to stderr"""
                logger.info(format % args)

        return CallbackRequestHandler

    def start_server(self):
        """Start the OAuth callback server"""
        if self.server is not None:
            logger.warning("Server is already running")
            return

        try:
            handler_class = self.create_request_handler()
            self.server = HTTPServer((self.host, self.port), handler_class)

            def run_server():
                logger.info(f"Starting OAuth callback server on {self.host}:{self.port}")
                self.server.serve_forever()

            self.server_thread = Thread(target=run_server, daemon=True)
            self.server_thread.start()

        except Exception as e:
            logger.exception(f"Failed to start OAuth callback server: {e}")
            self.server = None

    def stop_server(self):
        """Stop the OAuth callback server"""
        if self.server:
            logger.info("Stopping OAuth callback server")
            self.server.shutdown()
            self.server.server_close()
            self.server = None
            if self.server_thread:
                self.server_thread.join(timeout=5)
                self.server_thread = None

    def get_callback_url(self) -> str:
        """Get the callback URL for OAuth registration"""
        return f"http://{self.host}:{self.port}/callback"


# Global instance
oauth_callback_handler = OAuthCallbackHandler()

import requests
import json
from sp_bot import CLIENT_ID, CLIENT_SECRET, REDIRECT_URI
from sp_bot.modules.oauth_callback import oauth_callback_handler


class SpotifyUser:
    authorize_url = "https://accounts.spotify.com/authorize"
    token_url = "https://accounts.spotify.com/api/token"

    def __init__(self, client_id, client_secret, redirect_uri):
        self.client_id = client_id
        self.client_secret = client_secret
        self.redirect_uri = redirect_uri

    def getAuthUrl(self, state=None):
        # Use the OAuth callback handler's URL if available, otherwise fall back to configured redirect URI
        callback_url = oauth_callback_handler.get_callback_url()
        redirect_uri = callback_url if callback_url else self.redirect_uri
        
        authorization_redirect_url = self.authorize_url + '?response_type=code&client_id=' + \
            self.client_id + '&redirect_uri=' + redirect_uri + \
            '&scope=user-read-currently-playing'
        if state:
            authorization_redirect_url += '&state=' + state
        return authorization_redirect_url

    def getAccessToken(self, authCode):
        data = {'grant_type': 'authorization_code',
                'code': authCode, 'redirect_uri': self.redirect_uri}
        r = requests.post(
            self.token_url, data=data, allow_redirects=True, auth=(self.client_id, self.client_secret))

        if r.status_code in range(200, 299):
            res = json.loads(r.text)
            return res['refresh_token']
        else:
            return 'error'

    def getCurrentyPlayingSong(self, refreshToken):
        data = {
            'grant_type': 'refresh_token',
            'refresh_token': refreshToken,
            'redirect_uri': self.redirect_uri,
            'client_id': self.client_id,
            'client_secret': self.client_secret
        }
        token = requests.post(self.token_url, data=data).json()

        headers = {
            'Accept': 'application/json',
            'Content-Type': 'application/json',
            'Authorization': 'Bearer ' + token['access_token']
        }
        r = requests.get(
            'https://api.spotify.com/v1/me/player/currently-playing', headers=headers)

        return r


SPOTIFY = SpotifyUser(CLIENT_ID, CLIENT_SECRET, REDIRECT_URI)

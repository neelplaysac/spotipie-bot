package telegram

const BOT_URL = "t.me/%s"

const START_TEXT = `Hi %s,

Follow these steps to start using the bot -
1. Use /register to connect your spotify account with this bot.
2. Open the provided link, give access to the bot & you will be redirected to bot's telegram link.
3. When you open that link you will be redirected back to telegram, click start.
4. After you see a 'successful' message use /name to set a display name (this will be displayed on the song status).

thats it! you can then share your song status using -
/now or using the inline query @spotipiebot.

You can also use /linkfm to connect your lastfm account.
Then set a display name using /namefm.
and use /last to share song status.

use /style to change background style.
use /help to get the list of commands.
`

const HELP_TEXT = `Heres the list of commands -

/now - share currently playing song on spotify.
/name - change your display name on song status.
/unregister - to unlink your spotify account from the bot.
/register - to connect your spotify account with the bot.
/style - change background style to blur or black.
@spotipiebot - share song using inline query

/last - share lastfm song status.
/linkfm - link LastFm account with the bot.
/namefm - set display name for LastFm status.
/unlinkfm - unlink LastFm account from bot.
/status - recently played song via LastFm.

`

const NOW_REG_MSG = "You need to connect your Spotify account first. Contact me in pm and use /register command."
const NOW_USR_NAME_MSG = "You need to add a username to start using the bot. Contact me in pm and use /name command."
const NOW_TOKEN_ERR_MSG = `Your spotify account is not properly linked with bot :( 
please use /unregister command in pm and /register again.`

const STYLE_REG_MSG = "Use /register to connect your spotify account or /linkfm to connect your LastFm account to get started."

const NAME_PM_MSG = "Contact me in pm to change your username (*this will not change your telegram username)."
const NAME_REG_MSG = "You need to register first. use /register to get started."

const REG_REG_MSG = "Open the link below, to connect your Spotify account."
const REG_PM_MSG = "Contact me in pm to /register or /unregister your account."

const FM_PM_MSG = "Contact me in pm and use /linkfm to link or /unlinkfm to unlink your LastFm account."
const FM_REG_MSG = "Open the link below, to connect your LastFm account."
const FM_NAME_PM_MSG = "Contact me in pm to change your display name."
const FM_NAME_REG_MSG = "You need to link your LastFm account first. use /lastfm in pm to get started."

const LAST_REG_MSG = "You need to connect your LastFm account first. Contact me in pm and use /linkfm command."
const LAST_USR_NAME_MSG = "You need to add a display name to start using the bot. Contact me in pm and use /linkfm command."

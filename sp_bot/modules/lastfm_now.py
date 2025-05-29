import requests

from telegram import Update, InlineKeyboardMarkup, InlineKeyboardButton
from telegram.ext import CommandHandler, ContextTypes, ConversationHandler
from telegram.constants import ChatAction

from sp_bot import application, LOGGER
from sp_bot.modules.db import DATABASE
from sp_bot.modules.misc.blurr_image import blurrImage
from sp_bot.config import Config

REG_MSG = 'You need to connect your LastFm account first. Contact me in pm and use /linkfm command.'
USR_NAME_MSG = 'You need to add a display name to start using the bot. Contact me in pm and use /linkfm command.'
BOT_URL = 't.me/{}'
SCROBBLER_URL = 'http://ws.audioscrobbler.com/2.0/'
LASTKEY = Config.LASTFM_API_KEY


async def nowLastFm(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    """Sends currently playing song when command /noww is issued."""
    await context.bot.send_chat_action(update.message.chat_id, ChatAction.TYPING)

    try:
        tg_id = str(update.message.from_user.id)
        is_user = DATABASE.getLastFmUser(tg_id)
        if is_user == None:
            if update.effective_chat.type == "private":
                await update.effective_message.reply_text(
                    "Use /linkfm command to link your lastfm account first.")
            else:
                button = InlineKeyboardMarkup(
                    [[InlineKeyboardButton(text='Contact in pm', url=BOT_URL.format(context.bot.username))]])
                await update.effective_message.reply_text(
                    REG_MSG, reply_markup=button)

            return ConversationHandler.END
        elif is_user["name"] == 'User':
            if update.effective_chat.type == "private":
                await update.effective_message.reply_text(
                    "Use /namefm command to set a display name.")
            else:
                button = InlineKeyboardMarkup(
                    [[InlineKeyboardButton(text='Contact in pm', url=BOT_URL.format(context.bot.username))]])
                await update.effective_message.reply_text(
                    USR_NAME_MSG, reply_markup=button)

            return ConversationHandler.END

        else:
            last_fm_username = is_user['fm_username']
            r = getLastFmStatus(last_fm_username)

    except Exception as ex:
        LOGGER.exception(ex)
        return ConversationHandler.END

    try:
        pfp_url = await context.bot.get_user_profile_photos(
            tg_id, limit=1)
        if pfp_url.photos:
            file_id = pfp_url.photos[0][0].file_id
            file_obj = await context.bot.get_file(file_id)
            pfp = requests.get(file_obj.file_path)
        else:
            pfp = None
    except:
        pfp = None

    try:
        if 'error' in r.json():
            response = "LastFM username is invalid. Try to relink lastfm with the bot."
            await update.message.reply_text(response)
            return ConversationHandler.END

        elif r.status_code not in range(200, 299):
            response = "Unable to get data from LastFm."
            await update.message.reply_text(response)
            return ConversationHandler.END

        else:
            username = is_user["name"]
            if "counter" in is_user and is_user["counter"] == "off":
                image = blurrImage(r, username, pfp, scrobbles="off")
            else:
                scrobbles = getScrobbles(r, last_fm_username)
                image = blurrImage(r, username, pfp, scrobbles)
            button = InlineKeyboardButton(
                text="LastFm Song Link", url=r.json()['recenttracks']['track'][0]['url'])

            await context.bot.send_photo(
                update.message.chat_id, image, reply_markup=InlineKeyboardMarkup([[button]]))

    except Exception as ex:
        LOGGER.exception(ex)
        await update.message.reply_text("Unable to get data from LastFm.")


def getLastFmStatus(fm_username):
    METHOD = 'method=user.getrecenttracks'
    r = requests.get(
        f"{SCROBBLER_URL}?{METHOD}&limit=1&user={fm_username}&api_key={LASTKEY}&format=json")
    return r


def getScrobbles(res, user):
    track = res.json()['recenttracks']['track'][0]
    artists = track['artist']['#text']
    albumname = track['album']['#text']
    songname = track['name']
    scr = "00"
    try:
        res = requests.get(
            f"{SCROBBLER_URL}?method=track.getInfo&username={user}&api_key={LASTKEY}&artist={artists}&track={songname}&format=json")
        scr = res.json()["track"]["userplaycount"]
    except:
        pass

    return scr


LASTFM_HANDLER = CommandHandler("last", nowLastFm)
application.add_handler(LASTFM_HANDLER)

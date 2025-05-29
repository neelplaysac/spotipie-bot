import requests

from telegram import Message, Chat, Update, Bot, User, ChatAction, InlineKeyboardMarkup, InlineKeyboardButton
from telegram.ext import CommandHandler, CallbackContext, ConversationHandler, run_async

from sp_bot import dispatcher
from sp_bot.modules.misc.cook_image import drawImage
from sp_bot.modules.db import DATABASE
from sp_bot.modules.misc.request_spotify import SPOTIFY

REG_MSG = 'You need to connect your Spotify account first. Contact me in pm and use /register command.'
USR_NAME_MSG = 'You need to add a username to start using the bot. Contact me in pm and use /name command.'
TOKEN_ERR_MSG = '''
Your spotify account is not properly linked with bot :( 
please use /unregister command in pm and /register again.
'''
BOT_URL = 't.me/{}'


def nowPlaying(update: Update, context: CallbackContext) -> None:
    """Sends currently playing song when command /noww is issued."""
    context.bot.sendChatAction(update.message.chat_id, ChatAction.TYPING)

    try:
        tg_id = str(update.message.from_user.id)
        is_user = DATABASE.fetchData(tg_id)
        if is_user == None:
            button = InlineKeyboardMarkup(
                [[InlineKeyboardButton(text='Contact in pm', url=BOT_URL.format(context.bot.username))]])
            update.effective_message.reply_text(REG_MSG, reply_markup=button)

            return ConversationHandler.END
        elif is_user["username"] == 'User':
            button = InlineKeyboardMarkup(
                [[InlineKeyboardButton(text='Contact in pm', url=BOT_URL.format(context.bot.username))]])
            update.effective_message.reply_text(
                USR_NAME_MSG, reply_markup=button)

            return ConversationHandler.END
        elif is_user["token"] == '00000':
            button = InlineKeyboardMarkup(
                [[InlineKeyboardButton(text='Contact in pm', url=BOT_URL.format(context.bot.username))]])
            update.effective_message.reply_text(
                TOKEN_ERR_MSG, reply_markup=button)

            return ConversationHandler.END
        else:
            token = is_user["token"]
            r = SPOTIFY.getCurrentyPlayingSong(token)

    except Exception as ex:
        print(ex)
        return

    try:
        pfp_url = context.bot.getUserProfilePhotos(
            tg_id, limit=1)['photos'][0][0]['file_id']
        pfp = requests.get(context.bot.getFile(pfp_url).file_path)
    except:
        pfp = None

    try:
        res = r.json()
        if res['currently_playing_type'] == 'ad':
            response = "You're listening to ads."

        elif res['currently_playing_type'] == 'track':
            username = is_user["username"]
            style = is_user["style"]
            image = drawImage(res, username, pfp, style)
            button = InlineKeyboardButton(
                text="Play on Spotify", url=res['item']['external_urls']['spotify'])

            context.bot.send_photo(
                update.message.chat_id, image, reply_markup=InlineKeyboardMarkup([[button]]))

        else:
            response = "Not sure what you're listening to."
            update.message.reply_text(response)
    except Exception as ex:
        print(ex)
        update.message.reply_text("You are not listening to anything.")


NOW_PLAYING_HANDLER = CommandHandler("now", nowPlaying, run_async=True)
dispatcher.add_handler(NOW_PLAYING_HANDLER)

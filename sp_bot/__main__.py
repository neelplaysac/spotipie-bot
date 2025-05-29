import re
import importlib
from bson.objectid import ObjectId

from telegram import Update, InlineKeyboardMarkup, InlineKeyboardButton
from telegram.ext import CommandHandler, MessageHandler, CallbackQueryHandler, ContextTypes, ConversationHandler, filters
from telegram.constants import ParseMode, ChatAction

from sp_bot.modules import ALL_MODULES
from sp_bot import application, LOGGER, TOKEN
from sp_bot.modules.misc.request_spotify import SPOTIFY
from sp_bot.modules.db import DATABASE

START_TEXT = '''
Hi {},

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
'''

HELP_TEXT = '''
Heres the list of commands -

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

'''

IMPORTED = {}

for module_name in ALL_MODULES:
    imported_module = importlib.import_module("sp_bot.modules." + module_name)

    if not hasattr(imported_module, "__mod_name__"):
        imported_module.__mod_name__ = imported_module.__name__

    if not imported_module.__mod_name__.lower() in IMPORTED:
        IMPORTED[imported_module.__mod_name__.lower()] = imported_module
    else:
        raise Exception(
            "Can't have two modules with the same name! Please change one")


# /start command
async def start(update: Update, context: ContextTypes.DEFAULT_TYPE):
    if update.effective_chat.type == "private":
        first_name = update.effective_user.first_name
        text = update.effective_message.text
        if len(text) <= 10:
            await update.effective_message.reply_text(
                START_TEXT.format(first_name), parse_mode=ParseMode.MARKDOWN)
        elif text.endswith('register'):
            await update.message.reply_text(
                "To register your account use /register command.")
        elif text.endswith('username'):
            await update.message.reply_text(
                "To change your username use /name command.")
        elif text.endswith('token'):
            await update.message.reply_text(
                "use /unregister to unlink your account & register again using /register command.")
        elif text.endswith('notsure'):
            await update.message.reply_text("I'm not sure what you're listening to.")
        elif text.endswith('ads'):
            await update.message.reply_text(
                "You're listening to ads!")
        elif text.endswith('notlistening'):
            await update.message.reply_text(
                "You're not listening to anything on Spotify at the moment.")
        elif text.endswith('lastfm'):
            await update.message.reply_text(
                "To link your lastfm account use /lastfm command.")
        else:
            _id = text[7:]
            try:
                codeObject = DATABASE.fetchCode(ObjectId(_id))
                _ = DATABASE.deleteCode(ObjectId(_id))
            except Exception as ex:
                await update.message.reply_text(
                    "Please use /register command to initiate the login process.")
                LOGGER.exception(ex)
                return ConversationHandler.END

            if codeObject is None:
                await update.message.reply_text(
                    "Please use /register command to initiate the login process.")
            else:
                try:
                    tg_id = str(update.effective_user.id)
                    is_user = DATABASE.fetchData(tg_id)
                    if is_user != None:
                        await update.message.reply_text(
                            "You are already registered. If the bot is not working /unregister and /register again.")
                        return ConversationHandler.END
                    authcode = codeObject["authCode"]
                    refreshToken = SPOTIFY.getAccessToken(authcode)
                    if refreshToken == 'error':
                        await update.message.reply_text(
                            "Unable to authenticate. Please try again using /register. If you are having issues using the bot contact in support chat (check bot info)")
                        return ConversationHandler.END
                    user = DATABASE.addUser(tg_id, refreshToken)
                    await update.message.reply_text(
                        "Account successfully linked. Now use /name to set a display name then use /now to use the bot.")
                except Exception as ex:
                    await update.message.reply_text("Database Error")
                    LOGGER.exception(ex)
                    return ConversationHandler.END

        await update.effective_message.delete()
        return ConversationHandler.END
    else:
        await update.effective_message.reply_text("Hmm?")
        return ConversationHandler.END


# /help command
async def get_help(update: Update, context: ContextTypes.DEFAULT_TYPE):
    # ONLY send help in PM
    if update.effective_chat.type != "private":
        await update.effective_message.reply_text("Contact me in PM to get the list of possible commands.",
                                            reply_markup=InlineKeyboardMarkup(
                                                [[InlineKeyboardButton(text="Help",
                                                                       url="t.me/{}?start=help".format(
                                                                           context.bot.username))]]))
        return

    else:
        await update.effective_message.reply_text(
            HELP_TEXT, parse_mode=ParseMode.MARKDOWN)


# main
def main():
    start_handler = CommandHandler("start", start)
    help_handler = CommandHandler("help", get_help)

    application.add_handler(start_handler)
    application.add_handler(help_handler)

    application.run_polling()


if __name__ == '__main__':
    LOGGER.info("Successfully loaded modules: " + str(ALL_MODULES))
    main()

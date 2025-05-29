from telegram import Update, InlineKeyboardButton, InlineKeyboardMarkup
from telegram.ext import Updater, CallbackContext, CommandHandler, ConversationHandler, Filters, MessageHandler

from sp_bot import dispatcher, LOGGER
from sp_bot.modules.db import DATABASE


PM_MSG = 'Contact me in pm to change your display name.'
REG_MSG = 'You need to link your LastFm account first. use /lastfm in pm to get started.'
BOT_URL = 't.me/{}'


# /username command
def getLastFmUserData(update: Update, context: CallbackContext) -> None:
    'ask user for usename'
    if update.effective_chat.type != update.effective_chat.PRIVATE:
        button = InlineKeyboardMarkup(
            [[InlineKeyboardButton(text="Change display name", url=BOT_URL.format(context.bot.username))]])
        update.effective_message.reply_text(
            PM_MSG, reply_markup=button)
        return ConversationHandler.END
    update.effective_message.reply_text(
        "Send me a username (max 15 characters)")
    return LASTFM_DISPLAY_NAME


# username command state
def setLastFmUserData(update: Update, context: CallbackContext) -> None:
    'save username in db'
    text = update.effective_message.text.strip()
    if len(text) > 15:
        update.message.reply_text(
            "Invalid username. Try again using /namefm ")
        return ConversationHandler.END
    elif text.startswith('/'):
        update.message.reply_text(
            "Invalid username. Try again using /namefm ")
        return ConversationHandler.END
    else:
        try:
            tg_id = str(update.message.from_user.id)
            is_user = DATABASE.getLastFmUser(tg_id)

            if is_user == None:
                update.message.reply_text(REG_MSG)
                return ConversationHandler.END
            else:
                DATABASE.updateLastFmData(tg_id, text)
                update.message.reply_text(
                    f"Username updated to {text}. Use /last to share lastfm song status.")
                return ConversationHandler.END

        except Exception as ex:
            LOGGER.exception(ex)
            update.message.reply_text("Database Error")
            return ConversationHandler.END


def cancel(update, context):
    update.message.reply_text('Canceled.')
    return ConversationHandler.END


LASTFM_DISPLAY_NAME = 1
NAMEFM_HANDLER = ConversationHandler(
    entry_points=[CommandHandler('namefm', getLastFmUserData)],
    states={LASTFM_DISPLAY_NAME: [MessageHandler(
            Filters.text & ~Filters.command, setLastFmUserData)]},
    fallbacks=[CommandHandler('cancel', cancel)])

dispatcher.add_handler(NAMEFM_HANDLER)

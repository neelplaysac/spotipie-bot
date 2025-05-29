from telegram import Update, InlineKeyboardButton, InlineKeyboardMarkup
from telegram.ext import Updater, CallbackContext, CommandHandler, ConversationHandler, Filters, MessageHandler

from sp_bot import dispatcher, LOGGER
from sp_bot.modules.db import DATABASE

PM_MSG = 'Contact me in pm and use /linkfm to link or /unlinkfm to unlink your LastFm account.'
REG_MSG = 'Open the link below, to connect your LastFm account.'
BOT_URL = 't.me/{}'


def getLastFmUserName(update: Update, context: CallbackContext) -> None:
    'ask user for usename'
    if update.effective_chat.type != update.effective_chat.PRIVATE:
        button = InlineKeyboardMarkup(
            [[InlineKeyboardButton(text="Link LastFm account.", url=BOT_URL.format(context.bot.username))]])
        update.effective_message.reply_text(
            PM_MSG, reply_markup=button)
        return ConversationHandler.END
    else:
        update.effective_message.reply_text(
            "Send me your LastFm username. use /cancel to cancel.")
        return LASTFM_USERNAME


# username command state
def linkLastFmUser(update: Update, context: CallbackContext) -> None:
    'save username in db'
    text = update.effective_message.text.strip()
    if len(text) > 25:
        update.message.reply_text(
            "Invalid username. Try again using /linkfm ")
        return ConversationHandler.END
    elif text.startswith('/'):
        update.message.reply_text(
            "Invalid username. Try again using /linkfm ")
        return ConversationHandler.END
    else:
        try:
            tg_id = str(update.message.from_user.id)
            is_user = DATABASE.getLastFmUser(tg_id)

            if is_user:
                update.message.reply_text(
                    "Your LastFm account is already linked. Use /unlinkfm to unlink.")
                return ConversationHandler.END
            else:
                DATABASE.addLastFmUser(tg_id, text)
                update.message.reply_text(
                    f"LastFM account linked. Now set a display name using /namefm command.")
                return ConversationHandler.END

        except Exception as ex:
            LOGGER.exception(ex)
            update.message.reply_text("Database Error")
            return ConversationHandler.END


def cancel(update, context):
    update.message.reply_text('Canceled.')
    return ConversationHandler.END


def unLinkFm(update: Update, context: CallbackContext) -> None:
    'add new user'
    if update.effective_chat.type == update.effective_chat.PRIVATE:
        tg_id = str(update.effective_user.id)
        try:
            is_user = DATABASE.getLastFmUser(tg_id)
            if is_user == None:
                update.message.reply_text(
                    "You haven't registered your account yet.")
                return ConversationHandler.END
            else:
                DATABASE.removeLastFmUser(tg_id)
                update.message.reply_text("Account successfully removed.")
                return ConversationHandler.END

        except Exception as ex:
            LOGGER.exception(ex)
            update.effective_message.reply_text("Database Error.")
            return ConversationHandler.END
    else:
        button = InlineKeyboardMarkup(
            [[InlineKeyboardButton(text="Contact me in pm", url=BOT_URL.format(context.bot.username))]])
        update.effective_message.reply_text(
            PM_MSG, reply_markup=button)
        return ConversationHandler.END


LASTFM_USERNAME = 1

LASTFM_HANDLER = ConversationHandler(
    entry_points=[CommandHandler('linkfm', getLastFmUserName)],
    states={LASTFM_USERNAME: [MessageHandler(
            Filters.text & ~Filters.command, linkLastFmUser)]},
    fallbacks=[CommandHandler('cancel', cancel)])

UNLINKFM_HANDLER = CommandHandler("unlinkfm", unLinkFm)


dispatcher.add_handler(LASTFM_HANDLER)
dispatcher.add_handler(UNLINKFM_HANDLER)

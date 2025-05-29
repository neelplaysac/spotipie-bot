from telegram import Update, InlineKeyboardButton, InlineKeyboardMarkup
from telegram.ext import ContextTypes, CommandHandler, ConversationHandler, filters, MessageHandler

from sp_bot import application, LOGGER
from sp_bot.modules.db import DATABASE

PM_MSG = 'Contact me in pm and use /linkfm to link or /unlinkfm to unlink your LastFm account.'
REG_MSG = 'Open the link below, to connect your LastFm account.'
BOT_URL = 't.me/{}'


async def getLastFmUserName(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    'ask user for usename'
    if update.effective_chat.type != "private":
        button = InlineKeyboardMarkup(
            [[InlineKeyboardButton(text="Link LastFm account.", url=BOT_URL.format(context.bot.username))]])
        await update.effective_message.reply_text(
            PM_MSG, reply_markup=button)
        return ConversationHandler.END
    else:
        await update.effective_message.reply_text(
            "Send me your LastFm username. use /cancel to cancel.")
        return LASTFM_USERNAME


# username command state
async def linkLastFmUser(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    'save username in db'
    text = update.effective_message.text.strip()
    if len(text) > 25:
        await update.message.reply_text(
            "Invalid username. Try again using /linkfm ")
        return ConversationHandler.END
    elif text.startswith('/'):
        await update.message.reply_text(
            "Invalid username. Try again using /linkfm ")
        return ConversationHandler.END
    else:
        try:
            tg_id = str(update.message.from_user.id)
            is_user = DATABASE.getLastFmUser(tg_id)

            if is_user:
                await update.message.reply_text(
                    "Your LastFm account is already linked. Use /unlinkfm to unlink.")
                return ConversationHandler.END
            else:
                DATABASE.addLastFmUser(tg_id, text)
                await update.message.reply_text(
                    f"LastFM account linked. Now set a display name using /namefm command.")
                return ConversationHandler.END

        except Exception as ex:
            LOGGER.exception(ex)
            await update.message.reply_text("Database Error")
            return ConversationHandler.END


async def cancel(update, context):
    await update.message.reply_text('Canceled.')
    return ConversationHandler.END


async def unLinkFm(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    'add new user'
    if update.effective_chat.type == "private":
        tg_id = str(update.effective_user.id)
        try:
            is_user = DATABASE.getLastFmUser(tg_id)
            if is_user == None:
                await update.message.reply_text(
                    "You haven't registered your account yet.")
                return ConversationHandler.END
            else:
                DATABASE.removeLastFmUser(tg_id)
                await update.message.reply_text("Account successfully removed.")
                return ConversationHandler.END

        except Exception as ex:
            LOGGER.exception(ex)
            await update.effective_message.reply_text("Database Error.")
            return ConversationHandler.END
    else:
        button = InlineKeyboardMarkup(
            [[InlineKeyboardButton(text="Contact me in pm", url=BOT_URL.format(context.bot.username))]])
        await update.effective_message.reply_text(
            PM_MSG, reply_markup=button)
        return ConversationHandler.END


LASTFM_USERNAME = 1

LASTFM_HANDLER = ConversationHandler(
    entry_points=[CommandHandler('linkfm', getLastFmUserName)],
    states={LASTFM_USERNAME: [MessageHandler(
            filters.TEXT & ~filters.COMMAND, linkLastFmUser)]},
    fallbacks=[CommandHandler('cancel', cancel)])

UNLINKFM_HANDLER = CommandHandler("unlinkfm", unLinkFm)


application.add_handler(LASTFM_HANDLER)
application.add_handler(UNLINKFM_HANDLER)

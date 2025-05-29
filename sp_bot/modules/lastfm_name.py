from telegram import Update, InlineKeyboardButton, InlineKeyboardMarkup
from telegram.ext import ContextTypes, CommandHandler, ConversationHandler, filters, MessageHandler

from sp_bot import application, LOGGER
from sp_bot.modules.db import DATABASE


PM_MSG = 'Contact me in pm to change your display name.'
REG_MSG = 'You need to link your LastFm account first. use /lastfm in pm to get started.'
BOT_URL = 't.me/{}'


# /username command
async def getLastFmUserData(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    'ask user for usename'
    if update.effective_chat.type != "private":
        button = InlineKeyboardMarkup(
            [[InlineKeyboardButton(text="Change display name", url=BOT_URL.format(context.bot.username))]])
        await update.effective_message.reply_text(
            PM_MSG, reply_markup=button)
        return ConversationHandler.END
    await update.effective_message.reply_text(
        "Send me a username (max 15 characters)")
    return LASTFM_DISPLAY_NAME


# username command state
async def setLastFmUserData(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    'save username in db'
    text = update.effective_message.text.strip()
    if len(text) > 15:
        await update.message.reply_text(
            "Invalid username. Try again using /namefm ")
        return ConversationHandler.END
    elif text.startswith('/'):
        await update.message.reply_text(
            "Invalid username. Try again using /namefm ")
        return ConversationHandler.END
    else:
        try:
            tg_id = str(update.message.from_user.id)
            is_user = DATABASE.getLastFmUser(tg_id)

            if is_user == None:
                await update.message.reply_text(REG_MSG)
                return ConversationHandler.END
            else:
                DATABASE.updateLastFmData(tg_id, text)
                await update.message.reply_text(
                    f"Username updated to {text}. Use /last to share lastfm song status.")
                return ConversationHandler.END

        except Exception as ex:
            LOGGER.exception(ex)
            await update.message.reply_text("Database Error")
            return ConversationHandler.END


async def cancel(update, context):
    await update.message.reply_text('Canceled.')
    return ConversationHandler.END


LASTFM_DISPLAY_NAME = 1
NAMEFM_HANDLER = ConversationHandler(
    entry_points=[CommandHandler('namefm', getLastFmUserData)],
    states={LASTFM_DISPLAY_NAME: [MessageHandler(
            filters.TEXT & ~filters.COMMAND, setLastFmUserData)]},
    fallbacks=[CommandHandler('cancel', cancel)])

application.add_handler(NAMEFM_HANDLER)

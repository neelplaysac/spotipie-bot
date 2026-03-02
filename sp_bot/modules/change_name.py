from telegram import Update, InlineKeyboardButton, InlineKeyboardMarkup
from telegram.ext import ContextTypes, CommandHandler, ConversationHandler, MessageHandler, filters

from sp_bot import application, LOGGER
from sp_bot.modules.db import DATABASE
from sp_bot.modules.misc.cooldown import cooldown


PM_MSG = 'Contact me in pm to change your username (*this will not change your telegram username).'
REG_MSG = 'You need to register first. use /register to get started.'
BOT_URL = 't.me/{}'


# /username command
@cooldown(seconds=3)
async def getUsername(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    'ask user for usename'
    if update.effective_chat.type != "private":
        button = InlineKeyboardMarkup(
            [[InlineKeyboardButton(text="Change username", url=BOT_URL.format(context.bot.username))]])
        await update.effective_message.reply_text(
            PM_MSG, reply_markup=button)
        return ConversationHandler.END
    await update.effective_message.reply_text(
        "Send me a username (max 15 characters)")
    return USERNAME


# username command state
async def setUsername(update: Update, context: ContextTypes.DEFAULT_TYPE) -> None:
    'save username in db'
    text = update.effective_message.text.strip()
    if not text or len(text) > 15 or text.startswith('/'):
        await update.message.reply_text(
            "Invalid username. Must be 1-15 characters and not start with /. Try again using /name")
        return ConversationHandler.END
    else:
        try:
            tg_id = str(update.message.from_user.id)
            is_user = DATABASE.fetchData(tg_id)

            if is_user is None:
                await update.message.reply_text(REG_MSG)
                return ConversationHandler.END
            else:
                DATABASE.updateData(tg_id, text)
                await update.message.reply_text(f"Username updated to {text}")
                return ConversationHandler.END

        except Exception as ex:
            LOGGER.exception(ex)
            await update.message.reply_text("Database Error")
            return ConversationHandler.END


async def cancel(update, context):
    await update.message.reply_text('Canceled.')
    return ConversationHandler.END


USERNAME = 1
USERNAME_HANDLER = ConversationHandler(
    entry_points=[CommandHandler('name', getUsername)],
    states={USERNAME: [MessageHandler(
            filters.TEXT & ~filters.COMMAND, setUsername)]},
    fallbacks=[CommandHandler('cancel', cancel)],
    conversation_timeout=60)

application.add_handler(USERNAME_HANDLER)

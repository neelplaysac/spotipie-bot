import re
from telegram import Update, InlineKeyboardMarkup, InlineKeyboardButton
from telegram.ext import ContextTypes, CommandHandler, ConversationHandler, CallbackQueryHandler

from sp_bot import application, LOGGER
from sp_bot.modules.db import DATABASE

BOT_URL = 't.me/{}'
REG_MSG = 'Use /register to connect your spotify account or /linkfm to connect your LastFm account to get started.'


async def style(update: Update, context: ContextTypes.DEFAULT_TYPE):
    'change background style'

    if update.effective_chat.type == "private":
        try:
            tg_id = str(update.message.from_user.id)
            is_user = DATABASE.fetchData(tg_id)
            lastfm_user = DATABASE.getLastFmUser(tg_id)

            if is_user == None and lastfm_user == None:
                await update.message.reply_text(REG_MSG)
                return ConversationHandler.END

            keyboard = [[]]

            if is_user != None:
                curr_style = "Blur" if is_user["style"] == "blur" else "Black"
                keyboard[0].append(InlineKeyboardButton(
                    f"Current Style: {curr_style}", callback_data=curr_style))

            if lastfm_user != None:
                if "counter" not in lastfm_user:
                    DATABASE.toggleCounter(tg_id, "on")
                    lastfm_user["counter"] = "on"
                counter = lastfm_user["counter"]
                counterStatus = "Disabled" if counter == "off" else "Enabled"
                keyboard[0].append(InlineKeyboardButton(
                    f"LastFm Scrobbles: {counterStatus}", callback_data=counterStatus))

            await update.effective_message.reply_text(
                "Settings : ", reply_markup=InlineKeyboardMarkup(keyboard))

        except Exception as ex:
            LOGGER.exception(ex)
            await update.message.reply_text("Database Error")
    else:
        button = InlineKeyboardMarkup(
            [[InlineKeyboardButton(text='Contact in pm', url=BOT_URL.format(context.bot.username))]])
        await update.effective_message.reply_text(
            "Contact me in pm and use /style command to change background style.", reply_markup=button)

    return ConversationHandler.END


async def button(update: Update, _: ContextTypes.DEFAULT_TYPE) -> None:
    query = update.callback_query
    await query.answer()

    style = {"Black": "blur", "Blur": "black"}
    counter = {"Enabled": "off", "Disabled": "on"}
    try:
        tg_id = str(update.effective_user.id)
        if query.data in style:
            updatedValue = style[query.data]
            DATABASE.updateStyle(tg_id, updatedValue)
            updatedField = "Style updated to"
        if query.data in counter:
            updatedValue = counter[query.data]
            DATABASE.toggleCounter(tg_id, updatedValue)
            updatedField = "Scrobble Counter is now"

    except Exception as ex:
        LOGGER.exception(ex)

    await query.edit_message_text(text=f"{updatedField}: {updatedValue}")


STYLE_HANDLER = CommandHandler("style", style)
application.add_handler(STYLE_HANDLER)

BUTTON_CALLBACK = CallbackQueryHandler(button)
application.add_handler(BUTTON_CALLBACK)
import httpx
from uuid import uuid4

from telegram import Update, InlineKeyboardMarkup, InlineKeyboardButton, InlineQueryResultCachedPhoto, InlineQueryResultsButton
from telegram.ext import ContextTypes, InlineQueryHandler, ConversationHandler

from sp_bot import application, TEMP_CHANNEL, LOGGER
from sp_bot.modules.misc.cook_image import drawImage
from sp_bot.modules.db import DATABASE
from sp_bot.modules.misc.request_spotify import SPOTIFY


async def inlineNowPlaying(update: Update, context: ContextTypes.DEFAULT_TYPE):
    'inline implementation of nowPlaying() function along with exception handling for new users'
    try:
        tg_id = str(update.inline_query.from_user.id)
        is_user = DATABASE.fetchData(tg_id)
        if is_user is None:
            await update.inline_query.answer(
                [], button=InlineQueryResultsButton(text="You need to register first.", start_parameter='register'), cache_time=0)
            return ConversationHandler.END
        elif is_user["username"] == 'User':
            await update.inline_query.answer(
                [], button=InlineQueryResultsButton(text="You need to set a username.", start_parameter='username'), cache_time=0)
            return ConversationHandler.END
        elif is_user['token'] == '00000':
            await update.inline_query.answer(
                [], button=InlineQueryResultsButton(text="Registration error, please click here to fix.", start_parameter='token'), cache_time=0)
            return ConversationHandler.END
        else:
            token = is_user["token"]
            r = await SPOTIFY.getCurrentyPlayingSong(token)
    except Exception as ex:
        LOGGER.exception(ex)
        return

    try:
        user_profile_photos = await context.bot.get_user_profile_photos(tg_id, limit=1)
        if user_profile_photos.photos:
            pfp_file = await context.bot.get_file(user_profile_photos.photos[0][0].file_id)
            async with httpx.AsyncClient(follow_redirects=True) as client:
                pfp = await client.get(pfp_file.file_path)
        else:
            pfp = None
    except:
        pfp = None

    try:
        if r.status_code == 204 or not r.content:
            await update.inline_query.answer(
                [], button=InlineQueryResultsButton(text="You're not listening to anything.", start_parameter='notlistening'), cache_time=0)
            return

        res = r.json()
        if res['currently_playing_type'] == 'ad':
            await update.inline_query.answer(
                [], button=InlineQueryResultsButton(text="You are listening to ads.", start_parameter='ads'), cache_time=0)
        elif res['currently_playing_type'] == 'track':
            username = is_user["username"]
            style = is_user["style"]
            image = await drawImage(res, username, pfp, style)
            button = InlineKeyboardButton(
                text="Play on Spotify", url=res['item']['external_urls']['spotify'])
            temp = await context.bot.send_photo(TEMP_CHANNEL, photo=image)
            photo = temp.photo[1].file_id
            await temp.delete()

            await update.inline_query.answer(
                [
                    InlineQueryResultCachedPhoto(
                        id=uuid4(),
                        photo_file_id=photo,
                        reply_markup=InlineKeyboardMarkup([[button]])
                    )
                ], cache_time=0
            )
        else:
            await update.inline_query.answer(
                [], button=InlineQueryResultsButton(text="Not sure what you're listening to.", start_parameter='notsure'), cache_time=0)
    except Exception as ex:
        LOGGER.exception(ex)
        await update.inline_query.answer(
            [], button=InlineQueryResultsButton(text="You're not listening to anything.", start_parameter='notlistening'), cache_time=0)


INLINE_QUERY_HANDLER = InlineQueryHandler(inlineNowPlaying)
application.add_handler(INLINE_QUERY_HANDLER)
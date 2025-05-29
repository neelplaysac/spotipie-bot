from telegram import Update
from telegram.ext import Updater, CallbackContext, CommandHandler, ConversationHandler

from sp_bot import dispatcher
from sp_bot.modules.db import DATABASE


def details(update: Update, context: CallbackContext):
    'returns the number of registered users, devs only'
    user = str(update.message.from_user.id)
    if user in ['394012198', '259972454']:

        total_users = DATABASE.countAll()
        total_lastfm_users = DATABASE.countAllLastFm()

        usersStats = DATABASE.aggregateUsers()
        lastFmUsersStats = DATABASE.aggregateLastFmUsers()

        blur_count, counter_on, counter_off = 0, 0, 0

        for stat in usersStats:
            if stat['_id'] == 'blur':
                blur_count += stat['count']

        for counter in lastFmUsersStats:
            if counter['_id'] == 'off':
                counter_off += counter['count']
            if counter['_id'] == 'on':
                counter_on += counter['count']

        result = [
            f'Spotify Users: {total_users}',
            f'LastFm Users: {total_lastfm_users}',
            '',
            f'Blur style: {blur_count}, Black Style: {total_users-blur_count}',
            f'On: {counter_on}, Off: {counter_off}',
            f'Others: {total_lastfm_users-counter_on-counter_off}'
        ]

        update.message.reply_text("\n".join(result))
    else:
        update.effective_message.reply_text(
            "Only bot admins can use this command ^-^")

    return ConversationHandler


def statss(update: Update, context: CallbackContext):
    'returns the number of registered users, devs only'
    user = str(update.message.from_user.id)
    if user in ['394012198', '259972454']:
        total_users = DATABASE.countAll()
        update.message.reply_text(total_users)
    else:
        update.effective_message.reply_text(
            "Only bot admins can use this command ^-^")

    return ConversationHandler


STATS_HANDLER = CommandHandler("statss", statss)
dispatcher.add_handler(STATS_HANDLER)
DETAILS_HANDLER = CommandHandler("details", details)
dispatcher.add_handler(DETAILS_HANDLER)

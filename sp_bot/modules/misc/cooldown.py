import time
from functools import wraps
from telegram import Update
from telegram.ext import ContextTypes


# In-memory store: {user_id: {handler_name: last_invoked_timestamp}}
_cooldowns: dict[str, dict[str, float]] = {}


def cooldown(seconds: int = 5):
    """
    Decorator that enforces a per-user cooldown on a handler.
    If the user invokes the command again within `seconds`, 
    it replies with a wait message and skips execution.
    """
    def decorator(func):
        @wraps(func)
        async def wrapper(update: Update, context: ContextTypes.DEFAULT_TYPE, *args, **kwargs):
            # Get user ID from either message or inline query
            if update.message:
                user_id = str(update.message.from_user.id)
            elif update.inline_query:
                user_id = str(update.inline_query.from_user.id)
            elif update.callback_query:
                user_id = str(update.callback_query.from_user.id)
            else:
                return await func(update, context, *args, **kwargs)

            handler_name = func.__name__
            now = time.monotonic()

            if user_id not in _cooldowns:
                _cooldowns[user_id] = {}

            last_used = _cooldowns[user_id].get(handler_name, 0)
            elapsed = now - last_used

            if elapsed < seconds:
                remaining = round(seconds - elapsed)
                if remaining < 1:
                    remaining = 1

                if update.message:
                    await update.message.reply_text(
                        f"Please wait {remaining}s before using this again.")
                elif update.inline_query:
                    await update.inline_query.answer(
                        [], switch_pm_text=f"Slow down! Wait {remaining}s.",
                        switch_pm_parameter="cooldown", cache_time=0)
                elif update.callback_query:
                    await update.callback_query.answer(
                        f"Please wait {remaining}s.", show_alert=False)
                return

            _cooldowns[user_id][handler_name] = now
            return await func(update, context, *args, **kwargs)

        return wrapper
    return decorator

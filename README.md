# 🎵 Spotipie bot

Your music, turned into listening cards for Telegram. Share what's playing on Spotify, catch up with your Last.fm scrobbles, and bring your listening activity into the conversation.

![Go](https://img.shields.io/badge/Code-Go-informational?style=flat&logo=go&logoColor=white&color=00ADD8)
![Gin](https://img.shields.io/badge/Framework-Gin-informational?style=flat&logo=gin&logoColor=white&color=008ECF)
![Telegram](https://img.shields.io/badge/Platform-Telegram-informational?style=flat&logo=telegram&logoColor=white&color=26A5E4)
![Spotify](https://img.shields.io/badge/Music-Spotify-informational?style=flat&logo=spotify&logoColor=white&color=1DB954)
![Last.fm](https://img.shields.io/badge/Scrobbles-Last.fm-informational?style=flat&logo=lastdotfm&logoColor=white&color=D51007)
![MongoDB](https://img.shields.io/badge/Database-MongoDB-informational?style=flat&logo=mongodb&logoColor=white&color=47A248)
![Docker](https://img.shields.io/badge/Containers-Docker-informational?style=flat&logo=docker&logoColor=white&color=2496ED)
![Sentry](https://img.shields.io/badge/Monitoring-Sentry-informational?style=flat&logo=sentry&logoColor=white&color=362D59)

Originally built in Python and now written in Go, Spotipie combines Spotify and Last.fm with customizable listening cards. Album artwork, track details, and your chosen display name come together in an image you can share directly in Telegram.

![Song Screenshot](screenshots/lagrima.png)
![Song Screenshot](screenshots/die-for-you.png)

🎧 Share your current Spotify track without leaving the chat.

📻 Show what you've been listening to with Last.fm scrobbles and play counts.

🎨 Make your cards your own with custom display names and different styles.

💬 Share listening cards through Telegram inline queries.

🔐 Connect Spotify through an authorization flow tied to your Telegram account, with expiring, single-use sessions.

## Available Commands

- `/now` - Share currently playing Spotify track
- `/register` - Link Spotify account
- `/name` - Set Spotify display name
- `/style` - Change background style
- `/unregister` - Unlink Spotify account
- `/linkfm` - Link Last.fm account
- `/namefm` - Set Last.fm display name
- `/last` - Share recent Last.fm track
- `/unlinkfm` - Unlink Last.fm account

## Alternatives

[SpotifyNowBot](https://github.com/notdedsec/SpotifyNow): Python Telegram bot to share what you're listening to on Spotify.

## Support

If you're enjoying Spotipie bot, drop a ⭐️ on the repo!

<a href="https://www.buymeacoffee.com/neelplaysac" target="_blank"><img src="https://cdn.buymeacoffee.com/buttons/v2/default-yellow.png" alt="Buy Me A Coffee" style="height: 60px !important;width: 217px !important;" ></a>

## Star History

<a href="https://www.star-history.com/?repos=neelplaysac%2Fspotipie-bot">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/chart?repos=neelplaysac/spotipie-bot&type=date&theme=dark&legend=top-left" />
   <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/chart?repos=neelplaysac/spotipie-bot&type=date&legend=top-left" />
   <img alt="Star History Chart" src="https://api.star-history.com/chart?repos=neelplaysac/spotipie-bot&type=date&legend=top-left" />
 </picture>
</a>

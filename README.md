# Spotipie Bot

A Telegram bot that allows users to share their currently playing music from both Spotify and Last.fm with beautifully styled images and track information.

## Features

### Spotify Integration

- **Now Playing**: Share your currently playing Spotify track with a custom styled image
- **Account Linking**: Secure OAuth2 integration with Spotify API
- **Inline Queries**: Use `@spotipiebot` to share music in any chat
- **Custom Styling**: Choose between blur or black background styles
- **Display Names**: Set personalized usernames for your music shares

### Last.fm Integration

- **Scrobble Sharing**: Share recent tracks from your Last.fm profile
- **Play Count**: Display how many times you've listened to a track
- **Recent Activity**: View and share your listening history
- **Custom Display Names**: Separate display names for Last.fm posts

### Visual Features

- **Dynamic Images**: Automatically generated images with album art, track info, and user details
- **Profile Pictures**: Incorporates user's Telegram profile picture into the generated image
- **Multiple Styles**: Customizable background styles for different aesthetics

### User Management

- **Privacy Focused**: All interactions happen in private messages for security
- **Easy Setup**: Simple step-by-step registration process
- **Account Management**: Easy linking/unlinking of both Spotify and Last.fm accounts
- **Database Storage**: Persistent user data and preferences

## Available Commands

- `/now` - Share currently playing Spotify track
- `/last` - Share recent Last.fm track
- `/register` - Link Spotify account
- `/linkfm` - Link Last.fm account
- `/name` - Set Spotify display name
- `/namefm` - Set Last.fm display name
- `/style` - Change background style
- `/unregister` - Unlink Spotify account
- `/unlinkfm` - Unlink Last.fm account

## Technologies Used

- **Python** - Core programming language
- **python-telegram-bot** - Telegram Bot API wrapper
- **MongoDB** - Database for user data and preferences
- **Spotify Web API** - Music streaming service integration
- **Last.fm API** - Music scrobbling service integration
- **Pillow (PIL)** - Image processing and generation
- **Requests** - HTTP client for API calls
- **OAuth2** - Secure authentication flow

## Live Bot

You can find the bot running on Telegram as [@Spotipiebot](https://t.me/Spotipiebot).

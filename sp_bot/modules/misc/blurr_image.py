import math
import requests
from PIL import Image, ImageDraw, ImageFont, ImageFilter, ImageEnhance
from io import BytesIO

from sp_bot.modules.misc import Fonts
from sp_bot import LOGGER


def truncate(text, font, limit):
    edited = True if font.getsize(text)[0] > limit else False
    while font.getsize(text)[0] > limit:
        text = text[:-1]
    if edited:
        return(text.strip() + '..')
    else:
        return(text.strip())


def checkUnicode(text):
    return text == str(text.encode('utf-8'))[2:-1]


def blurrImage(res, username, pfp, scrobbles):
    last_fm_temp_image = 'https://lastfm.freetls.fastly.net/i/u/300x300/2a96cbd8b46e442fc41c2b86b821562f.png'
    last_fm_logo = 'https://files.catbox.moe/098341.png'
    # last_fm_logo = 'https://files.catbox.moe/ymirt1.png'

    track = res.json()['recenttracks']['track'][0]

    artists = track['artist']['#text']
    albumname = track['album']['#text']
    songname = track['name']
    cover_url = track['image'][3]['#text']
    # coverart = 'https://files.catbox.moe/kux4sq.jpeg' if cover_url == '' or 'https://lastfm.freetls.fastly.net/i/u/300x300/2a96cbd8b46e442fc41c2b86b821562f.png' else cover_url
    coverart = cover_url
    if cover_url == '':
        coverart = last_fm_logo
    if cover_url == last_fm_temp_image:
        coverart = last_fm_logo

    song_url = track['url']
    tense = 'is' if '@attr' in track else 'was'

    # background object
    canvas = Image.new("RGB", (600, 250), (18, 18, 18))
    draw = ImageDraw.Draw(canvas, "RGBA")

    # album art
    try:
        link = coverart
        r = requests.get(link)
        art = Image.open(BytesIO(r.content))
        art.thumbnail((200, 200), Image.ANTIALIAS)

        bg = Image.open(BytesIO(r.content))
        bg = bg.resize((600, 600))
        bg = bg.crop((0, 175, 600, 425))

        blurr = bg.filter(ImageFilter.GaussianBlur(radius=25))

        blurr_dark = ImageEnhance.Brightness(blurr)
        blurr_dark = blurr_dark.enhance(0.9)

        blurr_cont = ImageEnhance.Contrast(blurr_dark)
        blurr_cont = blurr_cont.enhance(0.8)

        canvas.paste(blurr_cont, (0, 0))
        canvas.paste(art, (25, 25))

    except Exception as ex:
        LOGGER.exception(ex)

    # profile pic
    if pfp:
        profile_pic = Image.open(BytesIO(pfp.content))
        profile_pic.thumbnail((52, 52), Image.ANTIALIAS)
        canvas.paste(profile_pic, (523, 25))

    # set font sizes
    open_sans = ImageFont.truetype(Fonts.OPEN_SANS, 24)
    # open_bold = ImageFont.truetype(Fonts.OPEN_BOLD, 23)
    poppins = ImageFont.truetype(Fonts.POPPINS, 26)
    arial = ImageFont.truetype(Fonts.ARIAL, 26)
    arial23 = ImageFont.truetype(Fonts.ARIAL, 23)
    bold = ImageFont.truetype(Fonts.OPEN_BOLD, 22)

    # assign fonts
    songfont = poppins if checkUnicode(songname) else arial
    artistfont = open_sans if checkUnicode(artists) else arial23
    albumfont = open_sans if checkUnicode(albumname) else arial23

    # draw text on canvas
    white = '#ffffff'
    # draw.text((248, 18), truncate(username, poppins, 250),
    #           fill=white, font=poppins)
    # draw.text((248, 53), f"{tense} listening to",
    #           fill=white, font=open_sans)
    # draw.text((248, 132), truncate(songname, songfont, 315),
    #           fill=white, font=songfont)
    # draw.text((248, 167), truncate(artists, artistfont, 315),
    #           fill=white, font=artistfont)
    # draw.text((248, 197), truncate(albumname, albumfont, 315),
    #           fill=white, font=albumfont)

    '''
    draw.text((248, 18), truncate(username, poppins, 250),
              fill=white, font=poppins)
    draw.text((248, 53), f"{tense} listening to",
              fill=white, font=open_sans)
    draw.text((248, 115), truncate(songname, songfont, 315),
              fill=white, font=songfont)
    draw.text((248, 150), truncate(artists, artistfont, 315),
              fill=white, font=artistfont)
    draw.text((248, 180), truncate(albumname, albumfont, 315),
              fill=white, font=albumfont)

    '''
    draw.text((248, 18), truncate(username, poppins, 250),
              fill=white, font=poppins)
    draw.text((248, 53), f"{tense} listening to",
              fill=white, font=open_sans)
    draw.text((248, 105), truncate(songname, songfont, 315),
              fill=white, font=songfont)
    draw.text((248, 140), truncate(artists, artistfont, 315),
              fill=white, font=artistfont)
    draw.text((248, 170), truncate(albumname, albumfont, 315),
              fill=white, font=albumfont)
    draw.rectangle([(578, 221), (248, 223)], fill='#B3B3B3')

    if scrobbles != "off":
        w = {1: 45, 2: 58, 3: 73, 4: 86, 5: 100, 6: 111}[len(scrobbles)]
        draw.rounded_rectangle([(18, 200), (w, 230)],
                               radius=13, fill='#121212')
        draw.text((26, 200), scrobbles, fill='#ffffff', font=bold)

    # return canvas
    image = BytesIO()
    canvas.save(image, 'JPEG', quality=200)
    image.seek(0)
    return image

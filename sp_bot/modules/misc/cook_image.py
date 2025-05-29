import math
import requests
from PIL import Image, ImageDraw, ImageFont, ImageFilter, ImageEnhance
from io import BytesIO

from sp_bot.modules.misc import Fonts


def truncate(text, font, limit):
    try:
        bbox = font.getbbox(text)
        text_width = bbox[2] - bbox[0]
    except AttributeError:
        text_width = font.getsize(text)[0]

    edited = True if text_width > limit else False
    while True:
        try:
            bbox = font.getbbox(text)
            text_width = bbox[2] - bbox[0]
        except AttributeError:
            text_width = font.getsize(text)[0]

        if text_width <= limit:
            break
        text = text[:-1]

    if edited:
        return (text.strip() + '..')
    else:
        return (text.strip())


def checkUnicode(text):
    return text == str(text.encode('utf-8'))[2:-1]


def drawImage(res, username, pfp, style):
    songname = res['item']['name']
    albumname = res['item']['album']['name']
    totaltime = res['item']['duration_ms']
    currtime = res['progress_ms']
    coverart = res['item']['album']['images'][1]['url']
    song_url = res['item']['external_urls']['spotify']
    artists = ', '.join([x['name']
                         for x in res['item']['artists']])

    # background object
    canvas = Image.new("RGB", (600, 250), (18, 18, 18))
    draw = ImageDraw.Draw(canvas)

    # album art
    try:
        link = coverart
        r = requests.get(link)

        if style == "blur":
            bg = Image.open(BytesIO(r.content))
            bg = bg.resize((600, 600))
            bg = bg.crop((0, 175, 600, 425))

            blurr = bg.filter(ImageFilter.GaussianBlur(radius=25))

            blurr_dark = ImageEnhance.Brightness(blurr)
            blurr_dark = blurr_dark.enhance(0.9)

            blurr_cont = ImageEnhance.Contrast(blurr_dark)
            blurr_cont = blurr_cont.enhance(0.8)

            canvas.paste(blurr_cont, (0, 0))

        art = Image.open(BytesIO(r.content))
        art.thumbnail((200, 200), Image.Resampling.LANCZOS)
        canvas.paste(art, (25, 25))
    except Exception as ex:
        print(ex)

    # profile pic
    if pfp:
        profile_pic = Image.open(BytesIO(pfp.content))
        profile_pic.thumbnail((52, 52), Image.Resampling.LANCZOS)
        canvas.paste(profile_pic, (523, 25))

    # set font sizes
    open_sans = ImageFont.truetype(Fonts.OPEN_SANS, 24)
    # open_bold = ImageFont.truetype(Fonts.OPEN_BOLD, 23)
    poppins = ImageFont.truetype(Fonts.POPPINS, 26)
    arial = ImageFont.truetype(Fonts.ARIAL, 26)
    arial23 = ImageFont.truetype(Fonts.ARIAL, 23)

    # assign fonts
    songfont = poppins if checkUnicode(songname) else arial
    artistfont = open_sans if checkUnicode(artists) else arial23
    albumfont = open_sans if checkUnicode(albumname) else arial23

    # draw text on canvas
    white = '#ffffff'
    draw.text((248, 18), truncate(username, poppins, 250),
              fill=white, font=poppins)
    draw.text((248, 53), "is listening to",
              fill=white, font=open_sans)
    draw.text((248, 115), truncate(songname, songfont, 315),
              fill=white, font=songfont)
    draw.text((248, 150), truncate(artists, artistfont, 315),
              fill=white, font=artistfont)
    draw.text((248, 180), truncate(albumname, albumfont, 315),
              fill=white, font=albumfont)    # draw progress bar on canvas
    draw.rectangle([(248, 222), (578, 224)],
                   fill='#404040')
    draw.rectangle([(248, 222), (248 + (currtime / totaltime * 330), 224)],
                   fill='#B3B3B3')

    # return canvas
    image = BytesIO()
    canvas.save(image, 'JPEG', quality=200)
    image.seek(0)
    return image

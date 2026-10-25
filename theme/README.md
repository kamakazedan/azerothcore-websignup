# Your own look

The top of the page (the part behind the realm's name and the sign-up
form) uses files from your theme folder. Drop files in and the site picks
them up within a few seconds; there's no need to restart anything. Every
file is optional: with no picture of your own, the top of the page shows
the built-in one, a lone knight in a blizzard on a frozen plain (made by
[`art/background.py`](../art/background.py), free to use under CC0).

If you run the site with `docker compose`, the theme folder is this one.
In Portainer it's the `theme` folder inside `WEBSIGNUP_FILES`.

| File | What it's for |
|---|---|
| `background.jpg` | The picture behind the name and the form. `.png`, `.webp` and `.avif` work too. Use a wide picture, at least 1920 pixels across. |
| `background-phone.jpg` | A picture for phones, if the wide one crops badly on a tall screen. Usually a portrait crop of the same picture. |
| `background.mp4` | A looping video instead of the picture. It's muted and starts by itself. `.webm` works too; add both and browsers pick the one they play best. `background.jpg` shows while it loads, and to anyone who has turned animation off. |
| `lich-king.1920x1080.mp4` | Any other video is a *scene*: see [Videos](#videos) below. |
| `page.jpg` | A picture behind the rest of the page, dimmed so the text stays readable. |
| `particle.png` | An image that floats across the top: a rock, a shard, an ember, a snowflake. Add `particle-2.png`, `particle-3.png` and so on (up to 12) for variety. Use PNG or WebP with a transparent background, or SVG. Animated GIFs and WebPs animate too. |
| `logo.svg` | Replaces the crystal in the top corner. `.png` and `.webp` work too. It's shown 40 pixels tall. |
| `favicon.png` | Replaces the browser tab icon. `.svg` and `.ico` work too. |
| `theme.conf` | Settings: see below. |
| `theme.css` | Your own CSS, loaded last, for anything else. |

File names must be plain: letters, digits, `-`, `_` and `.`, no spaces.

## Videos

Every video in the folder is a scene, named by the part of its file name
before the size. Each visit shows one scene, picked at random, and can
move from one to the next (see `video_change` below).

Give each scene several sizes and the page picks the one that suits the
screen, so phones aren't sent a 30 MB 4K file:

```
lich-king.720x1280.mp4      phones held upright
lich-king.1920x1080.mp4     most computers, and phones on their side
lich-king.3840x2160.mp4     only on screens wider than about 2900 pixels
lich-king.jpg               shown while the video loads (optional)
```

The size goes after the name, as `name.WIDTHxHEIGHT.mp4` (a `-` works
in place of the `.` too). A scene can have any sizes, or just one, and a
video with no size in its name is used for every screen. A `.webm` of
the same name and size is offered alongside the MP4. A picture with the
scene's name shows before the video plays; without one, `background.jpg`
does.

However many scenes there are, at most two videos are ever loaded at
once: the one showing and the next one, and each is emptied out of the
browser's memory as soon as it has faded away.

Nobody who has turned animation off in their system settings gets a
video, and neither does anyone whose browser is set to save data. The
video stops while it's scrolled out of sight.

## Settings

Copy `theme.conf.example` into your theme folder as `theme.conf`. It
explains every setting:

| Setting | Default | What it does |
|---|---|---|
| `background_focus` | `50% 40%` | Which part of the picture stays in view when a screen crops it: across, then down |
| `background_shade` | `0.35` | How much to darken the picture behind the words, `0` to `0.9` |
| `background_motion` | `none` | `zoom` slowly pushes in and out, `drift` slowly sways side to side |
| `page_shade` | `0.75` | How much to darken `page.jpg` |
| `particles` | `rise` with particle images, otherwise `none` | `rise`, `fall`, `float` or `none` |
| `particle_count` | `18` | How many, `0` to `60` |
| `particle_size` | `12-36` | Size in pixels, smallest to largest |
| `particle_speed` | `1` | `0.5` is half as fast, `2` twice as fast |
| `particle_opacity` | `0.9` | How solid the closest ones are |
| `particle_spin` | `yes` | Whether they turn as they move |
| `particle_target` | `none` | Rising particles drift towards this point across the screen (`0` to `100`) and shrink into it |
| `video_scene` | `random` | Which scene each visit starts on: `random`, or a scene's name like `lich-king` |
| `video_change` | `0` | Seconds before fading to the next scene (`10` to `3600`); `0` stays on one |
| `video_preload` | `50%` | How early the next scene starts loading, as a share of `video_change`, so it fades in on time. `0` waits until it's due |
| `video_flip` | `none` | Scenes to show mirrored left to right, like `lich-king, night-elf-archer`, or `all` |
| `video_quality` | `auto` | The largest size to send, by its shorter side: `720`, `1080`, `2160`. `auto` sends whatever suits the screen. A 4K video uses about four times the memory of a 1080p one, so `1080` keeps the page light everywhere |

How the particles move:

- `rise`: up from the bottom, fading in and out. With `particle_target` set
  they also drift in towards that point and shrink, as if pulled into
  something far away.
- `fall`: down from the top, like snow, ash or embers.
- `float`: they hang in the air and bob gently, like rocks hovering.

Particles without images of your own are drawn as small lit shards.

## If something doesn't show

The site's log (in Portainer, the container's **Logs**) says what it
loaded each time the folder changes, like `theme loaded background=true
particles=rise "particle images"=3`, and what's wrong with anything else:

- a mistyped setting, which keeps its default
- a file it doesn't use, and why: a name it doesn't know (`bg.jpg`, or
  `background.jpg.jpg` when Windows hides the real extension), a type it
  can't use there, or a second background where only one is used
- a file it isn't allowed to read. The site doesn't run as root, so the
  files must be readable by everyone: `chmod -R a+rX` on the folder fixes it

Changes show on the next page load, a few seconds after you save. The
files' addresses change whenever the folder does, so browsers don't hang on
to old pictures.

## Tips

- Shrink big pictures before you add them: a 1920 or 2560 pixel wide JPEG
  or WebP at 80% quality is plenty. Visitors on phones download them too.
- For video, an H.264 MP4 plays everywhere. A WebM next to it is often
  smaller.
- If the name or the form gets hard to read over a busy picture, raise
  `background_shade`, or move the busy part out of the way with
  `background_focus`.
- Use pictures you made or have permission to use. Fan art and game
  screenshots belong to the people and companies who made them.

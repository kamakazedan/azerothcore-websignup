# The built-in background

`background.py` paints the picture the top of the page shows when the
theme folder has no `background.jpg` of its own: a blizzard over a frozen
plain, ice spires on the horizon and a cold glow behind the clouds.

It's drawn entirely by the script, from polygons and noise: nothing is
traced, copied or taken from a game, and no outside images are used. The
two pictures it writes, `internal/site/static/img/background.jpg`
(2560×1440) and `background-phone.jpg` (1080×1920, for phones held
upright), are released under
[CC0 1.0](https://creativecommons.org/publicdomain/zero/1.0/): use them
for anything, with no need to credit anyone.

To paint them again, or a different storm:

```bash
pip install numpy pillow
python3 art/background.py            # the pictures that ship
python3 art/background.py --seed 7   # another seed, another storm
```

Then rebuild the image: the pictures are built into the binary.

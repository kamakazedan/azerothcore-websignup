# The hero scene

The sky and the land behind the sign-up form are painted by these scripts,
from a fixed random seed, so the same picture comes out every time.

```bash
python3 art/sky.py  $PWD/art/sky.svg
python3 art/land.py $PWD/art/land.svg
node art/render.js $PWD/art/sky.svg  sky.png  1600 1000 1.5
node art/render.js $PWD/art/land.svg land.png 1600 560 1.5 transparent
```

`render.js` uses Playwright's Chromium. Save the results as WebP (quality
about 86) over `internal/site/static/img/sky.webp` and `land.webp`. A
second argument to either Python script changes the seed.

The tear sits 55% of the way across the sky; if you move it, change
`--tx` for `.shards` in the stylesheet so the rising fragments still head
for it.

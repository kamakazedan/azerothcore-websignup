#!/usr/bin/env python3
"""Paints the site's built-in background: a blizzard over a frozen plain,
ice spires on the horizon, a cold glow behind the clouds.

Everything is drawn here from shapes and noise; nothing is traced or copied.
The result is released under CC0 1.0 (see art/README.md).

    python3 art/background.py            # writes both pictures
    python3 art/background.py --seed 7   # a different storm

Needs numpy and Pillow.
"""
import argparse
import math
import os

import numpy as np
from PIL import Image, ImageDraw, ImageFilter

OUT = os.path.join(os.path.dirname(__file__), "..", "internal", "site", "static", "img")


# ------------------------------------------------------------------ helpers
def noise(rng, w, h, cell):
    """Smooth value noise: a coarse random grid, scaled up bicubically."""
    gw, gh = max(2, int(w / cell) + 2), max(2, int(h / cell) + 2)
    g = Image.fromarray((rng.random((gh, gw)) * 255).astype(np.uint8))
    g = g.resize((int(gw * cell), int(gh * cell)), Image.BICUBIC)
    return np.asarray(g, np.float32)[:h, :w] / 255.0


def fbm(rng, w, h, cell, octaves=6, gain=0.5):
    out, amp, total = np.zeros((h, w), np.float32), 1.0, 0.0
    for _ in range(octaves):
        out += noise(rng, w, h, cell) * amp
        total += amp
        amp *= gain
        cell = max(2, cell / 2)
    return out / total


def blur(a, r):
    if r <= 0:
        return a
    im = Image.fromarray(np.clip(a * 255, 0, 255).astype(np.uint8))
    return np.asarray(im.filter(ImageFilter.GaussianBlur(r)), np.float32) / 255.0


def mask(w, h, draw, ss=3):
    """A soft-edged mask: draw(ImageDraw, scale) at ss times the size, then shrink."""
    im = Image.new("L", (w * ss, h * ss), 0)
    draw(ImageDraw.Draw(im), ss)
    return np.asarray(im.resize((w, h), Image.LANCZOS), np.float32) / 255.0


def col(hexs):
    return np.array([int(hexs[i:i + 2], 16) / 255.0 for i in (1, 3, 5)], np.float32)


def over(img, m, c):
    m = m[..., None]
    return img * (1 - m) + c * m


def screen(img, m, c):
    return 1 - (1 - img) * (1 - m[..., None] * c)


def smooth(e0, e1, x):
    t = np.clip((x - e0) / (e1 - e0), 0, 1)
    return t * t * (3 - 2 * t)


# ------------------------------------------------------------------ the land
def spires(w, h, x0, x1, base, top, rng):
    """A far citadel: tall thin spires with pointed roofs, and jagged ice
    crags around them."""
    def draw(d, ss):
        x = x0
        while x < x1:
            sw = rng.uniform(0.004, 0.012) * w
            mid = (x - x0) / (x1 - x0)
            tall = (top + (base - top) * (0.25 + 0.75 * abs(mid - 0.45) * 2)) + rng.uniform(-0.04, 0.04) * h
            tall = min(tall, base - 0.02 * h)
            flare = sw * rng.uniform(0.3, 0.8)  # wider at the foot
            d.polygon([((x - flare) * ss, base * ss), (x * ss, tall * ss), ((x + sw / 2) * ss, (tall - sw * rng.uniform(3, 7)) * ss),
                       ((x + sw) * ss, tall * ss), ((x + sw + flare) * ss, base * ss)], fill=255)
            if rng.random() < 0.5:  # a buttress or lower hall beside it
                hw = rng.uniform(0.01, 0.03) * w
                hy = base - rng.uniform(0.02, 0.06) * h
                d.rectangle([x * ss, hy * ss, (x + hw) * ss, base * ss], fill=255)
            x += sw + rng.uniform(0.002, 0.014) * w
        # Ice crags along the foot of the citadel.
        pts = [(x0 * ss - 0.08 * w * ss, base * ss)]
        x = x0 - 0.08 * w
        while x < x1 + 0.08 * w:
            pts.append((x * ss, (base - rng.uniform(0.005, 0.05) * h) * ss))
            x += rng.uniform(0.004, 0.02) * w
        pts.append((x * ss, base * ss))
        d.polygon(pts, fill=255)
    return mask(w, h, draw)


def ridge(w, h, base, height, rough, rng, sharp=False):
    """A mountain line above `base`, `height` tall at most."""
    x = np.arange(w, dtype=np.float32)
    line = np.zeros(w, np.float32)
    amp, freq = 1.0, rough / w
    for _ in range(6):
        ph = rng.uniform(0, 100)
        wave = np.sin(x * freq * 2 * math.pi + ph) + 0.5 * np.sin(x * freq * 4.3 * math.pi + ph * 1.7)
        line += (np.abs(wave) if sharp else wave) * amp
        amp *= 0.5
        freq *= 2.1
    line = (line - line.min()) / (line.max() - line.min() + 1e-6)
    top = base - line * height
    yy = np.arange(h, dtype=np.float32)[:, None]
    return np.clip(yy - top[None, :] + 0.5, 0, 1)


# ------------------------------------------------------------------ the storm
def snow(w, h, rng, count, size, length, angle, alpha, blur_r):
    """Wind-driven snow: short streaks, all leaning with the wind."""
    ss = 2
    im = Image.new("L", (w * ss, h * ss), 0)
    d = ImageDraw.Draw(im)
    dx, dy = math.cos(angle), math.sin(angle)
    for _ in range(count):
        x, y = rng.uniform(-0.05, 1.05) * w, rng.uniform(-0.05, 1.05) * h
        ln = length * rng.uniform(0.4, 1.4)
        sz = size * rng.uniform(0.5, 1.3)
        a = int(255 * alpha * rng.uniform(0.35, 1.0))
        d.line([(x * ss, y * ss), ((x + dx * ln) * ss, (y + dy * ln) * ss)], fill=a, width=max(1, int(sz * ss)))
    im = im.resize((w, h), Image.LANCZOS)
    if blur_r:
        im = im.filter(ImageFilter.GaussianBlur(blur_r))
    return np.asarray(im, np.float32) / 255.0


def flakes(w, h, rng, count, size, alpha, blur_r, gust):
    """Larger, flatter flakes and torn ice, tumbling in the gusts."""
    ss = 2
    im = Image.new("L", (w * ss, h * ss), 0)
    d = ImageDraw.Draw(im)
    for _ in range(count):
        x, y = rng.uniform(0, 1) * w, rng.uniform(0, 1) * h
        if gust[min(h - 1, max(0, int(y))), min(w - 1, max(0, int(x)))] < rng.random() * 0.9:
            continue
        a = rng.uniform(0, math.pi)
        lx, ly = size * rng.uniform(0.8, 2.2), size * rng.uniform(0.25, 0.6)
        pts = []
        for k in range(4):
            t = a + k * math.pi / 2
            r = lx if k % 2 == 0 else ly
            pts.append(((x + math.cos(t) * r) * ss, (y + math.sin(t) * r) * ss))
        d.polygon(pts, fill=int(255 * alpha * rng.uniform(0.4, 1.0)))
    im = im.resize((w, h), Image.LANCZOS)
    if blur_r:
        im = im.filter(ImageFilter.GaussianBlur(blur_r))
    return np.asarray(im, np.float32) / 255.0


# ------------------------------------------------------------------ painting
def paint(w, h, seed, glow_x, citadel):
    rng = np.random.default_rng(seed)
    u = np.linspace(0, 1, w, dtype=np.float32)[None, :]
    v = np.linspace(0, 1, h, dtype=np.float32)[:, None]
    aspect = w / h
    big = max(w, h)
    horizon = 0.735 * h

    # The sky: blue-black, lighter and bluer toward a glow behind the
    # clouds, with drifting banks of storm cloud.
    deep, mid, pale = col("#02050b"), col("#0d2238"), col("#3f6f9c")
    img = np.broadcast_to(deep, (h, w, 3)).copy()
    g = np.exp(-(((u - glow_x) * aspect / 0.75) ** 2 + ((v - 0.3) / 0.42) ** 2))
    clouds = fbm(rng, w, h, big / 2.2, octaves=7)
    streaks = fbm(np.random.default_rng(seed + 1), w, h, big / 5, octaves=5)
    streaks = np.asarray(Image.fromarray((streaks * 255).astype(np.uint8)).resize((w, h)).filter(
        ImageFilter.BoxBlur(int(big * 0.004))), np.float32) / 255.0
    sky = np.clip(0.55 * g + 0.65 * (clouds - 0.35) + 0.25 * (streaks - 0.5) + 0.15 * (1 - v), 0, 1)
    img = over(img, smooth(0.0, 0.75, sky), mid)
    img = screen(img, smooth(0.35, 1.0, sky * (0.4 + 0.9 * g)) * 0.75, pale)
    # A colder light under the cloud deck near the horizon.
    img = screen(img, np.exp(-((v - 0.66) / 0.1) ** 2) * (0.25 + 0.3 * clouds) * np.ones_like(u), col("#2a4c70"))

    # Far land: a pale mountain line, then the citadel and its crags.
    far = ridge(w, h, horizon + 0.01 * h, 0.07 * h, 3.2, rng, sharp=True)
    img = over(img, far * 0.55, col("#1b334c"))
    cx0, cx1 = citadel
    keep = spires(w, h, cx0 * w, cx1 * w, horizon + 0.012 * h, horizon - 0.25 * h, rng)
    img = over(img, keep * 0.9, col("#07101a"))
    near_crags = ridge(w, h, horizon + 0.03 * h, 0.06 * h, 7, rng, sharp=True) * smooth(0.4, 0.0, u)
    img = over(img, near_crags * 0.95, col("#060d16"))

    # Mist rolling along the horizon, thick enough to swallow the citadel's foot.
    mist = fbm(rng, w, h, big / 3, octaves=5)
    band = np.exp(-((v * h - horizon) / (0.06 * h)) ** 2)
    img = screen(img, np.clip(band * (0.25 + 0.7 * mist), 0, 1) * 0.4, col("#46698c"))

    # The snowfield, a little lighter where the glow falls, with drifts.
    ground = smooth(horizon - 2, horizon + 0.03 * h, v * h) * np.ones_like(u)
    drifts = fbm(rng, w, h, big / 6, octaves=6)
    lit = np.exp(-(((u - glow_x * 0.9) * aspect / 1.1) ** 2)) * smooth(1.0, 0.75, v)
    snowc = col("#1a2e43") * (0.65 + 0.55 * drifts[..., None] + 0.35 * lit[..., None])
    img = img * (1 - ground[..., None]) + snowc * ground[..., None]
    img = screen(img, ground * smooth(0.55, 0.85, drifts) * 0.18, col("#6d8fb3"))

    # Snow blown low across the field.
    veil = smooth(0.82, 1.0, v) * (0.4 + 0.6 * fbm(rng, w, h, big / 8, octaves=5))
    img = screen(img, veil * 0.35, col("#4d6f92"))

    # The storm, back to front: fine streaks, gusts of haze, then larger
    # torn flakes, the nearest ones out of focus.
    gust = fbm(rng, w, h, big / 2.5, octaves=4)
    gust = smooth(0.3, 0.75, gust)
    angle = math.radians(16)
    s = big / 1920
    area = (w * h) / (1920 * 1080)
    img = screen(img, snow(w, h, rng, int(2600 * area), 1.0 * s, 6 * s, angle, 0.3, 0.3 * s) *
                 (0.15 + 0.85 * gust), col("#bcd6f0"))
    haze = smooth(0.4, 0.9, fbm(rng, w, h, big / 3.5, octaves=6)) * (0.35 + 0.65 * g)
    img = screen(img, haze * 0.3, col("#5a83ab"))
    img = screen(img, snow(w, h, rng, int(900 * area), 2.0 * s, 14 * s, angle, 0.4, 0.8 * s) *
                 (0.2 + 0.8 * gust), col("#cfe2f5"))
    img = screen(img, flakes(w, h, rng, int(800 * area), 3.2 * s, 0.45, 0.8 * s, gust), col("#b9d3ec"))
    img = screen(img, flakes(w, h, rng, int(260 * area), 7 * s, 0.3, 3.0 * s, gust), col("#c8dcf0"))
    img = screen(img, flakes(w, h, rng, int(25 * area), 22 * s, 0.12, 12 * s, gust), col("#c8dcf0"))

    # A deep vignette, a little cold grain.
    r = np.sqrt(((u - 0.5) * 1.1) ** 2 + ((v - 0.45) * 1.25) ** 2)
    img *= (1 - 0.6 * smooth(0.35, 0.95, r))[..., None]
    img += rng.normal(0, 0.008, (h, w, 1)).astype(np.float32)
    img = np.clip(img, 0, 1) ** 1.05
    return Image.fromarray((img * 255 + 0.5).astype(np.uint8))


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--seed", type=int, default=3)
    ap.add_argument("--out", default=OUT)
    a = ap.parse_args()
    wide = paint(2560, 1440, a.seed, glow_x=0.62, citadel=(0.07, 0.2))
    wide.save(os.path.join(a.out, "background.jpg"), quality=80, optimize=True, progressive=True, subsampling="4:2:0")
    tall = paint(1080, 1920, a.seed, glow_x=0.65, citadel=(0.04, 0.3))
    tall.save(os.path.join(a.out, "background-phone.jpg"), quality=78, optimize=True, progressive=True, subsampling="4:2:0")


if __name__ == "__main__":
    main()

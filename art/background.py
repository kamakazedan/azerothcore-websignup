#!/usr/bin/env python3
"""Paints the site's built-in background: a lone knight in a blizzard on a
frozen plain, ice spires on the horizon, a cold glow behind the clouds.

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


def blurf(a, r):
    """Blur that keeps float precision (in 16-bit steps)."""
    im = Image.fromarray((np.clip(a, 0, 1) * 65535).astype(np.uint16).astype(np.int32), "I")
    im = im.convert("F").filter(ImageFilter.GaussianBlur(r))
    return np.asarray(im, np.float32) / 65535.0


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


# ------------------------------------------------------------------ the figure
# The knight as outlines in figure units: x across from the body's centre
# line, y from the top of the helm (0) to the feet (1). He stands facing
# us, both gauntlets on the pommel of a sword planted in the snow, cloak
# streaming out to the right in the wind.
BODY = [
    (0.0, -0.012), (0.034, 0.012), (0.056, 0.045), (0.062, 0.085), (0.052, 0.118),  # hood
    (0.075, 0.128), (0.12, 0.112), (0.175, 0.118), (0.215, 0.145), (0.238, 0.19),   # near pauldron
    (0.236, 0.235), (0.212, 0.262), (0.196, 0.31), (0.17, 0.355), (0.12, 0.395),    # near arm
    (0.105, 0.45), (0.098, 0.52), (0.122, 0.575), (0.13, 0.68),                      # hip, tasset
    (0.135, 0.8), (0.15, 0.93), (0.172, 0.99), (0.06, 1.0), (0.05, 0.9), (0.04, 0.8),  # near leg
    (0.0, 0.72), (-0.035, 0.8), (-0.04, 0.9), (-0.03, 0.985), (-0.135, 1.0),         # far leg
    (-0.12, 0.92), (-0.12, 0.8), (-0.125, 0.68), (-0.115, 0.58),
    (-0.1, 0.5), (-0.14, 0.478), (-0.185, 0.47), (-0.205, 0.43),                     # hands
    (-0.2, 0.37), (-0.215, 0.31), (-0.212, 0.255),                                   # far arm
    (-0.205, 0.205), (-0.185, 0.16), (-0.145, 0.133), (-0.098, 0.128), (-0.06, 0.124),  # far pauldron
    (-0.052, 0.112), (-0.062, 0.08), (-0.054, 0.042), (-0.032, 0.012),
]
SPIKES = [  # base left, tip, base right
    [(0.15, 0.116), (0.165, 0.07), (0.18, 0.12)],
    [(0.195, 0.13), (0.228, 0.095), (0.218, 0.15)],
    [(0.226, 0.17), (0.268, 0.155), (0.236, 0.2)],
    [(-0.12, 0.13), (-0.135, 0.09), (-0.15, 0.135)],
    [(-0.17, 0.148), (-0.2, 0.12), (-0.19, 0.17)],
]
# Lines of the armour, picked out faintly by the light.
SEAMS = [
    [(0.07, 0.24), (0.13, 0.262), (0.2, 0.258), (0.232, 0.232)],   # near pauldron's lower edge
    [(0.08, 0.205), (0.14, 0.2), (0.2, 0.2)],                      # its ridge
    [(-0.07, 0.235), (-0.13, 0.252), (-0.2, 0.245)],                # far pauldron
    [(-0.085, 0.5), (0.0, 0.512), (0.1, 0.5)],                      # belt
    [(-0.05, 0.3), (0.0, 0.42), (0.05, 0.3)],                       # breastplate keel
    [(0.11, 0.6), (0.12, 0.68)], [(-0.1, 0.6), (-0.11, 0.68)],       # tassets
    [(0.05, 0.86), (0.14, 0.86)], [(-0.04, 0.86), (-0.12, 0.86)],    # greaves
    [(-0.06, 0.52), (-0.075, 0.74), (-0.06, 0.95)], [(0.075, 0.52), (0.09, 0.74), (0.075, 0.95)],  # tabard
]
TABARD = [(-0.07, 0.5), (0.08, 0.5), (0.1, 0.74), (0.085, 0.965), (0.0, 0.985), (-0.075, 0.965), (-0.085, 0.74)]
SX = -0.152
POMMEL = (SX, 0.418, 0.017)
GRIP = [(SX - 0.01, 0.43), (SX + 0.01, 0.43), (SX + 0.011, 0.5), (SX - 0.011, 0.5)]
GUARD = [(SX - 0.11, 0.52), (SX - 0.09, 0.5), (SX - 0.03, 0.495), (SX, 0.485), (SX + 0.03, 0.495),
         (SX + 0.09, 0.5), (SX + 0.11, 0.52), (SX + 0.075, 0.512), (SX + 0.025, 0.515), (SX, 0.535),
         (SX - 0.025, 0.515), (SX - 0.075, 0.512)]
BLADE = [(SX - 0.028, 0.512), (SX + 0.028, 0.512), (SX + 0.025, 0.8), (SX + 0.013, 0.975),
         (SX - 0.013, 0.975), (SX - 0.025, 0.8)]


def cloak(rng):
    pts = [(-0.11, 0.15), (0.0, 0.13), (0.17, 0.14), (0.25, 0.2), (0.31, 0.32), (0.39, 0.5), (0.48, 0.68), (0.6, 0.86),
           (0.66, 0.9)]
    n = 15
    for i in range(1, n + 1):  # a torn hem, back toward his heels
        t = i / n
        x = 0.66 - t * 0.55
        y = 0.9 + 0.1 * t + (rng.uniform(0.01, 0.045) if i % 2 else -rng.uniform(0, 0.015)) * (1 - 0.5 * t)
        pts.append((x, y))
    return pts + [(-0.06, 0.99), (-0.175, 0.995), (-0.2, 0.75), (-0.215, 0.45), (-0.11, 0.15)]


def figure_layers(w, h, fx, fy, fh, rng):
    """Masks for the knight standing with his feet at (fx, fy), fh tall (pixels)."""
    cape_pts = cloak(rng)

    def at(pts, ss):
        return [((fx + x * fh) * ss, (fy - fh + y * fh) * ss) for x, y in pts]

    def body(d, ss):
        d.polygon(at(BODY, ss), fill=255)
        d.polygon(at(TABARD, ss), fill=255)
        for p in SPIKES:
            d.polygon(at(p, ss), fill=255)

    def cape(d, ss):
        d.polygon(at(cape_pts, ss), fill=255)

    def sword(d, ss):
        x, y, r = POMMEL
        d.polygon(at([(x, y - r * 1.4), (x + r, y), (x, y + r * 1.1), (x - r, y)], ss), fill=255)
        for p in (GRIP, GUARD, BLADE):
            d.polygon(at(p, ss), fill=255)

    def edge(d, ss):  # the blade's edge and fuller
        d.line(at([(SX + 0.026, 0.52), (SX + 0.023, 0.8), (SX + 0.012, 0.97)], ss), fill=255, width=max(1, int(0.003 * fh * ss)))
        d.line(at([(SX + 0.004, 0.53), (SX + 0.003, 0.9)], ss), fill=150, width=max(1, int(0.005 * fh * ss)))
        d.line(at([(SX - 0.09, 0.502), (SX + 0.09, 0.502)], ss), fill=90, width=max(1, int(0.003 * fh * ss)))

    def seams(d, ss):
        for p in SEAMS:
            d.line(at(p, ss), fill=255, width=max(1, int(0.003 * fh * ss)), joint="curve")

    def eyes(d, ss):
        for ex in (-0.017, 0.019):
            (px, py), = at([(ex, 0.075)], ss)
            r = 0.005 * fh * ss
            d.ellipse([px - r * 1.7, py - r * 0.8, px + r * 1.7, py + r * 0.8], fill=255)

    return {k: mask(w, h, f) for k, f in
            (("body", body), ("cape", cape), ("sword", sword), ("edge", edge), ("seams", seams), ("eyes", eyes))}


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
def paint(w, h, seed, fig_x, fig_scale, glow_x, citadel):
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

    # The knight.
    fh = fig_scale * h
    fx, fy = fig_x * w, 0.965 * h
    m = figure_layers(w, h, fx, fy, fh, rng)
    # Light breaking through behind his shoulders, so he stands out against it.
    halo = np.exp(-(((u * w - (fx + 0.12 * fh)) / (0.55 * fh)) ** 2 + ((v * h - (fy - 0.8 * fh)) / (0.5 * fh)) ** 2))
    img = screen(img, halo * 0.32, col("#4c7fb3"))
    low = np.exp(-(((u * w - (fx + 0.2 * fh)) / (0.9 * fh)) ** 2 + ((v * h - (fy - 0.3 * fh)) / (0.22 * fh)) ** 2))
    img = screen(img, low * 0.18, col("#41668c"))
    # A soft contact shadow under him.
    shadow = blur(mask(w, h, lambda d, ss: d.ellipse([(fx - 0.25 * fh) * ss, (fy - 0.02 * fh) * ss,
                                                      (fx + 0.62 * fh) * ss, (fy + 0.03 * fh) * ss], fill=255)), fh * 0.02)
    img = over(img, shadow * 0.7, col("#03060b"))
    sil = np.clip(m["body"] + m["cape"] + m["sword"], 0, 1)
    up = smooth(0.0, 0.55, (fy - v * h) / fh)  # 0 at his feet, 1 from the waist up
    # Inside the silhouette: near black, a faint cold sheen on the armour
    # toward the light, folds in the cloak.
    folds = noise(rng, w, h, fh * 0.03)
    folds = np.asarray(Image.fromarray((folds * 255).astype(np.uint8)).filter(
        ImageFilter.BoxBlur(max(1, int(fh * 0.006)))), np.float32) / 255.0
    toward = np.clip((u * w - (fx - 0.25 * fh)) / (0.5 * fh), 0, 1)
    armour = col("#04080e") + col("#1c3047") * (0.45 * toward * up)[..., None]
    cloakc = col("#03060a") + col("#122133") * (0.75 * folds * smooth(-0.05, 0.6, (u * w - fx) / fh) * (0.4 + 0.6 * up))[..., None]
    img = over(img, m["cape"], cloakc)
    img = over(img, m["body"], armour)
    img = screen(img, blur(m["seams"], fh * 0.002) * (0.05 + 0.16 * toward * up), col("#7fa9d6"))
    steel = col("#070d15") + col("#22384f") * (0.25 + 0.45 * toward * up)[..., None]
    img = over(img, m["sword"], steel)
    img = screen(img, m["edge"] * (0.2 + 0.45 * up), col("#a9cff5"))
    # Rim light: a thin line where his edges face the glow (up and to the right).
    lx, ly = 0.004 * fh, -0.003 * fh
    shifted = np.asarray(Image.fromarray((sil * 255).astype(np.uint8)).transform(
        (w, h), Image.AFFINE, (1, 0, lx, 0, 1, ly), Image.BILINEAR), np.float32) / 255.0
    rim = np.clip(sil - shifted, 0, 1) * (0.15 + 0.85 * up)
    img = screen(img, blur(rim, fh * 0.001) * 0.75, col("#a7cdf5"))
    img = screen(img, blur(rim, fh * 0.01) * 0.3, col("#4f86c0"))
    # A cold light in the helm, faint, with a bloom.
    img = screen(img, m["eyes"] * 0.9, col("#d6eeff"))
    img = screen(img, blur(m["eyes"], fh * 0.006) * 0.8, col("#5fa8ff"))
    img = screen(img, blur(m["eyes"], fh * 0.025) * 0.45, col("#2c6fb8"))
    # Blown snow veiling his feet and the hem of the cloak.
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
    wide = paint(2560, 1440, a.seed, fig_x=0.36, fig_scale=0.66, glow_x=0.62, citadel=(0.07, 0.2))
    wide.save(os.path.join(a.out, "background.jpg"), quality=80, optimize=True, progressive=True, subsampling="4:2:0")
    tall = paint(1080, 1920, a.seed, fig_x=0.42, fig_scale=0.42, glow_x=0.65, citadel=(0.04, 0.3))
    tall.save(os.path.join(a.out, "background-phone.jpg"), quality=78, optimize=True, progressive=True, subsampling="4:2:0")


if __name__ == "__main__":
    main()

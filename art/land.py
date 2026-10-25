"""The hero's ground: heavy black crags lit at their edges by the tear
above, fragments breaking away from them, mist, and dark ice with a pool
of light under the tear. Original art, seeded. Transparent above."""
import math, random, sys

W, H = 1600, 560
TX, TY = 880, -720           # the tear, far above this layer
rnd = random.Random(int(sys.argv[2]) if len(sys.argv) > 2 else 11)

def f(v):
    s = f"{v:.1f}"
    return s[:-2] if s.endswith(".0") else s

def pts(p):
    return " ".join(f"{f(x)},{f(y)}" for x, y in p)

def mix(c1, c2, t):
    t = max(0.0, min(1.0, t))
    a = [int(c1[i:i + 2], 16) for i in (1, 3, 5)]
    b = [int(c2[i:i + 2], 16) for i in (1, 3, 5)]
    return "#" + "".join(f"{round(x + (y - x) * t):02x}" for x, y in zip(a, b))

def ridge(anchors, rough, depth, decay=.55):
    p = anchors[:]
    for _ in range(depth):
        n = [p[0]]
        for (x0, y0), (x1, y1) in zip(p, p[1:]):
            n += [((x0 + x1) / 2 + rnd.uniform(-.1, .1) * (x1 - x0), (y0 + y1) / 2 + rnd.uniform(-rough, rough)), (x1, y1)]
        p = n
        rough *= decay
    return p

def rim(p, strength, colour="#d8f2ff", width=1.6):
    """Light along the edges that face the tear: up, and towards it."""
    lines = []
    for (x0, y0), (x1, y1) in zip(p, p[1:]):
        mx, my = (x0 + x1) / 2, (y0 + y1) / 2
        # outward normal of a ridge drawn left to right points up: (dy, -dx)
        nx, ny = (y1 - y0), -(x1 - x0)
        L = math.hypot(nx, ny) or 1
        nx, ny = nx / L, ny / L
        lx, ly = TX - mx, TY - my
        LL = math.hypot(lx, ly)
        lx, ly = lx / LL, ly / LL
        facing = nx * lx + ny * ly
        if facing <= 0.3:
            continue
        near = max(0.0, 1 - abs(mx - TX) / 950) ** 1.2
        a = min(1.0, (facing - .3) * 1.8) * (.12 + .88 * near) * strength * rnd.uniform(.35, 1.0)
        if a < .05:
            continue
        lines.append(f'<line x1="{f(x0)}" y1="{f(y0)}" x2="{f(x1)}" y2="{f(y1)}" stroke-opacity="{a:.2f}"/>')
    body = "".join(lines)
    return (f'<g stroke="{colour}" stroke-width="{width * 2.6}" stroke-linecap="round" filter="url(#blur2)" opacity=".5">{body}</g>'
            f'<g stroke="{colour}" stroke-width="{width}" stroke-linecap="round">{body}</g>')

def lit_faces(p, depth, colour="#3f6c9e"):
    """Soft lit planes below the edges that face the tear."""
    polys = []
    for (x0, y0), (x1, y1) in zip(p, p[1:]):
        nx, ny = (y1 - y0), -(x1 - x0)
        L = math.hypot(nx, ny) or 1
        mx, my = (x0 + x1) / 2, (y0 + y1) / 2
        lx, ly = TX - mx, TY - my
        LL = math.hypot(lx, ly)
        facing = (nx / L) * (lx / LL) + (ny / L) * (ly / LL)
        if facing <= .35:
            continue
        near = max(0.0, 1 - abs(mx - TX) / 950)
        d = depth * rnd.uniform(.5, 1.2)
        side = 1 if mx < TX else -1
        poly = [(x0, y0), (x1, y1), (x1 - side * d * .25, y1 + d), (x0 - side * d * .25, y0 + d)]
        polys.append(f'<polygon points="{pts(poly)}" fill-opacity="{(facing - .35) * .5 * (.2 + .8 * near):.2f}"/>')
    return f'<g fill="{colour}" filter="url(#blur6)">' + "".join(polys) + '</g>'

out = []
add = out.append
add(f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {W} {H}" width="{W}" height="{H}">')
add(f'''<defs>
<linearGradient id="farfill" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#0e1d36"/><stop offset="1" stop-color="#091426"/></linearGradient>
<linearGradient id="midfill" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#0b1629"/><stop offset="1" stop-color="#070d19"/></linearGradient>
<linearGradient id="rock" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#0a1322"/><stop offset=".5" stop-color="#060b15"/><stop offset="1" stop-color="#04070d"/></linearGradient>
<linearGradient id="mist" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#4c82bd" stop-opacity="0"/><stop offset=".55" stop-color="#4c82bd" stop-opacity=".3"/><stop offset="1" stop-color="#4c82bd" stop-opacity="0"/></linearGradient>
<linearGradient id="ground" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#0a1322"/><stop offset=".4" stop-color="#060a13"/><stop offset="1" stop-color="#05080f"/></linearGradient>
<radialGradient id="pool" cx="{TX}" cy="482" r="420" gradientUnits="userSpaceOnUse" gradientTransform="translate({TX} 482) scale(1 .15) translate(-{TX} -482)">
  <stop offset="0" stop-color="#bfe6ff" stop-opacity=".55"/><stop offset=".4" stop-color="#5f97d2" stop-opacity=".25"/><stop offset="1" stop-color="#5f97d2" stop-opacity="0"/>
</radialGradient>
<filter id="blur10" x="-10%" y="-60%" width="120%" height="220%"><feGaussianBlur stdDeviation="10"/></filter>
<filter id="blur2" x="-20%" y="-20%" width="140%" height="140%"><feGaussianBlur stdDeviation="1.6"/></filter>
<filter id="blur6" x="-20%" y="-20%" width="140%" height="140%"><feGaussianBlur stdDeviation="6"/></filter>
</defs>''')

# far ridge, low and hazy
far = ridge([(-20, 360), (240, 318), (480, 352), (700, 326), (900, 350), (1120, 318), (1360, 346), (1620, 322)], 22, 6)
add(f'<polygon points="-20,{H} {pts(far)} 1620,{H}" fill="url(#farfill)"/>')
add(rim(far, .45, "#a9d8ff", 1.2))
add(f'<rect x="-20" y="300" width="{W + 40}" height="160" fill="url(#mist)" filter="url(#blur10)"/>')

# two lone pinnacles in the middle distance, softened by the haze
for anchors in ([(560, 470), (600, 360), (628, 262), (652, 352), (700, 470)],
                [(1080, 470), (1118, 380), (1150, 300), (1176, 372), (1224, 470)]):
    p = ridge(anchors, 9, 4)
    add(f'<polygon points="{pts(p)}" fill="url(#midfill)"/>')
    add(lit_faces(p, 70, "#4a7aae"))
    add(rim(p, .7, "#bfe4ff", 1.2))

# the great mass on the left
left = ridge([(-30, 150), (70, 96), (150, 160), (226, 40), (290, 138), (356, 74), (420, 184), (500, 250),
              (566, 340), (626, 430), (668, 520)], 26, 6)
add(f'<polygon points="-30,{H} {pts(left)} 668,{H}" fill="url(#rock)"/>')
add(lit_faces(left, 120))
# faint creases down its face
for x0, y0, x1, y1 in [(226, 40, 190, 300), (356, 74, 330, 340), (226, 40, 260, 260), (70, 96, 40, 330), (500, 250, 470, 420)]:
    c = ridge([(x0, y0), (x1, y1)], 8, 3)
    add(f'<polyline points="{pts(c)}" fill="none" stroke="#13223a" stroke-width="1.6" opacity=".45" filter="url(#blur2)"/>')
add(rim(left, 1.0))

# a lower, sharper mass on the right, leaning in
right = ridge([(1050, 520), (1100, 452), (1170, 380), (1230, 300), (1290, 196), (1338, 262), (1398, 118),
               (1452, 214), (1520, 160), (1580, 230), (1630, 210)], 22, 6)
add(f'<polygon points="1050,{H} {pts(right)} 1630,{H}" fill="url(#rock)"/>')
add(lit_faces(right, 110))
for x0, y0, x1, y1 in [(1398, 118, 1420, 340), (1290, 196, 1310, 380), (1520, 160, 1540, 360)]:
    c = ridge([(x0, y0), (x1, y1)], 7, 3)
    add(f'<polyline points="{pts(c)}" fill="none" stroke="#13223a" stroke-width="1.6" opacity=".45" filter="url(#blur2)"/>')
add(rim(right, 1.0))

# fragments lifting away from the summits
def chunk(cx, cy, r):
    n = rnd.choice([6, 7, 8])
    rot = rnd.uniform(0, math.pi)
    angs = sorted(rnd.uniform(0, 2 * math.pi) for _ in range(n))
    p = []
    for a in angs:
        rr = r * rnd.uniform(.72, 1.08)
        x, y = math.cos(a) * rr * 1.2, math.sin(a) * rr
        p.append((cx + x * math.cos(rot) - y * math.sin(rot), cy + x * math.sin(rot) + y * math.cos(rot)))
    return p

frags = []
for sx, sy, n in [(226, 40, 6), (356, 74, 5), (70, 96, 3), (1398, 118, 6), (1520, 160, 4), (1290, 196, 3), (628, 262, 2), (1150, 300, 2)]:
    for k in range(n):
        x = sx + rnd.uniform(-40, 40)
        y = sy - rnd.uniform(16, 150)
        r = max(1.8, rnd.uniform(3, 11) * (1 - (sy - y) / 230))
        p = chunk(x, y, r)
        near = max(0.0, 1 - abs(x - TX) / 1000)
        up = sorted(range(len(p)), key=lambda i: p[i][1])[:3]
        i0, i1 = min(up), max(up)
        frags.append(f'<polygon points="{pts(p)}" fill="#04080f"/>'
                     f'<polyline points="{pts(p[i0:i1 + 1])}" fill="none" stroke="#cdeeff" stroke-width="1.2" stroke-opacity="{.35 + .5 * near:.2f}"/>')
add('<g>' + "".join(frags) + '</g>')

# the ground: dark ice, with light pooled beneath the tear
gr = ridge([(-20, 482), (300, 470), (620, 488), (880, 474), (1140, 490), (1420, 470), (1620, 484)], 7, 5)
add(f'<polygon points="-20,{H} {pts(gr)} 1620,{H}" fill="url(#ground)"/>')
add(f'<rect x="{TX - 440}" y="390" width="880" height="{H - 390}" fill="url(#pool)"/>')
add(rim(gr, .3, "#9fd0ff", 1))
add(f'<rect x="-20" y="450" width="{W + 40}" height="70" fill="url(#mist)" filter="url(#blur10)" opacity=".6"/>')
add('</svg>')
open(sys.argv[1], "w").write("".join(out))

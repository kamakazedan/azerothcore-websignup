"""The hero's sky: a rip of cold light, storm cloud around it, and rock
fragments streaming up into it. Original art, seeded."""
import math, random, sys

W, H = 1600, 1000
TX, TY = 880, 280          # middle of the tear
TOP, BOT = 30, 540         # its ends
FOOT = "#08142a"           # the colour the sky fades to at the bottom
rnd = random.Random(int(sys.argv[2]) if len(sys.argv) > 2 else 7)

def f(v):
    s = f"{v:.1f}"
    return s[:-2] if s.endswith(".0") else s

def pts(p):
    return " ".join(f"{f(x)},{f(y)}" for x, y in p)

def mix(c1, c2, t):
    a = [int(c1[i:i + 2], 16) for i in (1, 3, 5)]
    b = [int(c2[i:i + 2], 16) for i in (1, 3, 5)]
    return "#" + "".join(f"{round(x + (y - x) * t):02x}" for x, y in zip(a, b))

out = []
add = out.append
add(f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {W} {H}" width="{W}" height="{H}">')
add(f'''<defs>
<linearGradient id="sky" x1="0" y1="0" x2="0" y2="1">
  <stop offset="0" stop-color="#020409"/><stop offset=".45" stop-color="#050c19"/>
  <stop offset=".8" stop-color="#0a1a33"/><stop offset="1" stop-color="#0d2240"/>
</linearGradient>
<radialGradient id="halo" cx="{TX}" cy="{TY + 20}" r="900" gradientUnits="userSpaceOnUse" gradientTransform="translate({TX} {TY}) scale(1.15 .82) translate(-{TX} -{TY})">
  <stop offset="0" stop-color="#5d9ad8" stop-opacity=".75"/><stop offset=".22" stop-color="#2f62a3" stop-opacity=".5"/>
  <stop offset=".55" stop-color="#173a69" stop-opacity=".25"/><stop offset="1" stop-color="#0b1f3d" stop-opacity="0"/>
</radialGradient>
<radialGradient id="bloom" cx="{TX}" cy="{TY}" r="320" gradientUnits="userSpaceOnUse" gradientTransform="translate({TX} {TY}) scale(.62 1.2) translate(-{TX} -{TY})">
  <stop offset="0" stop-color="#e9f8ff" stop-opacity=".55"/><stop offset=".3" stop-color="#9fd5ff" stop-opacity=".25"/>
  <stop offset="1" stop-color="#4a8fd6" stop-opacity="0"/>
</radialGradient>
<radialGradient id="near" cx="{TX}" cy="{TY}" r="560" gradientUnits="userSpaceOnUse" gradientTransform="translate({TX} {TY}) scale(1 .9) translate(-{TX} -{TY})">
  <stop offset="0" stop-color="#fff"/><stop offset=".5" stop-color="#fff" stop-opacity=".45"/><stop offset="1" stop-color="#fff" stop-opacity="0"/>
</radialGradient>
<radialGradient id="far" cx="{TX}" cy="{TY}" r="760" gradientUnits="userSpaceOnUse" gradientTransform="translate({TX} {TY}) scale(1.1 .85) translate(-{TX} -{TY})">
  <stop offset="0" stop-color="#000"/><stop offset=".18" stop-color="#000"/><stop offset=".55" stop-color="#fff" stop-opacity=".8"/><stop offset="1" stop-color="#fff"/>
</radialGradient>
<radialGradient id="vignette" cx="{TX}" cy="{TY + 120}" r="1150" gradientUnits="userSpaceOnUse">
  <stop offset=".45" stop-color="#000" stop-opacity="0"/><stop offset="1" stop-color="#000" stop-opacity=".7"/>
</radialGradient>
<mask id="m-near" maskUnits="userSpaceOnUse" x="0" y="0" width="{W}" height="{H}"><rect width="{W}" height="{H}" fill="url(#near)"/></mask>
<mask id="m-far" maskUnits="userSpaceOnUse" x="0" y="0" width="{W}" height="{H}"><rect width="{W}" height="{H}" fill="url(#far)"/></mask>
<filter id="darkcloud" x="0" y="0" width="{W}" height="{H}" filterUnits="userSpaceOnUse" color-interpolation-filters="sRGB">
  <feTurbulence type="fractalNoise" baseFrequency="0.0021 0.0047" numOctaves="6" seed="23"/>
  <feColorMatrix type="matrix" values="0 0 0 0 0.008  0 0 0 0 0.016  0 0 0 0 0.035  2.6 0 0 0 -0.92"/>
</filter>
<filter id="mist" x="0" y="0" width="{W}" height="{H}" filterUnits="userSpaceOnUse" color-interpolation-filters="sRGB">
  <feTurbulence type="fractalNoise" baseFrequency="0.0034 0.0082" numOctaves="6" seed="5"/>
  <feColorMatrix type="matrix" values="0 0 0 0 0.62  0 0 0 0 0.82  0 0 0 0 1  2.4 0 0 0 -1.02"/>
</filter>
<filter id="g4" x="-50%" y="-20%" width="200%" height="140%"><feGaussianBlur stdDeviation="4"/></filter>
<filter id="g14" x="-100%" y="-30%" width="300%" height="160%"><feGaussianBlur stdDeviation="14"/></filter>
<filter id="g36" x="-200%" y="-50%" width="500%" height="200%"><feGaussianBlur stdDeviation="36"/></filter>
<filter id="soft"><feGaussianBlur stdDeviation="1.2"/></filter>
<linearGradient id="riftfill" x1="0" y1="0" x2="1" y2="0">
  <stop offset="0" stop-color="#5aa6ee"/><stop offset=".3" stop-color="#bfe8ff"/><stop offset=".5" stop-color="#ffffff"/>
  <stop offset=".7" stop-color="#bfe8ff"/><stop offset="1" stop-color="#5aa6ee"/>
</linearGradient>
<linearGradient id="ray" x1="0" y1="0" x2="0" y2="1">
  <stop offset="0" stop-color="#bfe6ff" stop-opacity=".2"/><stop offset="1" stop-color="#bfe6ff" stop-opacity="0"/>
</linearGradient>
<linearGradient id="trail" x1="0" y1="0" x2="0" y2="1">
  <stop offset="0" stop-color="#9fd2ff" stop-opacity=".35"/><stop offset="1" stop-color="#9fd2ff" stop-opacity="0"/>
</linearGradient>
</defs>''')
add(f'<rect width="{W}" height="{H}" fill="url(#sky)"/>')

stars = []
for _ in range(160):
    x, y = rnd.uniform(0, W), H * 0.62 * rnd.random() ** 1.3
    if math.hypot((x - TX) / 1.3, y - TY) < 380:
        continue
    stars.append(f'<circle cx="{f(x)}" cy="{f(y)}" r="{rnd.choice([.6, .7, .8, 1])}" opacity="{rnd.uniform(.15, .5):.2f}"/>')
add('<g fill="#cfe6ff">' + "".join(stars) + '</g>')

add(f'<rect width="{W}" height="{H}" fill="url(#halo)"/>')
# glowing mist close to the tear, heavy dark cloud further out
add(f'<g mask="url(#m-near)" opacity=".55"><rect width="{W}" height="{H}" filter="url(#mist)"/></g>')
add(f'<g mask="url(#m-far)"><rect width="{W}" height="{H}" filter="url(#darkcloud)"/></g>')
add(f'<rect width="{W}" height="{H}" fill="url(#bloom)"/>')

# light falling from the tear
rays = []
for i in range(11):
    a = math.radians(rnd.uniform(-42, 42))
    L = rnd.uniform(520, 950)
    w0, w1 = rnd.uniform(3, 9), rnd.uniform(40, 130)
    x0, y0 = TX + rnd.uniform(-12, 12), rnd.uniform(220, 470)
    ex, ey = x0 + math.sin(a) * L, y0 + math.cos(a) * L
    nx, ny = math.cos(a), -math.sin(a)
    poly = [(x0 - nx * w0, y0 - ny * w0), (x0 + nx * w0, y0 + ny * w0), (ex + nx * w1, ey + ny * w1), (ex - nx * w1, ey - ny * w1)]
    rays.append(f'<polygon points="{pts(poly)}" opacity="{rnd.uniform(.3, .75):.2f}"/>')
add('<g fill="url(#ray)" filter="url(#g14)">' + "".join(rays) + '</g>')

# the tear: a ragged rip of light. Its torn edge varies in thickness and
# fades into the glow, and light streams down inside it.
def fracture(points, amp, depth=5, decay=.55):
    """Midpoint displacement sideways: jagged at every scale, like a crack."""
    for _ in range(depth):
        nxt = [points[0]]
        for (x0, y0), (x1, y1) in zip(points, points[1:]):
            nxt += [((x0 + x1) / 2 + rnd.uniform(-amp, amp), (y0 + y1) / 2 + rnd.uniform(-amp * .3, amp * .3)), (x1, y1)]
        points = nxt
        amp *= decay
    return points

def centre(t):
    return TX + 22 * math.sin(t * 4.3 + 0.9) + 6 * math.sin(t * 15.7)

def rip_points(half, amp):
    n = 9
    left, right = [], []
    for i in range(n + 1):
        t = i / n
        y = TOP + (BOT - TOP) * t
        w = (math.sin(math.pi * t) ** 1.1) * half * rnd.uniform(.75, 1.2) + 1.0
        left.append((centre(t) - w, y))
        right.append((centre(t) + w, y))
    left[0] = right[0] = (centre(0), TOP - 6)
    left[-1] = right[-1] = (centre(1), BOT + 6)
    return fracture(left, amp), fracture(right, amp)

left, right = rip_points(36, 16)
inner = left + right[::-1]
lipL = [(x - rnd.uniform(2, 12) * (rnd.random() < .85), y) for x, y in left]
lipR = [(x + rnd.uniform(2, 12) * (rnd.random() < .85), y) for x, y in right]
lip = lipL + lipR[::-1]
add(f'<polygon points="{pts(lip)}" fill="#3b86d8" filter="url(#g36)" opacity=".95"/>')
add(f'<polygon points="{pts(lip)}" fill="#9fd8ff" filter="url(#g14)" opacity=".75"/>')
add(f'<polygon points="{pts(lip)}" fill="#07101f" opacity=".7" filter="url(#soft)"/>')
add(f'<clipPath id="inside"><polygon points="{pts(inner)}"/></clipPath>')
add(f'<g clip-path="url(#inside)">')
add(f'<rect x="{TX - 120}" y="{TOP - 10}" width="240" height="{BOT - TOP + 20}" fill="#a9dcff"/>')
streaks = []
for i in range(26):
    x0 = rnd.uniform(-34, 34)
    p = []
    for k in range(12):
        t = k / 11
        y = TOP - 10 + (BOT - TOP + 20) * t
        p.append((centre(t) + x0 * math.sin(math.pi * t) + rnd.uniform(-3, 3), y))
    col = rnd.choice(["#ffffff", "#ffffff", "#e8f7ff", "#7cbcf2"])
    streaks.append(f'<polyline points="{pts(p)}" stroke="{col}" stroke-width="{rnd.uniform(2, 9):.1f}" opacity="{rnd.uniform(.3, .9):.2f}"/>')
add('<g fill="none" filter="url(#g4)">' + "".join(streaks) + '</g>')
seam = []
for i in range(41):
    t = i / 40
    y = TOP + 20 + (BOT - TOP - 40) * t
    seam.append((TX + 22 * math.sin(t * 4.3 + 0.9) + 6 * math.sin(t * 15.7) + rnd.uniform(-1.5, 1.5), y))
add(f'<polyline points="{pts(seam)}" fill="none" stroke="#ffffff" stroke-width="7" stroke-linecap="round" filter="url(#g4)"/>')
add('</g>')
add(f'<polygon points="{pts(inner)}" fill="#ffffff" filter="url(#g14)" opacity=".55" style="mix-blend-mode:screen"/>')
# a bright rim just inside the torn edge
add(f'<polygon points="{pts(inner)}" fill="none" stroke="#e6f7ff" stroke-width="1.6" stroke-linejoin="round" opacity=".7"/>')

# rock fragments streaming up into the tear: big and near at the bottom,
# small and far as they reach it
def chunk(cx, cy, r):
    n = rnd.choice([6, 7, 7, 8])
    rot = rnd.uniform(0, math.pi)
    stretch = rnd.uniform(1.0, 1.35)
    angs = sorted(rnd.uniform(0, 2 * math.pi) for _ in range(n))
    p = []
    for a in angs:
        rr = r * rnd.uniform(.72, 1.08)
        x, y = math.cos(a) * rr * stretch, math.sin(a) * rr
        p.append((cx + x * math.cos(rot) - y * math.sin(rot), cy + x * math.sin(rot) + y * math.cos(rot)))
    return p

def draw_chunk(p, cx, cy, light, blur=False):
    n = len(p)
    # split into a lit facet (towards the tear) and a shadowed one
    k = max(range(n), key=lambda i: -math.hypot(p[i][0] - TX, p[i][1] - TY))
    j = (k + n // 2) % n
    a, b = sorted((k, j))
    face1 = p[a:b + 1]
    face2 = p[b:] + p[:a + 1]
    lit = mix("#0a1220", "#5d8fc4", light)
    rim = mix("#4d7aa8", "#e2f5ff", light)
    near_side = face1 if min(math.hypot(x - TX, y - TY) for x, y in face1) <= min(math.hypot(x - TX, y - TY) for x, y in face2) else face2
    far_side = face2 if near_side is face1 else face1
    flt = ' filter="url(#soft)"' if blur else ''
    s = (f'<g{flt}><polygon points="{pts(p)}" fill="#03060b"/>'
         f'<polygon points="{pts(near_side)}" fill="{lit}"/>'
         f'<polygon points="{pts(far_side)}" fill="#04080f"/>')
    # rim light on the edges facing the tear
    edges = []
    for i in range(n):
        x1, y1 = p[i]; x2, y2 = p[(i + 1) % n]
        mx, my = (x1 + x2) / 2, (y1 + y2) / 2
        if math.hypot(mx - TX, my - TY) < math.hypot(cx - TX, cy - TY):
            edges.append(f'<line x1="{f(x1)}" y1="{f(y1)}" x2="{f(x2)}" y2="{f(y2)}"/>')
    s += f'<g stroke="{rim}" stroke-width="1.3" stroke-linecap="round" opacity="{.35 + .6 * light:.2f}">' + "".join(edges) + '</g></g>'
    return s

frags, trails = [], []
# streams: curves from below sweeping up into the tear
for s in range(6):
    x_start = rnd.uniform(-100, W + 100)
    bend = rnd.uniform(-260, 260)
    count = rnd.randint(10, 16)
    for i in range(count):
        t = rnd.random() ** .8          # 0 at the bottom, 1 at the tear
        x = x_start + (TX - x_start) * t + bend * math.sin(math.pi * t) + rnd.uniform(-60, 60) * (1 - t)
        y = H + 40 - (H + 40 - TY) * t + rnd.uniform(-30, 30)
        if y > H - 60:
            continue
        d = math.hypot(x - TX, y - TY)
        r = 2.5 + 34 * (1 - t) ** 2.4 * rnd.uniform(.45, 1.1)
        light = max(0.0, min(1.0, 1.1 - d / 650))
        p = chunk(x, y, r)
        frags.append(draw_chunk(p, x, y, light, blur=r > 18))
# a cloud of small debris crowding the tear itself
for i in range(120):
    d = 40 + 330 * rnd.random() ** 1.5
    a = rnd.uniform(0, 2 * math.pi)
    x, y = TX + math.cos(a) * d * .9, TY + 30 + math.sin(a) * d * 1.05
    if not (0 < y < H - 100):
        continue
    r = rnd.uniform(1.2, 4.5) * (1 + d / 400)
    light = max(0.0, min(1.0, 1.15 - d / 420))
    frags.append(draw_chunk(chunk(x, y, r), x, y, light))
add('<g fill="url(#trail)">' + "".join(trails) + '</g>')
add('<g>' + "".join(frags) + '</g>')
add(f'<rect width="{W}" height="{H}" fill="url(#vignette)"/>')
# fade the foot to one flat colour, which the page continues below
add(f'''<linearGradient id="foot" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="{FOOT}" stop-opacity="0"/><stop offset="1" stop-color="{FOOT}"/></linearGradient>''')
add(f'<rect y="{H - 230}" width="{W}" height="230" fill="url(#foot)"/>')
add('</svg>')
open(sys.argv[1], "w").write("".join(out))

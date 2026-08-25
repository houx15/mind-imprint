"""Generate the four ASCII-art masks used by the homepage digital rain.

The shapes are HOLLOW: each is the outline of a solid form plus a few interior
strokes. At this resolution a filled silhouette with carved detail turns to
confetti, and a hollow figure lets the rain fall through it, which is the point.

Run from the repo root:
    python3 apps/site/scripts/gen-glyph-masks.py
It rewrites apps/site/src/scripts/glyph-masks.ts and prints the shapes, so a
change can be looked at before it ships.
"""
import io
import os
import math

# A mono cell is ~0.6 as wide as it is tall, so the domain is sampled in SQUARE
# units and the grid is shaped to match. Nothing here is stretched.
DW, DH = 21.0, 20.0
ROWS = 22
COLS = round(DW * ROWS / (0.6 * DH))  # 38


def rasterize(*preds):
    g = []
    for row in range(ROWS):
        y = (row + 0.5) * DH / ROWS
        line = []
        for col in range(COLS):
            x = (col + 0.5) * DW / COLS
            line.append(any(p(x, y) for p in preds))
        g.append(line)
    return g


def union(*grids):
    return [[any(g[r][c] for g in grids) for c in range(COLS)] for r in range(ROWS)]


def intersect(a, b):
    return [[a[r][c] and b[r][c] for c in range(COLS)] for r in range(ROWS)]


def outline(g):
    """Cells that are filled but touch an empty cell (or the edge)."""
    out = [[False] * COLS for _ in range(ROWS)]
    for r in range(ROWS):
        for c in range(COLS):
            if not g[r][c]:
                continue
            for dr, dc in ((1, 0), (-1, 0), (0, 1), (0, -1)):
                rr, cc = r + dr, c + dc
                if not (0 <= rr < ROWS and 0 <= cc < COLS) or not g[rr][cc]:
                    out[r][c] = True
                    break
    return out


def ellipse(cx, cy, rx, ry):
    return lambda x, y: ((x - cx) / rx) ** 2 + ((y - cy) / ry) ** 2 <= 1.0


def circle(cx, cy, r):
    return ellipse(cx, cy, r, r)


def rrect(x0, y0, x1, y1, r):
    def f(x, y):
        if not (x0 <= x <= x1 and y0 <= y <= y1):
            return False
        cx = min(max(x, x0 + r), x1 - r)
        cy = min(max(y, y0 + r), y1 - r)
        return (x - cx) ** 2 + (y - cy) ** 2 <= r * r
    return f


def capsule(pts, r):
    def f(x, y):
        for i in range(len(pts) - 1):
            ax, ay = pts[i]
            bx, by = pts[i + 1]
            dx, dy = bx - ax, by - ay
            L2 = dx * dx + dy * dy
            t = 0.0 if L2 == 0 else max(0.0, min(1.0, ((x - ax) * dx + (y - ay) * dy) / L2))
            px, py = ax + t * dx, ay + t * dy
            if (x - px) ** 2 + (y - py) ** 2 <= r * r:
                return True
        return False
    return f


def blob(cx, cy, rx, ry, terms):
    """A smooth lobed disc — a union of circles gives a jagged outline."""
    def f(x, y):
        dx, dy = (x - cx) / rx, (y - cy) / ry
        th = math.atan2(dy, dx)
        lim = 1.0
        for k, amp, ph in terms:
            lim += amp * math.cos(k * th + ph)
        return math.hypot(dx, dy) <= lim
    return f


def tri(p0, p1, p2):
    def sign(a, b, c):
        return (a[0] - c[0]) * (b[1] - c[1]) - (b[0] - c[0]) * (a[1] - c[1])

    def f(x, y):
        p = (x, y)
        d1, d2, d3 = sign(p, p0, p1), sign(p, p1, p2), sign(p, p2, p0)
        return not (((d1 < 0) or (d2 < 0) or (d3 < 0)) and ((d1 > 0) or (d2 > 0) or (d3 > 0)))

    return f


# -- brain: a lobed cerebrum, folds, a cerebellum and a stem -----------
def wave(x0, x1, base, amp, freq, phase, r):
    pts = []
    n = 34
    for i in range(n + 1):
        x = x0 + (x1 - x0) * i / n
        pts.append((x, base + amp * math.sin(freq * x + phase)))
    return capsule(pts, r)


BRAIN_LOBES = [(3, 0.07, 0.7), (5, 0.045, 2.2)]
brain_solid = rasterize(
    blob(9.4, 7.9, 7.7, 5.9, BRAIN_LOBES),
    capsule([(10.2, 12.2), (10.8, 16.8)], 1.1),
)
brain_inside = rasterize(blob(9.4, 7.9, 6.2, 4.4, BRAIN_LOBES))
brain = union(
    outline(brain_solid),
    intersect(
        rasterize(
            wave(1.0, 18.0, 6.2, 0.85, 0.8, 0.4, 0.3),
            wave(1.0, 18.0, 9.9, 0.85, 0.8, 2.6, 0.3),
        ),
        brain_inside,
    ),
)

# -- heart -------------------------------------------------------------
heart = outline(
    rasterize(
        circle(6.6, 7.0, 4.7),
        circle(14.5, 7.0, 4.7),
        tri((1.95, 7.4), (19.15, 7.4), (10.55, 18.8)),
    )
)

# -- robot -------------------------------------------------------------
robot_solid = rasterize(
    rrect(3.8, 5.2, 17.2, 16.2, 2.6),
    rrect(0.9, 8.8, 3.0, 12.2, 0.8),
    rrect(18.0, 8.8, 20.1, 12.2, 0.8),
    capsule([(10.5, 5.2), (10.5, 2.4)], 0.34),
    circle(10.5, 1.5, 1.2),
)
robot = union(
    outline(robot_solid),
    rasterize(
        circle(7.6, 9.4, 1.15),
        circle(13.4, 9.4, 1.15),
        capsule([(7.4, 13.4), (13.6, 13.4)], 0.34),
    ),
)

# -- mirror ------------------------------------------------------------
mirror_solid = rasterize(
    ellipse(10.5, 7.4, 7.2, 6.4),
    rrect(9.3, 12.6, 11.7, 19.4, 1.1),
)
mirror = union(
    outline(mirror_solid),
    intersect(outline(rasterize(ellipse(10.5, 7.4, 5.5, 4.8))), mirror_solid),
    intersect(
        rasterize(
            capsule([(6.9, 9.8), (10.4, 4.4)], 0.34),
            capsule([(9.0, 10.4), (11.8, 6.2)], 0.28),
        ),
        rasterize(ellipse(10.5, 7.4, 5.1, 4.4)),
    ),
)


def show(name, g):
    print("--- %s ---" % name)
    for row in g:
        print("".join("#" if v else "." for v in row))
    print()


def as_text(g):
    return ["".join("#" if v else "." for v in row) for row in g]


SHAPES = (("brain", brain), ("heart", heart), ("robot", robot), ("mirror", mirror))

OUT = os.path.join(
    os.path.dirname(os.path.abspath(__file__)), "..", "src", "scripts", "glyph-masks.ts"
)

buf = io.StringIO()
buf.write("/* GENERATED FILE - do not edit by hand.\n")
buf.write(" * Source of truth: apps/site/scripts/gen-glyph-masks.py\n")
buf.write(" *\n")
buf.write(
    " * The four figures the digital rain settles into. Each is a %d x %d grid\n"
    % (COLS, ROWS)
)
buf.write(" * sampled in SQUARE units, so the mono cell's aspect does not stretch them.\n")
buf.write(" * They are HOLLOW outlines: at this size a filled silhouette with carved\n")
buf.write(" * detail turns to confetti, and a hollow figure lets the rain fall through.\n")
buf.write(" */\n\n")
buf.write("export const GRID_COLS = %d;\n" % COLS)
buf.write("export const GRID_ROWS = %d;\n\n" % ROWS)
buf.write("export type GlyphKey = %s;\n\n" % " | ".join('"%s"' % n for n, _ in SHAPES))
buf.write("export const MASKS: Record<GlyphKey, readonly string[]> = {\n")
for _name, _g in SHAPES:
    buf.write("  %s: [\n" % _name)
    for _row in as_text(_g):
        buf.write('    "%s",\n' % _row)
    buf.write("  ],\n")
buf.write("};\n")

with open(OUT, "w") as fh:
    fh.write(buf.getvalue())

for _name, _g in SHAPES:
    show(_name, _g)
print("wrote", os.path.normpath(OUT))

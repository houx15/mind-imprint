/**
 * The waterfall, and the four figures that grow in it.
 *
 * The hero is water. It does not stop at the hero: it pours over the bottom of
 * it and falls through the screen below as a sheet of digits, in the hero's own
 * colours, with no seam between the two.
 *
 * The four figures are NOT drawn on top of the rain — they are made of it. A
 * cell of a figure only appears once a falling drop has actually passed through
 * it, so the shapes accumulate under the waterfall the way silt does. They
 * settle small, at the foot of the fall, and shine.
 *
 * Two canvases share one coordinate system: the rain erases itself with a
 * destination-out fade (which is what draws the trails), while the figures must
 * persist, and want a glow the rain must not get.
 *
 * Where one of the four has to appear standing still — beside its ability, on
 * the rail next to the dial, in the closing row — it is drawn from the same
 * masks as a static SVG (GlyphMark.astro), not by this.
 */
import { MASKS, GRID_COLS, GRID_ROWS, type GlyphKey } from "./glyph-masks";

const ORDER: readonly GlyphKey[] = ["brain", "heart", "robot", "mirror"];

/** One hue each — used for the copy's dots, and to pick a figure out on hover. */
const HUE: Record<GlyphKey, string> = {
  brain: "110, 168, 255",
  heart: "240, 148, 111",
  robot: "63, 214, 168",
  mirror: "240, 179, 84",
};

/* The hero's water, sampled from its own gradient: a deep teal that runs to
   aquamarine. Everything below falls in the same key, which is what makes the
   two screens read as one body of water. */
const TRAIL = "rgba(66, 224, 180, 0.34)";
const HEAD = "rgba(198, 255, 236, 0.96)";
const FIGURE = "214, 255, 240";

const DIGITS = "0123456789";
/** A mono cell is this much wider than it is tall. The masks assume it. */
const CELL_ASPECT = 0.6;

type Cell = { cx: number; cy: number; rank: number; seed: number };

/** Deterministic hash, so a cell keeps its character and its place in the queue. */
function hash(a: number, b: number): number {
  let h = (a * 374761393 + b * 668265263) | 0;
  h = (h ^ (h >>> 13)) * 1274126177;
  return ((h ^ (h >>> 16)) >>> 0) / 4294967296;
}

const CELLS: Record<GlyphKey, Cell[]> = Object.create(null);
for (const key of ORDER) {
  const rows = MASKS[key];
  const cells: Cell[] = [];
  for (let y = 0; y < rows.length; y++) {
    const row = rows[y];
    for (let x = 0; x < row.length; x++) {
      if (row[x] !== "#") continue;
      const h = hash(x, y);
      cells.push({
        cx: x,
        cy: y,
        // Water fills from the top down — but not in rows, or it reads as a
        // wipe rather than as an accumulation.
        rank: 0.5 * (y / GRID_ROWS) + 0.5 * h,
        seed: Math.floor(h * 1000),
      });
    }
  }
  CELLS[key] = cells;
}

const clamp01 = (v: number) => (v < 0 ? 0 : v > 1 ? 1 : v);
const smooth = (v: number) => v * v * (3 - 2 * v);

type Placed = { key: GlyphKey; ox: number; oy: number };

/** The four in a row, left to right, in the order the abilities are listed. */
function place(): { cols: number; rows: number; at: Placed[] } {
  const GAP_X = 5;
  return {
    cols: GRID_COLS * 4 + GAP_X * 3,
    rows: GRID_ROWS,
    at: ORDER.map((key, i) => ({ key, ox: i * (GRID_COLS + GAP_X), oy: 0 })),
  };
}

type Drop = { col: number; y: number; sp: number };

/**
 * Where the water goes over the edge, per column.
 *
 * A fall with a flat top edge is a curtain, not a waterfall — you have to be
 * able to see where it comes from. The far-left columns pour from high up
 * inside the hero; by a tenth of the way across, the lip has dropped to the
 * top of this section. That tenth is to the left of the hero's copy at every
 * width, so nothing is ever fallen on.
 */
function lipAt(x: number, flat: number, ramp: number, bleed: number): number {
  if (x <= flat) return 0;
  if (ramp <= 0) return bleed;
  return bleed * Math.min(1, (x - flat) / ramp);
}

interface Options {
  /** The element whose scroll position drives the fall. */
  driver?: HTMLElement | null;
  /** Lit one at a time as their figure grows; also the hover targets. */
  items?: HTMLElement[];
  /** The column the four should settle under. Measured, not guessed at. */
  figBox?: HTMLElement | null;
}

function mount(host: HTMLElement, opts: Options): void {
  const reduced = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
  const rainCv = host.querySelector<HTMLCanvasElement>(".gr-rain");
  const figCv = host.querySelector<HTMLCanvasElement>(".gr-figs");
  if (!figCv) return;
  const fctx = figCv.getContext("2d");
  const rctx = rainCv?.getContext("2d") ?? null;
  if (!fctx || !rctx) return;

  const items = opts.items ?? [];

  let gCols = 0;
  let gRows = 0;
  let at: Placed[] = [];

  let W = 0;
  let H = 0;
  let cw = 0; // figure cell
  let ch = 0;
  let figX = 0;
  let figY = 0;
  let rcw = 0; // rain cell — deliberately much larger
  let rch = 0;
  let rainCols = 0;
  let rainRows = 0;
  /** How far the canvas reaches up past the section, in rain rows. */
  let bleedRows = 0;
  /** How far left of the hero's copy the fall may climb, in px. */
  let lipFlat = 0;
  let lipRamp = 0;
  let drops: Drop[] = [];
  let dpr = 1;

  const eps = ORDER.map(() => 0);
  /** Which cells the water has reached, and what digit it left there. */
  let grown: Int16Array[] = [];
  /** When each landed — a cell drops the last little way rather than blinking on. */
  let born: Float64Array[] = [];
  /** Figure-grid lookup, so a falling drop can find what it just hit. */
  let index = new Map<number, [number, number]>();
  let focus: number | null = null;
  let lit = -1;

  function size(): void {
    const r = figCv.getBoundingClientRect();
    if (!r.width || !r.height) return;
    dpr = Math.min(2, window.devicePixelRatio || 1);
    W = Math.round(r.width);
    H = Math.round(r.height);

    ({ cols: gCols, rows: gRows, at } = place());

    for (const cv of [rainCv, figCv]) {
      if (!cv) continue;
      cv.width = Math.round(W * dpr);
      cv.height = Math.round(H * dpr);
      cv.getContext("2d")?.setTransform(dpr, 0, 0, dpr, 0, 0);
    }

    // Small, and at the foot of the fall: they are what the water left, not an
    // illustration of it. The canvas runs the full width of the page, so the
    // column they settle under is measured rather than guessed at.
    const cRect = figCv.getBoundingClientRect();
    const bRect = opts.figBox?.getBoundingClientRect();
    const colW = bRect ? bRect.width : W * 0.42;
    const boxL = bRect ? bRect.left - cRect.left : W * 0.05;
    // The floor is the bottom of that column, not the bottom of the canvas.
    // On a narrow screen the canvas runs the whole page and the copy is BELOW
    // the column — the four must not end up behind the words.
    const boxB = bRect ? bRect.bottom - cRect.top : H * 0.9;
    ch = Math.min((H * 0.19) / gRows, (colW * 0.86) / gCols / CELL_ASPECT);
    cw = ch * CELL_ASPECT;
    figX = boxL + (colW - gCols * cw) / 2;
    figY = boxB - gRows * ch;

    // The waterfall keeps its own, much larger character size. Tying it to the
    // figures would shrink the weather every time the figures got smaller.
    rch = Math.max(13, Math.min(18, H / 48));
    rcw = rch * CELL_ASPECT;
    rainCols = Math.ceil(W / rcw) + 1;
    rainRows = Math.ceil(H / rch) + 2;
    const cRect2 = figCv.getBoundingClientRect();
    const dRect = opts.driver?.getBoundingClientRect();
    bleedRows = dRect ? Math.max(0, (dRect.top - cRect2.top) / rch) : 0;
    // The ribbon that climbs into the hero must stay to the left of the hero's
    // own words at every width, so it is measured off the headline itself.
    const title = document.querySelector(".hi-title");
    const safe = title
      ? title.getBoundingClientRect().left - cRect2.left - 14
      : W * 0.08;
    lipFlat = Math.max(0, Math.min(safe, W * 0.1));
    lipRamp = lipFlat * 0.5;

    drops = [];
    {
      // Dense enough to be a fall rather than a drizzle, but a wall of type
      // with no gaps in it stops looking like water at all: the dark between
      // the streaks is what makes it read as falling.
      const n = Math.round(rainCols * 1.35);
      for (let i = 0; i < n; i++) {
        drops.push({
          col: Math.floor(Math.random() * rainCols),
          y: -Math.random() * rainRows,
          sp: 1.3 + Math.random() * 1.9,
        });
      }
    }

    grown = at.map(({ key }) => new Int16Array(CELLS[key].length).fill(-1));
    born = at.map(({ key }) => new Float64Array(CELLS[key].length));
    index = new Map();
    at.forEach(({ key, ox, oy }, gi) => {
      CELLS[key].forEach((c, ci) => {
        index.set((oy + c.cy) * 4096 + (ox + c.cx), [gi, ci]);
      });
    });
    rctx.clearRect(0, 0, W, H);
  }

  function font(ctx: CanvasRenderingContext2D, size: number, weight = 500): void {
    ctx.font = `${weight} ${size * 0.94}px "JetBrains Mono", ui-monospace, monospace`;
    ctx.textAlign = "center";
    ctx.textBaseline = "middle";
  }

  /** A drop just passed through this patch of screen: grow whatever is under it. */
  function wet(px: number, py: number): void {
    const fx0 = Math.floor((px - rcw / 2 - figX) / cw);
    const fx1 = Math.floor((px + rcw / 2 - figX) / cw);
    const fy0 = Math.floor((py - rch / 2 - figY) / ch);
    const fy1 = Math.floor((py + rch / 2 - figY) / ch);
    for (let fy = fy0; fy <= fy1; fy++) {
      if (fy < 0 || fy >= gRows) continue;
      for (let fx = fx0; fx <= fx1; fx++) {
        if (fx < 0 || fx >= gCols) continue;
        const found = index.get(fy * 4096 + fx);
        if (!found) continue;
        const [gi, ci] = found;
        if (grown[gi][ci] >= 0) continue;
        if (CELLS[at[gi].key][ci].rank > eps[gi]) continue;
        grown[gi][ci] = Math.floor(Math.random() * 10);
        born[gi][ci] = performance.now();
      }
    }
  }

  /** How long a cell takes to fall the last little way into place. */
  const DROP = 520;

  function drawFigures(now: number): void {
    fctx.clearRect(0, 0, W, H);
    font(fctx, ch * 1.18, 700);
    for (let gi = 0; gi < at.length; gi++) {
      const { key, ox, oy } = at[gi];
      // The figure belonging to the ability currently OPEN on the right takes
      // its hue without anyone having to hover — that is what tells the reader
      // the two columns are one thing. A pointer overrides it.
      const active = focus ?? lit;
      const focused = active === gi && active >= 0;
      const alpha = focused ? 1 : focus === null ? 0.6 : 0.28;
      const settled = focused ? `rgba(${HUE[key]}, 1)` : `rgba(${FIGURE}, ${alpha})`;
      const cells = CELLS[key];

      // Settled cells share one fillStyle; only the handful still falling need
      // their own, which is what keeps this cheap enough to run every frame.
      fctx.fillStyle = settled;
      const falling: number[] = [];
      for (let ci = 0; ci < cells.length; ci++) {
        const seeded = grown[gi][ci];
        if (seeded < 0) continue;
        const age = now - born[gi][ci];
        if (age < DROP) {
          falling.push(ci);
          continue;
        }
        const c = cells[ci];
        // It keeps flickering: it is still made of water, not set in stone.
        const step = Math.floor(now / (420 + (c.seed % 6) * 260));
        fctx.fillText(
          DIGITS[(seeded + step) % 10],
          figX + (ox + c.cx + 0.5) * cw,
          figY + (oy + c.cy + 0.5) * ch,
        );
      }

      for (const ci of falling) {
        const c = cells[ci];
        const e = 1 - Math.pow(1 - (now - born[gi][ci]) / DROP, 3);
        fctx.globalAlpha = 0.1 + 0.9 * e;
        fctx.fillText(
          DIGITS[grown[gi][ci]],
          figX + (ox + c.cx + 0.5) * cw,
          figY + (oy + c.cy + 0.5) * ch - (1 - e) * ch * 2.6,
        );
      }
      fctx.globalAlpha = 1;
    }
  }

  function drawRain(enter: number, through: number): void {
    if (!rctx) return;
    // Erase rather than paint over: the canvas stays transparent, so it sits on
    // any background, and the erasure IS the trail.
    rctx.globalCompositeOperation = "destination-out";
    rctx.fillStyle = "rgba(0, 0, 0, 0.11)";
    rctx.fillRect(0, 0, W, H);
    rctx.globalCompositeOperation = "source-over";
    font(rctx, rch);

    // It pours over the left corner first and widens into a sheet; once the
    // four are standing it thins, but it never stops.
    const reach = 0.2 + enter * 2.2;
    // It settles once the four are standing, but it never stops.
    const thin = 1 - 0.42 * smooth(clamp01((through - 0.72) / 0.28));

    for (const d of drops) {
      if (d.col / rainCols > reach) continue;
      const prev = d.y;
      d.y += d.sp;
      if (Math.random() > thin) continue;
      const x = (d.col + 0.5) * rcw;
      const lip = lipAt(x, lipFlat, lipRamp, bleedRows);
      for (let r = Math.floor(prev) + 1; r <= Math.floor(d.y); r++) {
        if (r < lip || r > rainRows) continue;
        const y = (r + 0.5) * rch;
        const head = r === Math.floor(d.y);
        rctx.fillStyle = head ? HEAD : TRAIL;
        rctx.fillText(DIGITS[Math.floor(Math.random() * 10)], x, y);
        wet(x, y);
      }
      if (d.y > rainRows) {
        d.col = Math.floor(Math.random() * rainCols);
        d.y =
          lipAt((d.col + 0.5) * rcw, lipFlat, lipRamp, bleedRows) -
          Math.random() * 8;
        d.sp = 1.3 + Math.random() * 1.9;
      }
    }
  }

  /**
   * Two clocks, not one.
   *
   * `enter` is how far the screen has arrived — 0 with its top at the foot of
   * the viewport, 1 with its top at the head. The fall widens on that, so the
   * water is already pouring by the time you get here.
   *
   * `through` is how far you are through the pinned part of it. The figures
   * grow on that, so each of the four gets a real beat of scroll to stand up
   * in and to be read about, instead of all four finishing before the screen
   * has even settled.
   */
  function progress(): { enter: number; through: number } {
    const driver = opts.driver;
    if (!driver) return { enter: 1, through: 1 };
    const r = driver.getBoundingClientRect();
    const vh = window.innerHeight || 1;
    return {
      enter: clamp01((vh - r.top) / vh),
      through: clamp01(-r.top / Math.max(1, r.height - vh)),
    };
  }

  function form(p: number): void {
    let next = -1;
    for (let i = 0; i < eps.length; i++) {
      eps[i] = smooth(clamp01((p - (0.06 + i * 0.21)) / 0.2));
      if (eps[i] > 0.5) next = i;
      // The water is not always thorough. Past the point where a figure should
      // be complete, finish it, so nothing is left half-grown on the page.
      if (eps[i] > 0.97) {
        const cells = CELLS[at[i].key];
        for (let ci = 0; ci < cells.length; ci++) {
          if (grown[i][ci] >= 0) continue;
          grown[i][ci] = Math.floor(Math.random() * 10);
          born[i][ci] = performance.now();
        }
      }
    }
    if (next !== lit) {
      lit = next;
      items.forEach((el, i) => {
        // arrived: its figure has grown. open: it is the one being read now.
        el.classList.toggle("on", i <= lit);
        el.classList.toggle("open", i === lit);
      });
    }
  }

  let raf = 0;
  let visible = true;
  function frame(now: number): void {
    raf = 0;
    if (!visible) return;
    const { enter, through } = progress();
    form(through);
    drawRain(enter, through);
    drawFigures(now);
    raf = requestAnimationFrame(frame);
  }

  function start(): void {
    if (raf || reduced) return;
    raf = requestAnimationFrame(frame);
  }

  size();
  if (reduced) {
    eps.fill(1);
    grown.forEach((g) => g.fill(0));
    born.forEach((b) => b.fill(-1e9));
    items.forEach((el, i) => {
      el.classList.add("on");
      el.classList.toggle("open", i === 0);
    });
    drawFigures(0);
  } else {
    start();
  }

  // Hovering an ability picks its figure out of the four.
  items.forEach((el, i) => {
    const on = () => {
      focus = i;
      if (reduced) drawFigures(0);
    };
    const off = () => {
      if (focus === i) focus = null;
      if (reduced) drawFigures(0);
    };
    el.addEventListener("mouseenter", on);
    el.addEventListener("focusin", on);
    el.addEventListener("mouseleave", off);
    el.addEventListener("focusout", off);
  });

  const io = new IntersectionObserver(
    (entries) => {
      visible = entries.some((e) => e.isIntersecting);
      if (visible) start();
    },
    { rootMargin: "160px" },
  );
  io.observe(host);

  let resizeT = 0;
  window.addEventListener(
    "resize",
    () => {
      window.clearTimeout(resizeT);
      resizeT = window.setTimeout(() => {
        size();
        if (reduced) {
          grown.forEach((g) => g.fill(0));
          born.forEach((b) => b.fill(-1e9));
          drawFigures(0);
        }
      }, 160);
    },
    { passive: true },
  );
}

function init(): void {
  document.querySelectorAll<HTMLElement>("[data-glyphrain]").forEach((host) => {
    const section = host.closest<HTMLElement>("[data-mission]");
    mount(host, {
      driver: section,
      figBox: section?.querySelector<HTMLElement>("[data-figbox]") ?? null,
      items: section
        ? Array.from(section.querySelectorAll<HTMLElement>("[data-glyph-item]"))
        : [],
    });
  });
}

if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", init, { once: true });
} else {
  init();
}

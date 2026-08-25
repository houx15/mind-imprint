/**
 * The waterfall on the second screen, and the order the screen unfolds in.
 *
 * The hero is water. It does not stop at the hero: it pours over the bottom of
 * it and falls through this screen as a sheet of digits, in the hero's own
 * colours, with no seam between the two. Then, one at a time, the four figures
 * appear in it — and only once they are standing do the abilities on the right
 * arrive to say what each of them is.
 *
 * ── what this used to cost ──────────────────────────────────────────────────
 * The four were grown out of the falling water itself: every drop hit-tested
 * against a 167x22 cell grid, and every cell of every figure was redrawn as a
 * character sixty times a second, on a second full-page canvas under three
 * stacked drop-shadows. It held 60fps and still cooked the machine.
 *
 * The figures are pixel art either way. So they are SVG now (GlyphMark), which
 * costs nothing at rest and reveals with one CSS transition — and this file is
 * left with two jobs: paint the water, and say when things appear.
 *
 * What remains is deliberately modest: one canvas, sized to the band it falls
 * through rather than to the page, painted at 1x. It is a texture, not type;
 * nobody reads it, and at device resolution it costs four times as much to say
 * exactly the same thing.
 */
const DIGITS = "0123456789";
/** A mono cell is this much wider than it is tall. */
const CELL_ASPECT = 0.6;

/* The hero's water, sampled from its own gradient: a deep teal running to
   aquamarine. Everything below falls in the same key, which is what makes the
   two screens read as one body of water. */
const TRAIL = "rgba(66, 224, 180, 0.34)";
const HEAD = "rgba(198, 255, 236, 0.96)";

/* The two halves of the screen, in order: the four appear, and only then do the
   abilities arrive to name them. */
const SHOW_FROM = 0.08;
const SHOW_STEP = 0.1;
const TELL_FROM = 0.54;
const TELL_STEP = 0.11;

const clamp01 = (v: number) => (v < 0 ? 0 : v > 1 ? 1 : v);
const smooth = (v: number) => v * v * (3 - 2 * v);

type Drop = { col: number; y: number; sp: number };

/**
 * Where the water goes over the edge, per column.
 *
 * A fall with a flat top edge is a curtain, not a waterfall — you have to be
 * able to see where it comes from. The leftmost columns pour from high up
 * inside the hero; a little way across, the lip has dropped to the top of this
 * screen. That strip is to the left of the hero's copy at every width, so
 * nothing is ever fallen on.
 */
function lipAt(x: number, flat: number, ramp: number, bleed: number): number {
  if (x <= flat) return 0;
  if (ramp <= 0) return bleed;
  return bleed * Math.min(1, (x - flat) / ramp);
}

function mount(section: HTMLElement): void {
  const reduced = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
  const cv = section.querySelector<HTMLCanvasElement>(".gr-rain");
  const ctx = cv?.getContext("2d", { alpha: true });
  const figs = Array.from(section.querySelectorAll<HTMLElement>("[data-glyph-fig]"));
  const items = Array.from(section.querySelectorAll<HTMLElement>("[data-glyph-item]"));

  /** Nothing pins on a narrow screen, so nothing can be paced by scrolling it. */
  const narrow = () => window.innerWidth <= 980;

  /** Reveal the four, then name them. Runs with or without the canvas. */
  let shown = -1;
  let lit = -1;
  let dropped = false;
  function dropFigs(): void {
    if (dropped) return;
    dropped = true;
    figs.forEach((el, i) => window.setTimeout(() => el.classList.add("on"), i * 170));
  }

  function sequence(p: number): void {
    // On a narrow screen the four sit in a band at the top of the section,
    // which has scrolled past before any scroll-driven cue could fire. There
    // they simply drop in, in order, as soon as the screen is reached.
    if (narrow()) {
      dropFigs();
    } else {
      let nextShown = -1;
      for (let i = 0; i < figs.length; i++) {
        if (p >= SHOW_FROM + i * SHOW_STEP) nextShown = i;
      }
      if (nextShown !== shown) {
        shown = nextShown;
        figs.forEach((el, i) => el.classList.toggle("on", i <= shown));
      }
    }

    let nextLit = -1;
    for (let i = 0; i < items.length; i++) {
      if (p >= TELL_FROM + i * TELL_STEP) nextLit = i;
    }
    if (nextLit === lit) return;
    lit = nextLit;
    items.forEach((el, i) => {
      // arrived: it has been named. open: it is the one being read now.
      el.classList.toggle("on", i <= lit);
      el.classList.toggle("open", i === lit);
    });
  }

  if (reduced) {
    figs.forEach((el) => el.classList.add("on"));
    items.forEach((el, i) => {
      el.classList.add("on");
      el.classList.toggle("open", i === 0);
    });
    return;
  }

  let W = 0;
  let H = 0;
  let cw = 0;
  let chh = 0;
  let cols = 0;
  let rows = 0;
  let bleedRows = 0;
  let lipFlat = 0;
  let lipRamp = 0;
  /** How many rows above the spill are already numbers, right across. */
  let bandRows = 0;

  /** Weighted to the left: that is the corner the water goes over. */
  const pickCol = () => Math.floor(Math.pow(Math.random(), 1.7) * cols);
  let drops: Drop[] = [];

  function size(): void {
    if (!cv || !ctx) return;
    const r = cv.getBoundingClientRect();
    if (!r.width || !r.height) return;
    W = Math.round(r.width);
    H = Math.round(r.height);
    cv.width = W;
    cv.height = H;
    ctx.setTransform(1, 0, 0, 1, 0, 0);
    ctx.clearRect(0, 0, W, H);

    chh = Math.max(13, Math.min(18, H / 48));
    cw = chh * CELL_ASPECT;
    cols = Math.ceil(W / cw) + 1;
    rows = Math.ceil(H / chh) + 2;

    bleedRows = Math.max(0, (section.getBoundingClientRect().top - r.top) / chh);
    // The strip that climbs into the hero is measured off the hero's own
    // headline, so it can never fall on the words at any width.
    const title = document.querySelector(".hi-title");
    const safe = title ? title.getBoundingClientRect().left - r.left - 14 : W * 0.18;
    lipFlat = Math.max(0, Math.min(safe, W * 0.22));
    lipRamp = lipFlat * 0.5;
    // The hero's copy ends well above its bottom edge, and that last band is
    // where the water is about to go over. Letting every column carry digits
    // there is what makes the fall look like it comes OUT of the hero rather
    // than starting underneath it.
    bandRows = Math.min(bleedRows, 80 / chh);

    drops = [];
    const n = Math.round(cols * 1.35);
    for (let i = 0; i < n; i++) {
      drops.push({
        col: pickCol(),
        y: -Math.random() * rows,
        sp: 1.3 + Math.random() * 1.9,
      });
    }
  }

  function draw(enter: number, through: number): void {
    if (!ctx) return;
    // Erase rather than paint over: the canvas stays transparent, so it sits on
    // any background, and the erasure IS the trail.
    ctx.globalCompositeOperation = "destination-out";
    ctx.fillStyle = "rgba(0, 0, 0, 0.11)";
    // Two rects, not one: high up inside the hero only the left ribbon can
    // carry anything, and the empty rest of that band is most of the canvas.
    const spill = (bleedRows - bandRows) * chh;
    ctx.fillRect(0, 0, lipFlat + lipRamp + cw, spill);
    ctx.fillRect(0, spill, W, H - spill);
    ctx.globalCompositeOperation = "source-over";
    ctx.font = `500 ${chh * 0.94}px "JetBrains Mono", ui-monospace, monospace`;
    ctx.textAlign = "center";
    ctx.textBaseline = "middle";

    // It pours over the left corner first and widens into a sheet; once the
    // four are standing it thins, but it never stops.
    const reach = 0.2 + enter * 2.2;
    const thin = 1 - 0.42 * smooth(clamp01((through - 0.45) / 0.3));

    for (const d of drops) {
      const at = d.col / cols;
      if (at > reach) continue;
      // Feathered, or the sheet has a ruled vertical edge where it happens to
      // have got to.
      if (at > reach - 0.07 && Math.random() > (reach - at) / 0.07) continue;
      const prev = d.y;
      d.y += d.sp;
      if (Math.random() > thin) continue;
      const x = (d.col + 0.5) * cw;
      const lip = Math.min(lipAt(x, lipFlat, lipRamp, bleedRows), bleedRows - bandRows);
      for (let r = Math.floor(prev) + 1; r <= Math.floor(d.y); r++) {
        if (r < lip || r > rows) continue;
        ctx.fillStyle = r === Math.floor(d.y) ? HEAD : TRAIL;
        ctx.fillText(DIGITS[Math.floor(Math.random() * 10)], x, (r + 0.5) * chh);
      }
      if (d.y > rows) {
        d.col = pickCol();
        d.y =
          Math.min(
            lipAt((d.col + 0.5) * cw, lipFlat, lipRamp, bleedRows),
            bleedRows - bandRows,
          ) - Math.random() * 8;
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
   * `through` is how far you are through the pinned part of it, and it runs the
   * sequence: the four appear, then the abilities arrive to name them.
   */
  let raf = 0;
  let visible = true;
  function frame(): void {
    raf = 0;
    if (!visible) return;
    const r = section.getBoundingClientRect();
    const vh = window.innerHeight || 1;
    const enter = clamp01((vh - r.top) / vh);
    const through = clamp01(-r.top / Math.max(1, r.height - vh));
    sequence(through);
    draw(enter, through);
    raf = requestAnimationFrame(frame);
  }

  function start(): void {
    if (!raf) raf = requestAnimationFrame(frame);
  }

  size();
  start();

  const io = new IntersectionObserver(
    (entries) => {
      visible = entries.some((e) => e.isIntersecting);
      if (visible) start();
    },
    { rootMargin: "160px" },
  );
  io.observe(section);

  let resizeT = 0;
  window.addEventListener(
    "resize",
    () => {
      window.clearTimeout(resizeT);
      resizeT = window.setTimeout(size, 160);
    },
    { passive: true },
  );
}

function init(): void {
  document
    .querySelectorAll<HTMLElement>("[data-mission]")
    .forEach((section) => mount(section));
}

if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", init, { once: true });
} else {
  init();
}

/**
 * The immersive hero's behaviour: water, the explorer, and the nav reveal.
 *
 * All of it is progressive enhancement. Without this script the hero still
 * renders — painted water, the explorer at its resting place, readable copy,
 * and a nav that is simply always visible.
 */

const reduced = window.matchMedia("(prefers-reduced-motion: reduce)").matches;

const hero = document.querySelector<HTMLElement>("[data-hero-water]");

if (hero) {
  const canvas = hero.querySelector<HTMLCanvasElement>("[data-hero-canvas]");
  const boat = hero.querySelector<HTMLElement>("[data-hero-boat]");
  const photo = hero.querySelector<HTMLImageElement>("[data-hero-photo]");

  // --- the photograph fades in only once it has really decoded -------------
  if (photo) {
    const ready = () => photo.classList.add("ready");
    if (photo.complete && photo.naturalWidth > 0) ready();
    else photo.addEventListener("load", ready, { once: true });
    // On error the painted water simply stays: nothing to do, nothing broken.
  }

  // --- the second screen, for the one button in the hero -------------------
  hero.querySelector<HTMLElement>("[data-hero-next]")?.addEventListener("click", () => {
    // Astro leaves this component's own <script> inline right after the
    // section, so the immediate next sibling is not the next screen — walk on
    // until an actual section turns up.
    let next = hero.nextElementSibling;
    while (next && next.tagName !== "SECTION") next = next.nextElementSibling;
    if (!next) return;
    next.scrollIntoView({ behavior: reduced ? "auto" : "smooth", block: "start" });
  });

  // --- the nav only arrives once the opening has been passed ---------------
  if (document.body.classList.contains("immersive")) {
    const nav = document.querySelector<HTMLElement>("nav.site-nav");
    if (nav) {
      const onScroll = () => {
        nav.classList.toggle("nav-on", window.scrollY > window.innerHeight * 0.5);
      };
      onScroll();
      window.addEventListener("scroll", onScroll, { passive: true });
    }
  }

  // -------------------------------------------------------------------------
  // Water + explorer
  // -------------------------------------------------------------------------
  // The water is a damped height field on a coarse grid. Coarseness is the
  // point: the canvas backing store is a fraction of the hero's size and the
  // browser's own upscaling is what turns discrete cells into soft light on a
  // moving surface. Everything below is a few hundred KB of arithmetic per
  // frame, which is cheap enough to run at 60fps on a laptop.
  if (canvas && boat && !reduced) {
    const ctx = canvas.getContext("2d", { alpha: true });

    const CELL = 6; // css px per simulation cell
    const DAMP = 0.967; // how fast a ripple dies away
    let W = 0;
    let H = 0;
    let cur = new Float32Array(0);
    let prv = new Float32Array(0);
    let field: ImageData | null = null;
    let heroW = 0;
    let heroH = 0;

    const measure = () => {
      const r = hero.getBoundingClientRect();
      heroW = Math.max(1, r.width);
      heroH = Math.max(1, r.height);
      const nw = Math.max(8, Math.round(heroW / CELL));
      const nh = Math.max(8, Math.round(heroH / CELL));
      if (nw === W && nh === H) return;
      W = nw;
      H = nh;
      cur = new Float32Array(W * H);
      prv = new Float32Array(W * H);
      canvas.width = W;
      canvas.height = H;
      field = ctx ? ctx.createImageData(W, H) : null;
    };
    measure();

    /** Push the surface down at a point, in hero-local css pixels. */
    const disturb = (px: number, py: number, power: number, radius: number) => {
      const gx = Math.round(px / CELL);
      const gy = Math.round(py / CELL);
      for (let dy = -radius; dy <= radius; dy++) {
        const ny = gy + dy;
        if (ny < 1 || ny >= H - 1) continue;
        for (let dx = -radius; dx <= radius; dx++) {
          const nx = gx + dx;
          if (nx < 1 || nx >= W - 1) continue;
          const dist = Math.sqrt(dx * dx + dy * dy);
          if (dist > radius) continue;
          cur[ny * W + nx] += power * (1 - dist / radius);
        }
      }
    };

    const step = () => {
      for (let y = 1; y < H - 1; y++) {
        const row = y * W;
        for (let x = 1; x < W - 1; x++) {
          const i = row + x;
          prv[i] =
            ((cur[i - 1] + cur[i + 1] + cur[i - W] + cur[i + W]) * 0.5 - prv[i]) * DAMP;
        }
      }
      const swap = cur;
      cur = prv;
      prv = swap;
    };

    const paint = () => {
      if (!ctx || !field) return;
      const d = field.data;
      d.fill(0);
      for (let y = 1; y < H - 1; y++) {
        const row = y * W;
        for (let x = 1; x < W - 1; x++) {
          const i = row + x;
          // The slope of the surface is what catches (or loses) the light.
          const s = cur[i - 1] - cur[i + 1] + (cur[i - W] - cur[i + W]);
          if (s > 0.004) {
            const p = i * 4;
            d[p] = 232;
            d[p + 1] = 255;
            d[p + 2] = 248;
            d[p + 3] = Math.min(215, s * 200);
          } else if (s < -0.004) {
            const p = i * 4;
            d[p] = 2;
            d[p + 1] = 22;
            d[p + 2] = 42;
            d[p + 3] = Math.min(130, -s * 95);
          }
        }
      }
      ctx.putImageData(field, 0, 0);
    };

    // --- the explorer --------------------------------------------------------
    // It enters from off-stage left and keeps sailing towards wherever it was
    // last sent. Motion is a slow lerp, never a transition: a boat arrives, it
    // does not snap.
    const restX = () => heroW * 0.27;
    const restY = () => heroH * 0.76;
    let bw = 240;
    let bh = 160;
    const sizeBoat = () => {
      const r = boat.getBoundingClientRect();
      if (r.width) {
        bw = r.width;
        bh = r.height;
      }
    };
    sizeBoat();

    let cx = -bw;
    let cy = restY();
    let tx = restX();
    let ty = restY();
    let vx = 0;
    let lean = 0;

    const clampTarget = () => {
      tx = Math.min(Math.max(tx, heroW * 0.07), heroW * 0.93);
      ty = Math.min(Math.max(ty, heroH * 0.56), heroH * 0.9);
    };

    // --- pointer ------------------------------------------------------------
    let lastX = -1;
    let lastY = -1;
    let pendingX = -1;
    let pendingY = -1;

    hero.addEventListener(
      "pointermove",
      (e) => {
        const r = hero.getBoundingClientRect();
        pendingX = e.clientX - r.left;
        pendingY = e.clientY - r.top;
      },
      { passive: true },
    );
    hero.addEventListener("pointerleave", () => {
      lastX = -1;
      lastY = -1;
      pendingX = -1;
    });

    hero.addEventListener("pointerdown", (e) => {
      const r = hero.getBoundingClientRect();
      const px = e.clientX - r.left;
      const py = e.clientY - r.top;
      disturb(px, py, 9, 5);
      tx = px;
      ty = py;
      clampTarget();
    });

    /** Ripple along the pointer's path, not just at its last sample. */
    const tracePointer = () => {
      if (pendingX < 0) return;
      const px = pendingX;
      const py = pendingY;
      if (lastX >= 0) {
        const dx = px - lastX;
        const dy = py - lastY;
        const dist = Math.sqrt(dx * dx + dy * dy);
        const steps = Math.min(6, Math.max(1, Math.round(dist / (CELL * 2))));
        for (let s = 1; s <= steps; s++) {
          disturb(lastX + (dx * s) / steps, lastY + (dy * s) / steps, 1.05, 3);
        }
      } else {
        disturb(px, py, 1.05, 3);
      }
      lastX = px;
      lastY = py;
    };

    // --- loop ---------------------------------------------------------------
    let running = true;
    let started = 0;
    let raf = 0;

    const frame = (now: number) => {
      if (!started) started = now;
      const t = now - started;

      tracePointer();

      // The hull cuts the water it moves through.
      const dx = tx - cx;
      const dy = ty - cy;
      vx = dx * 0.014;
      cx += vx;
      cy += dy * 0.014;
      const speed = Math.abs(vx);
      if (speed > 0.12) disturb(cx, cy + bh * 0.06, Math.min(1.6, speed * 0.5), 3);

      // Lean into the direction of travel, and ride the swell at rest.
      lean += (vx * 0.42 - lean) * 0.06;
      const bob = Math.sin(t * 0.0011) * 5;
      const roll = Math.sin(t * 0.0009) * 0.9;
      boat.style.transform =
        `translate3d(${(cx - bw / 2).toFixed(1)}px, ${(cy - bh / 2 + bob).toFixed(1)}px, 0)` +
        ` rotate(${(lean + roll).toFixed(2)}deg)`;

      // A little weather, so the surface is alive before anyone touches it.
      if (Math.random() < 0.03) {
        disturb(Math.random() * heroW, heroH * (0.42 + Math.random() * 0.54), 1.5, 4);
      }

      step();
      paint();

      if (running) raf = requestAnimationFrame(frame);
    };

    // Reveal, then set sail.
    requestAnimationFrame((now) => {
      boat.classList.add("sailing");
      canvas.classList.add("ready");
      started = now;
      raf = requestAnimationFrame(frame);
    });

    // --- only run while it is actually on screen -----------------------------
    const vis = new IntersectionObserver(
      (entries) => {
        entries.forEach((e) => {
          if (e.isIntersecting && !running) {
            running = true;
            raf = requestAnimationFrame(frame);
          } else if (!e.isIntersecting && running) {
            running = false;
            cancelAnimationFrame(raf);
          }
        });
      },
      { threshold: 0 },
    );
    vis.observe(hero);

    document.addEventListener("visibilitychange", () => {
      if (document.hidden) {
        running = false;
        cancelAnimationFrame(raf);
      } else if (!running) {
        running = true;
        raf = requestAnimationFrame(frame);
      }
    });

    // --- resize --------------------------------------------------------------
    let resizeTimer = 0;
    const onResize = () => {
      window.clearTimeout(resizeTimer);
      resizeTimer = window.setTimeout(() => {
        measure();
        sizeBoat();
        tx = Math.min(tx, heroW * 0.93);
        ty = Math.min(ty, heroH * 0.9);
        clampTarget();
      }, 160);
    };
    window.addEventListener("resize", onResize, { passive: true });
  } else if (boat) {
    // Reduced motion: the explorer is present, and simply stays where it is.
    boat.classList.add("sailing");
  }
}

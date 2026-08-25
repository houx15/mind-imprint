/**
 * Typed text.
 *
 * The page is about a machine reading and writing, so the lines that carry the
 * argument arrive the way a machine would put them down — a character at a
 * time, behind a caret.
 *
 * Progressive enhancement, strictly: the full text is in the HTML and is what
 * search engines and readers without JS get. Nothing is typed if the reader has
 * asked for reduced motion.
 *
 *   <h2 data-type>…</h2>              types once, when it first comes into view
 *   <h3 data-type data-type-hold>…</h3>  waits for a `type:play` event instead
 *
 * Dispatch `type:play` on an element to (re)type it — the third screen does
 * this each time a product panel becomes the active one.
 */
const reduced = window.matchMedia("(prefers-reduced-motion: reduce)").matches;

/** CJK sets its own pace: fewer characters, each carrying more. */
const CJK = /[㐀-鿿豈-﫿぀-ヿ]/;

function play(el: HTMLElement): void {
  const full = el.dataset.typeText ?? "";
  if (!full) return;

  const prev = Number(el.dataset.typeTimer || 0);
  if (prev) window.clearInterval(prev);

  const base = CJK.test(full) ? 48 : 26;
  // Long lines would otherwise outstay their welcome.
  const step = Math.max(12, Math.min(base, 1500 / full.length));

  const chars = Array.from(full);
  let i = 0;
  el.textContent = "";
  el.classList.add("typing");

  const timer = window.setInterval(() => {
    // Punctuation gets a beat, which is most of what makes it read as typing
    // rather than as a progress bar.
    el.textContent = chars.slice(0, ++i).join("");
    if (i < chars.length) return;
    window.clearInterval(timer);
    el.dataset.typeTimer = "0";
    el.classList.remove("typing");
    el.classList.add("typed");
  }, step);

  el.dataset.typeTimer = String(timer);
}

function init(): void {
  const targets = Array.from(document.querySelectorAll<HTMLElement>("[data-type]"));
  if (!targets.length) return;

  for (const el of targets) {
    el.dataset.typeText = (el.textContent ?? "").trim();
    // Hold the line's height from the start so nothing below it jumps.
    el.style.minHeight = `${el.getBoundingClientRect().height}px`;
    el.addEventListener("type:play", () => {
      if (!reduced) play(el);
    });
  }

  if (reduced) return;

  for (const el of targets) {
    if (el.dataset.typeHold === undefined) el.textContent = "";
  }

  const io = new IntersectionObserver(
    (entries) => {
      for (const e of entries) {
        if (!e.isIntersecting) continue;
        const el = e.target as HTMLElement;
        io.unobserve(el);
        if (el.dataset.typeHold === undefined) play(el);
      }
    },
    { threshold: 0.5, rootMargin: "0px 0px -8% 0px" },
  );
  targets.forEach((el) => {
    if (el.dataset.typeHold === undefined) io.observe(el);
  });
}

if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", init, { once: true });
} else {
  init();
}

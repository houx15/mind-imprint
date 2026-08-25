/**
 * Clips that behave like GIFs.
 *
 * Muted, no controls, no loop: a clip starts from the beginning when it comes
 * into view, and stops when it leaves. It is an illustration that happens to
 * move, not a video player parked on the page.
 *
 *   <video data-clip …>
 *
 * Nothing is downloaded until the clip is near the viewport, and a browser that
 * refuses muted autoplay simply leaves the poster up, which is a perfectly good
 * outcome.
 */
const reduced = window.matchMedia("(prefers-reduced-motion: reduce)").matches;

function init(): void {
  const clips = Array.from(document.querySelectorAll<HTMLVideoElement>("video[data-clip]"));
  if (!clips.length || reduced) return;

  const io = new IntersectionObserver(
    (entries) => {
      for (const e of entries) {
        const v = e.target as HTMLVideoElement;
        if (e.isIntersecting) {
          if (v.preload !== "auto") v.preload = "auto";
          v.currentTime = 0;
          void v.play().catch(() => {});
        } else {
          v.pause();
        }
      }
    },
    { threshold: 0.35, rootMargin: "0px 0px -8% 0px" },
  );
  clips.forEach((v) => io.observe(v));
}

if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", init, { once: true });
} else {
  init();
}

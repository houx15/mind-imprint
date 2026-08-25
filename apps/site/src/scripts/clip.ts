/**
 * Clips that behave like GIFs.
 *
 * Muted, no controls: a clip starts from the beginning when it comes into
 * view, repeats while it is on screen, and stops when it leaves. It is an
 * illustration that happens to move, not a video player parked on the page.
 *
 * It repeats because there is no way to ask it to. Playing once means ending
 * frozen on whatever frame came last, and anyone who arrives a moment late
 * sees a still they cannot replay.
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
          // Set here rather than on each <video>: this is the one place that
          // knows about every clip on the site.
          v.loop = true;
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

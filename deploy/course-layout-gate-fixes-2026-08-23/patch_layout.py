#!/usr/bin/env python3
"""Patch course interaction HTML that re-imposes its own frame on the slot.

The host already hands an `interactiveHtml` block the WHOLE slot and lets the
sandboxed document scroll (see packages/course-renderer/src/styles/course.css —
`aspectRatio` is an authoring hint, not a clamp). These interactions throw that
away and re-impose a fixed design box, in one of two ways:

  A · SCALE-CROP (course-19)
      A 1024x768 `#stage` is scaled with `transform: scale(s)` from a
      `transform-origin: top left`, while `body` flex-CENTRES it. Flex centres
      the PRE-transform 1024x768 box, then the transform shrinks it away from
      that centre — so for any s < 1 the content is dragged up and to the left,
      off the frame. `overflow-x: hidden` plus a body sized to the scaled
      footprint means there is nothing to scroll back to: those pixels are
      simply gone. Measured on the live slice-17 frame: -12,-9 px at a 1440x900
      window, -132,-99 at 1280x720, and at ~400px of frame height the whole
      interaction is off-screen.
      FIX: centre the POST-transform footprint. `html` becomes the centring
      flex container, `body` is the scaled-footprint box (its size is already
      set by the interaction's own fit()), and `#stage` is pinned at its
      origin so scaling from top-left starts from 0,0.

  B · RATIO-BOX (everything else here)
      A `#stage` / `.canvas` wrapper pins `aspect-ratio` and clips with
      `overflow: hidden`. In a slot whose shape differs from that ratio the
      interaction letterboxes itself — measured 386px of an available 750px in
      a split-horizontal right slot — and any content past the ratio box is
      clipped with no way to scroll to it. In course-08 the ratio box comes out
      TALLER than the frame, so body's flex centring pushes its top off-screen
      and `overflow: hidden` makes that unreachable too.
      FIX: let the wrapper fill the slot it was given, drop the ratio clamp,
      and scroll instead of clip.

Both patches are APPEND-ONLY: a single marked <style> block is added just
before </body>, so it wins on source order and `--revert` removes it exactly.
Nothing in the authored markup or logic is touched.

    python3 patch_layout.py            # write patched/ from ../../.tmp-dl
    python3 patch_layout.py --revert   # strip the block from patched/ again
"""
import os
import re
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
SRC = os.path.join(HERE, "..", "..", ".tmp-dl")
OUT = os.path.join(HERE, "patched")

BEGIN = "<!-- mind-imprint:layout-fix-2026-08-23 -->"
END = "<!-- /mind-imprint:layout-fix-2026-08-23 -->"

# A · the scaled-stage family: centre the scaled footprint, not the layout box.
FIX_SCALE = """
html { height: 100%; display: flex; align-items: center; justify-content: center; overflow: hidden; }
body { display: block !important; position: relative; flex: 0 0 auto; min-height: 0 !important; overflow: hidden; }
#stage { position: absolute !important; top: 0 !important; left: 0 !important; }
"""

# B · the ratio-box family: fill the slot, never clip.
FIX_FILL = """
html, body { height: 100%; min-height: 0; }
body { display: block !important; place-items: stretch !important; overflow: auto; }
#stage, .canvas {
  width: 100% !important;
  height: 100% !important;
  min-height: 0 !important;
  max-height: none !important;
  aspect-ratio: auto !important;
  margin: 0 !important;
  overflow: auto !important;
}
"""

# Which file gets which patch. Derived from a measured sweep of every published
# interaction (unreachable content at the frame sizes the host actually gives).
SCALE_FILES = [
    "course-19__gone-quadrant-desk.html",
    "course-19__highlighter-stop.html",
    "course-19__line-e-desk.html",
    "course-19__line-g-desk.html",
    "course-19__line-s-desk.html",
]

FILL_FILES = [
    # ratio box taller than the frame -> top clipped, unreachable (any slot)
    "course-08__information-agency-question.html",
    "course-08__reporter-challenge-question.html",
    "course-08__sandwich-model-question.html",
    "course-08__source-type-method-question.html",
    "course-08__venice-claim-question.html",
    # letterboxed inside a narrow split/grid slot
    "course-03__source-pyramid.html",
    "course-06__scope-lab.html",
    "course-06__restatement-lab.html",
    "course-06__feedback-revision.html",
    "course-11__opcvl-guided-lab.html",
    "course-24__concession-paragraph-writer.html",
    "follow-the-money-fossil-fuel__step-1-doubt.html",
    "follow-the-money-fossil-fuel__step-2-source.html",
    "follow-the-money-fossil-fuel__step-3-owner.html",
    "follow-the-money-fossil-fuel__step-4-org.html",
    "follow-the-money-fossil-fuel__step-5-thinktank.html",
    "follow-the-money-fossil-fuel__step-6-money.html",
    "follow-the-money-fossil-fuel__step-7-coi.html",
    "follow-the-money-fossil-fuel__step-8-name.html",
]

BLOCK_RE = re.compile(re.escape(BEGIN) + r".*?" + re.escape(END) + r"\n?", re.S)


def strip(html: str) -> str:
    return BLOCK_RE.sub("", html)


def apply(html: str, css: str) -> str:
    html = strip(html)
    block = f"{BEGIN}\n<style>{css}</style>\n{END}\n"
    if "</body>" in html:
        return html.replace("</body>", block + "</body>", 1)
    return html + block


def main() -> int:
    revert = "--revert" in sys.argv
    os.makedirs(OUT, exist_ok=True)
    n = 0
    for names, css in ((SCALE_FILES, FIX_SCALE), (FILL_FILES, FIX_FILL)):
        for name in names:
            src = os.path.join(OUT if revert else SRC, name)
            if not os.path.exists(src):
                print(f"  MISSING {src}", file=sys.stderr)
                continue
            html = open(src, encoding="utf-8", errors="replace").read()
            out = strip(html) if revert else apply(html, css)
            open(os.path.join(OUT, name), "w", encoding="utf-8").write(out)
            n += 1
    print(f"{'reverted' if revert else 'patched'} {n} files -> {OUT}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

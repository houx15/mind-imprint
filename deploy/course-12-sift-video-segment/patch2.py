#!/usr/bin/env python3
"""FOLLOW-UP to patch.py: keep the control row on ONE line.

patch.py added a 4th chip and a long end-of-segment hint. `.vctrl` is
`flex-wrap: wrap` inside a fixed-height 750px stage, so the row wrapped to two
lines (81px instead of 37px) and pushed the PRIMARY action —
「看完了，动手做 →」 — down to y=677 while the footer starts at y=691.
Measured on prod: `document.elementFromPoint()` at that button's centre
returned NONE, i.e. it was not clickable at all.

That button is the only way from watch mode into the questions, so the slice
went from "looks frozen" to an actual dead end. Strictly worse than the bug
being fixed.

Fix — take the width pressure off and make the row physically unable to wrap:

  1. `.vctrl` -> `flex-wrap: nowrap`, every chip `flex: 0 0 auto` so the
     primary action can never be shrunk or displaced.
  2. `.vhint` truncates with an ellipsis instead of growing a second line, and
     carries its full text in `title=` so nothing is lost.
  3. The end-of-segment hint gets short: 「本段到这里结束——点「重播」再看一遍。」
  4. The 播放/暂停 chip keeps a stable 2-character label (it no longer swells
     to 重看本段). Clicking it on a finished segment still replays — that
     behaviour is unchanged, and the 重播 chip already names the action.

Applies ON TOP of a patch.py-patched file (i.e. what is live now).

    python3 patch2.py
"""
import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
SRC = os.path.join(HERE, "original", "sift-check.html")
OUT_DIR = os.path.join(HERE, "patched")
OUT = os.path.join(OUT_DIR, "course-12__sift-check.html")

# (label, needle, replacement, expected_count)
EDITS = [
    (
        "css: control row can never wrap; hint truncates instead",
        ".vctrl{display:flex;align-items:center;gap:8px;flex:0 0 auto;flex-wrap:wrap}\n"
        ".vhint{color:#7b8593;flex:1 1 auto;min-width:0;line-height:1.35}",
        ".vctrl{display:flex;align-items:center;gap:8px;flex:0 0 auto;flex-wrap:nowrap}\n"
        ".vctrl .chip{flex:0 0 auto}\n"
        ".vhint{color:#7b8593;flex:1 1 auto;min-width:0;line-height:1.35;"
        "overflow:hidden;text-overflow:ellipsis;white-space:nowrap}",
        1,
    ),
    (
        "js: setHint helper keeps the full text in title=",
        "function syncPlayChip(){",
        "function setHint(t){\n"
        "  var h = el('vhint');\n"
        "  h.textContent = t;\n"
        "  h.title = t;  /* the row never wraps, so a long hint ellipsises — keep it recoverable */\n"
        "}\n"
        "function syncPlayChip(){",
        1,
    ),
    (
        "js: route both hint writes through setHint",
        "el('vhint').textContent = seg.hint;",
        "setHint(seg.hint);",
        2,
    ),
    (
        "js: shorter end-of-segment hint",
        "el('vhint').textContent = '本段到这里结束——这一步只截取了录屏的其中一节。想再看一遍，点「重看本段」。';",
        "setHint('本段到这里结束——点「重播」再看一遍。');",
        1,
    ),
    (
        "js: play chip keeps a stable 2-character label",
        "  b.textContent = segEnded ? '重看本段' : (vid.paused ? '播放' : '暂停');",
        "  b.textContent = vid.paused ? '播放' : '暂停';",
        1,
    ),
    (
        "js: zoom overlay label likewise stable",
        "  el('vzPlay').textContent = segEnded ? '重看本段' : (vid.paused ? '播放' : '暂停');",
        "  el('vzPlay').textContent = vid.paused ? '播放' : '暂停';",
        1,
    ),
]


def main() -> int:
    if not os.path.exists(SRC):
        sys.exit(f"missing source: {SRC} — run fetch_original.py")
    html = open(SRC, encoding="utf-8").read()
    if "本段到这里结束" not in html:
        sys.exit("source is NOT patch.py-patched — patch2 applies on top of it")
    for label, needle, repl, want in EDITS:
        n = html.count(needle)
        if n != want:
            sys.exit(f"REFUSING: '{label}' matched {n} times (expected {want})")
        html = html.replace(needle, repl)
        print(f"  ok  {label}")
    os.makedirs(OUT_DIR, exist_ok=True)
    open(OUT, "w", encoding="utf-8").write(html)
    print(f"\nwrote {OUT} ({os.path.getsize(OUT)}B)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

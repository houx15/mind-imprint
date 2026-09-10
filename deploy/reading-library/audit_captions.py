#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Flag one photograph whose caption differs between the levels of its story.

WHY THIS IS THE MOST USEFUL OF THE THREE AUDITS

The levels of a story are rewrites of the same reporting against the same set
of photographs, so a picture's caption should say the same thing in all of
them. Where it does not, one of the versions is usually damaged — and damaged
in a way nothing else catches, because the wreckage is still a grammatical
English sentence:

    [740L]                    Foundation/CC BY 4.0
    [930L,1090L,1200L,MAX]    In 2026, Hong Wang became only the third woman …

Reading any single level tells you nothing. Reading them side by side tells you
immediately. Six corrupt captions in the 2026-09-10 batch were found this way,
and a "climates corps" typo that had been live since the first batch.

NOT EVERY DIFFERENCE IS A DEFECT, AND THE ORDER IS NOT A VERDICT. Newsela
genuinely rewrites some captions per reading level, and those rewrites are
correct. So this audit never fails a build — it sorts, and a person reads the
whole list. Sorting alone is not enough to separate the two: the ugliest
damage in the 2026-09-10 batch (a neighbouring column spliced in mid-sentence)
scores LOWER than a legitimate rewrite, because the splice adds text while a
rewrite only reworks it. Two markers therefore run ahead of the score:

  no caption   one level has none at all — always damage
  spliced      "Image 3." appears mid-caption, which only happens when the
               PDF's two columns were interleaved — always damage
  近似         the variants are nearly identical, so the difference is a typo
               or a dropped word rather than a rewrite

Everything else is printed too, least-similar last. Read to the bottom.

Reads parse.py's output on stdin:  python3 parse.py | python3 audit_captions.py
"""
from __future__ import annotations

import difflib
import json
import re
import sys

# Above this similarity, two captions for the same picture are the same
# sentence with something wrong in one of them, not a rewrite. It only labels
# the report — nothing is dropped, and it is not the only marker.
SUSPICIOUS = 0.80
# "Image 3." belongs at the START of a caption; the export puts it there. Found
# anywhere else, the caption has another caption inside it.
SPLICED = re.compile(r".\s+Image\s+\d+[.:]\s")


def closeness(variants: list[str]) -> float:
    """Similarity of the two most alike variants, ignoring a missing one."""
    real = [v for v in variants if v]
    if len(real) < 2:
        return 1.0
    return max(
        difflib.SequenceMatcher(None, a, b).ratio()
        for i, a in enumerate(real)
        for b in real[i + 1:]
    )


def verdict(variants: list[str]) -> tuple[int, str]:
    """(sort rank, label). Lower rank prints first."""
    if any(not v for v in variants):
        return 0, "NO CAPTION"
    if any(SPLICED.search(v) for v in variants):
        return 0, "SPLICED"
    if closeness(variants) >= SUSPICIOUS:
        return 1, "NEARLY IDENTICAL"
    return 2, "read it"


def main() -> int:
    articles = json.load(sys.stdin)

    rows = []
    for art in articles:
        # file -> caption -> the levels that carry it
        seen: dict[str, dict[str, list[str]]] = {}
        for lvl in art["levels"]:
            for fig in lvl["figures"]:
                seen.setdefault(fig["file"], {}).setdefault(fig["caption"], []).append(lvl["level"])
        # A picture that only some levels use is normal (the levels differ in
        # length); what matters is disagreement among the levels that DO use it.
        for file, variants in seen.items():
            if len(variants) < 2:
                continue
            rank, label = verdict(list(variants))
            rows.append((rank, -closeness(list(variants)), label, art["slug"], file, variants))

    rows.sort(key=lambda r: (r[0], r[1]))
    for rank, negscore, label, slug, file, variants in rows:
        print("\n%-17s %.2f  %s\n%24s%s" % (label, -negscore, slug, "", file))
        for caption, levels in sorted(variants.items(), key=lambda kv: -len(kv[1])):
            print("    [%s] %s" % (",".join(levels), caption or "<<< NO CAPTION >>>"))

    certain = sum(1 for r in rows if r[0] == 0)
    near = sum(1 for r in rows if r[0] == 1)
    print("\n%d photographs captioned two ways · %d certainly damaged · %d nearly identical"
          % (len(rows), certain, near))
    print("The rest are printed above too. A rewrite and a defect are not "
          "separable by score — read them.")
    # Advisory, like the other two audits: a person decides which are rewrites.
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

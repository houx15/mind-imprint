#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Flag imperial→metric conversions in the corpus that do not agree.

The PDF export dropped digits in at least one caption ("155-foot-(477-meter)-
diameter circle"), and a student reading for evidence has no way to tell a
typo from a fact. This walks every body and caption, re-does each conversion
it can find, and prints the pairs that are more than ~15% apart.

It reads parse.py's output on stdin:  python3 parse.py | python3 audit_units.py
"""
import json
import re
import sys

# imperial unit -> its size in the metric base unit (metres, or grams)
IMPERIAL = {
    "foot": 0.3048, "feet": 0.3048, "inch": 0.0254, "inches": 0.0254,
    "yard": 0.9144, "yards": 0.9144, "mile": 1609.34, "miles": 1609.34,
    "ounce": 28.35, "ounces": 28.35, "pound": 453.6, "pounds": 453.6,
}
METRIC = {
    "meter": 1, "meters": 1, "metre": 1, "metres": 1,
    "centimeter": 0.01, "centimeters": 0.01,
    "kilometer": 1000, "kilometers": 1000,
    "gram": 1, "grams": 1, "kilogram": 1000, "kilograms": 1000,
}

PAIR = re.compile(
    r"([\d,.]+)[\s-]*(" + "|".join(IMPERIAL) + r")[\s-]*[-(]?\s*\(?\s*"
    r"([\d,.]+)[\s-]*(" + "|".join(METRIC) + r")",
    re.I,
)


def main() -> int:
    articles = json.load(sys.stdin)
    flagged = []
    checked = 0
    for a in articles:
        texts = [(lvl["level"], lvl["body"]) for lvl in a["levels"]]
        texts += [(lvl["level"], f["caption"]) for lvl in a["levels"] for f in lvl["figures"]]
        for level, text in texts:
            for m in PAIR.finditer(text):
                checked += 1
                imperial = float(m.group(1).replace(",", "")) * IMPERIAL[m.group(2).lower()]
                metric = float(m.group(3).replace(",", "")) * METRIC[m.group(4).lower()]
                if not imperial:
                    continue
                ratio = metric / imperial
                if 0.85 < ratio < 1.18:
                    continue
                flagged.append((a["slug"], level, round(ratio, 2), m.group(0)))

    for slug, level, ratio, phrase in sorted(set(flagged)):
        print("%-46s %-6s off by %sx  %s" % (slug, level, ratio, phrase))
    print("conversions checked: %d · suspicious: %d" % (checked, len(set(flagged))))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

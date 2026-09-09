#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Build the reading library's content file.

    parse.py       the 100 markdown files -> 20 stories x 5 levels
    tags.json      中文标题 / 一句话理由 / 学科（闭表内）
    corrections.json  逐字改掉的错处
    dist/images.json  make_images.py 写的图片清单（object key + 尺寸）
        |
        v
    apps/api/internal/library/articles.json     <- go:embed 读它

WHY A FILE AND NOT A TABLE. This is content, not user data: one copy for the
whole school, changed by editing and review rather than by a migration. It is
the same call `internal/disciplines` made, for the same reason, and it means
the catalogue can be diffed in a pull request. Only the EDGE — which article a
student read, at which level — goes to Postgres.

LEVEL NAMES. The export labels difficulty in Lexile ("430L") plus "MAX" for the
unsimplified original. A student has no way to read that scale, so each story's
five files are ranked and named 入门 / 基础 / 进阶 / 高阶 / 原文. The Lexile
number survives as a second line on the card for anyone who does know the
scale — renaming it away would hide a real measurement.

Run:  python3 deploy/reading-library/build.py
"""
from __future__ import annotations

import io
import json
import math
import os
import subprocess
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.abspath(os.path.join(HERE, "..", ".."))
DIST = os.path.join(HERE, "dist")
OUT = os.path.join(ROOT, "apps", "api", "internal", "library", "articles.json")
DISCIPLINES = os.path.join(ROOT, "packages", "contracts", "disciplines", "disciplines.json")

TIER_NAMES = ["入门", "基础", "进阶", "高阶", "原文"]
# Careful second-language reading, which is what the room is for. Shown as an
# estimate next to the word count, never as a countdown.
WORDS_PER_MINUTE = 150


def normalize_quotes(s: str) -> str:
    """One apostrophe and one double quote across the whole library.

    The export mixes both forms — 1421 ASCII apostrophes against 60 curly, 1785
    ASCII double quotes against 35 — because the two came out of different PDF
    fonts. Folding the minority into the majority is the safe direction: going
    the other way needs a guess about whether each quote opens or closes, and a
    wrong guess is visible in the text. Prime marks (″) are left alone; they are
    measurements, not punctuation.
    """
    return s.replace("’", "'").replace("“", '"').replace("”", '"')


def apply_corrections(text: str, replacements: list[dict], hits: dict[str, int]) -> str:
    for r in replacements:
        n = text.count(r["find"])
        if n:
            hits[r["find"]] = hits.get(r["find"], 0) + n
            text = text.replace(r["find"], r["replace"])
    return text


def main() -> int:
    with open(os.path.join(HERE, "tags.json"), encoding="utf-8") as fh:
        tags = json.load(fh)["articles"]
    with open(os.path.join(HERE, "corrections.json"), encoding="utf-8") as fh:
        replacements = json.load(fh)["replacements"]
    images_path = os.path.join(DIST, "images.json")
    if not os.path.exists(images_path):
        print("no image manifest at %s — run make_images.py first" % images_path, file=sys.stderr)
        return 1
    with open(images_path, encoding="utf-8") as fh:
        images = json.load(fh)
    with open(DISCIPLINES, encoding="utf-8") as fh:
        known = {d["id"]: d for d in json.load(fh)}

    parsed = json.loads(
        subprocess.run(
            [sys.executable, os.path.join(HERE, "parse.py")],
            check=True, capture_output=True, text=True,
        ).stdout
    )

    hits: dict[str, int] = {}
    out = []
    problems = []

    for art in parsed:
        slug = art["slug"]
        tag = tags.get(slug)
        if tag is None:
            problems.append("%s: no entry in tags.json" % slug)
            continue
        for d in tag["disciplines"]:
            if d not in known:
                problems.append("%s: %r is not in the discipline table" % (slug, d))
        if not tag["disciplines"]:
            problems.append("%s: needs at least one discipline" % slug)

        levels = []
        for tier, lvl in enumerate(art["levels"], start=1):
            body = apply_corrections(normalize_quotes(lvl["body"]), replacements, hits)
            figures = []
            for f in lvl["figures"]:
                meta = images.get(f["file"])
                if meta is None:
                    problems.append("%s %s: no processed image for %s" % (slug, lvl["level"], f["file"]))
                    continue
                # No separate `alt`: the caption already describes the
                # picture in full, and two fields holding the same sentence
                # would drift the moment one of them is corrected. The
                # renderer sets alt from caption.
                caption = f["caption"] or f["alt"]
                figures.append(
                    {
                        "after": f["after"],
                        "key": meta["key"],
                        "width": meta["width"],
                        "height": meta["height"],
                        "caption": apply_corrections(normalize_quotes(caption), replacements, hits),
                        "credit": normalize_quotes(f["credit"]),
                    }
                )
            levels.append(
                {
                    "tier": tier,
                    "name": TIER_NAMES[tier - 1],
                    # 0 means "the original, never simplified" — the scale does
                    # not apply, so the card shows 原文 alone rather than a
                    # number invented to fill the slot.
                    "lexile": 0 if lvl["level"] == "MAX" else int(lvl["level"].rstrip("L")),
                    "words": lvl["words"],
                    "minutes": max(1, math.ceil(lvl["words"] / WORDS_PER_MINUTE)),
                    "title": normalize_quotes(lvl["title"]),
                    "body": body,
                    "headings": lvl["headings"],
                    "figures": figures,
                }
            )

        if len(levels) != len(TIER_NAMES):
            problems.append("%s: %d levels, expected %d" % (slug, len(levels), len(TIER_NAMES)))

        cover = next((f for f in levels[-1]["figures"] if f["after"] == ""), None)
        if cover is None:
            problems.append("%s: no lead photograph to use as the shelf cover" % slug)

        out.append(
            {
                "slug": slug,
                "title": normalize_quotes(art["title"]),
                "zhTitle": tag["zh_title"],
                "reason": tag["reason"],
                "lang": "en",
                "disciplines": tag["disciplines"],
                # The main branch of the FIRST discipline. Drives the card's
                # colour on the shelf and the branch filter in the full list.
                "field": known[tag["disciplines"][0]]["field"] if tag["disciplines"] else "",
                "cover": {k: cover[k] for k in ("key", "width", "height", "caption")} if cover else None,
                "levels": levels,
            }
        )

    for r in replacements:
        n = hits.get(r["find"], 0)
        if n != r["count"]:
            problems.append("correction %r matched %d times, expected %d" % (r["find"][:40], n, r["count"]))

    if problems:
        for p in problems:
            print("BUILD FAILED: %s" % p, file=sys.stderr)
        return 1

    buf = io.StringIO()
    json.dump(out, buf, ensure_ascii=False, indent=2)
    buf.write("\n")
    os.makedirs(os.path.dirname(OUT), exist_ok=True)
    with open(OUT, "w", encoding="utf-8") as fh:
        fh.write(buf.getvalue())

    figs = sum(len(l["figures"]) for a in out for l in a["levels"])
    print("wrote %s" % os.path.relpath(OUT, ROOT))
    print("%d articles · %d levels · %d figures · %.1f KB"
          % (len(out), sum(len(a["levels"]) for a in out), figs, len(buf.getvalue().encode()) / 1024))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

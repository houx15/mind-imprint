#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Turn the Newsela markdown export into one structured article per story.

Input  docs/reference/reading-database/*.md      (gitignored reference copy)
Output apps/api/internal/library/articles.json   (the library's content source)

Each story arrives as five files — four Lexile rewrites plus the original
("MAX"). They share a title, a set of photographs and a topic, and differ only
in the prose, so the output is ONE article carrying five levels rather than a
hundred unrelated entries.

WHAT THIS FILE DOES BEYOND PARSING

  * Body and pictures are separated. The reading room splits a body on blank
    lines and hangs tool cards on the resulting paragraph ids, so a picture
    left inline would occupy a paragraph id and could be quoted back at the
    student as if it were prose. Pictures therefore come out as `figures`,
    each anchored `after` the id of the paragraph it followed.
  * Captions are split into caption and credit. The export writes them as one
    italic line ending in "Photo: Someone/AP", and prefixes all but the lead
    with "Image 3." — neither belongs in a caption read by a student.
  * Section headings keep their text but lose the "##", and their block ids
    are listed in `headings` so the room can set them as headings instead of
    printing the marker.
  * Block ids are computed exactly the way Go's SplitBlocks computes them
    (split on "\n\n", trim, drop empties, number the survivors b1, b2, …).
    If the two ever disagree, every figure in the library moves to the wrong
    paragraph, so `verify.py` re-derives them from the emitted body.

Level names come from `tags.json`; see `build.py` for how the two are joined.

Run:  python3 deploy/reading-library/parse.py
"""
from __future__ import annotations

import json
import os
import re
import sys
from collections import defaultdict

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.abspath(os.path.join(HERE, "..", ".."))
SRC = os.path.join(ROOT, "docs", "reference", "reading-database")

# "**Level:** 430L | **Word Count:** 471"
META = re.compile(r"^\*\*Level:\*\*\s*(\S+)\s*\|\s*\*\*Word Count:\*\*\s*([\d,]+)\s*$")
IMAGE = re.compile(r"^!\[(?P<alt>.*?)\]\((?P<src>[^)]+)\)\s*$")
CAPTION = re.compile(r"^\*(?P<text>.+)\*$")
HEADING = re.compile(r"^##\s+(?P<text>.+?)\s*$")
# "Image 3. " / "Image 3: " — the export numbers every picture but the lead.
IMAGE_NO = re.compile(r"^Image\s+\d+[.:]\s*")
# The credit trails the caption, and its marker word is not always "Photo:" —
# maps, diagrams and the pain-index chart are credited "Map:" and "Graphic:".
# Matching only "Photo:" left a third of the library's pictures uncredited.
CREDIT = re.compile(r"\b(?:Photos?|Maps?|Graphics?|Illustrations?|Videos?):")


def dedupe_credit(credit: str) -> str:
    """Collapse a credit the PDF export stamped twice.

    Two of the source captions end in the credit repeated back to back, the
    second copy carrying an OCR slip of its own:

        Photo: Steve Trewhella/Alamy Photo: Steve. Trewhella/Alamy

    Splitting on the marker and comparing the halves with punctuation and case
    removed keeps the first, well-formed copy and drops the rest.
    """
    marks = [m.start() for m in CREDIT.finditer(credit)]
    if len(marks) < 2:
        return credit.strip()
    parts = [credit[a:b].strip() for a, b in zip(marks, marks[1:] + [len(credit)])]
    seen, kept = set(), []
    for p in parts:
        key = re.sub(r"[^a-z0-9]", "", p.lower())
        if key in seen:
            continue
        seen.add(key)
        kept.append(p)
    return " ".join(kept)


def clean_caption(raw: str) -> tuple[str, str]:
    """Split one italic export line into (caption, credit).

    The credit keeps its marker word ("Photo: …", "Graphic: …") because the
    marker is what tells a reader whether they are looking at a photograph or
    at something a newsroom drew.
    """
    text = IMAGE_NO.sub("", raw.strip())
    credit = ""
    m = CREDIT.search(text)
    if m:
        credit = dedupe_credit(text[m.start():])
        text = text[: m.start()].strip()
    return text.strip(), credit


def parse_file(path: str) -> dict:
    """Parse one level file into {title, level, word_count, body, headings, figures}."""
    with open(path, encoding="utf-8") as fh:
        lines = fh.read().replace("\r\n", "\n").split("\n")

    title = ""
    level = ""
    word_count = 0
    # Paragraph-ish units in reading order. Each is (kind, text) where kind is
    # "text" or "heading"; figures are collected separately and remember how
    # many units preceded them.
    units: list[tuple[str, str]] = []
    figures: list[dict] = []
    pending_image: dict | None = None

    def flush_pending(caption: str = "", credit: str = "") -> None:
        nonlocal pending_image
        if pending_image is None:
            return
        pending_image["caption"] = caption
        pending_image["credit"] = credit
        # The export's own alt text is the caption cut at ~80 characters and
        # closed with an ellipsis, which is worse than the caption in every
        # way a screen reader cares about. Use the caption when there is one.
        if caption:
            pending_image["alt"] = caption
        figures.append(pending_image)
        pending_image = None

    for raw in lines:
        line = raw.strip()
        if not line:
            continue
        if not title and line.startswith("# "):
            title = line[2:].strip()
            continue
        m = META.match(line)
        if m:
            level = m.group(1)
            word_count = int(m.group(2).replace(",", ""))
            continue
        m = IMAGE.match(line)
        if m:
            # An image whose caption line never arrived still counts.
            flush_pending()
            pending_image = {
                "file": os.path.basename(m.group("src")),
                # The export's alt text is the caption truncated at ~80 chars
                # with an ellipsis. Kept only as a fallback; the real caption
                # arrives on the next line and replaces it in build.py.
                "alt": m.group("alt").strip(),
                "after": len(units),
            }
            continue
        m = CAPTION.match(line)
        if m and pending_image is not None:
            flush_pending(*clean_caption(m.group("text")))
            continue
        flush_pending()
        m = HEADING.match(line)
        if m:
            units.append(("heading", m.group("text")))
            continue
        units.append(("text", line))
    flush_pending()

    # Body + block ids. Numbering must match Go's SplitBlocks exactly: every
    # surviving unit gets the next id, headings included.
    body = "\n\n".join(text for _, text in units)
    block_ids = ["b%d" % (i + 1) for i in range(len(units))]
    headings = [block_ids[i] for i, (kind, _) in enumerate(units) if kind == "heading"]
    for fig in figures:
        n = fig.pop("after")
        # "" means the picture stands above the first paragraph (the lead photo).
        fig["after"] = block_ids[n - 1] if n > 0 else ""

    return {
        "title": title,
        "level": level,
        # `words` is counted from the body we actually serve. The export's own
        # "Word Count" counts the whole page, captions included, so it runs up
        # to a third high on a picture-heavy story (asian-games 570L declares
        # 546 for 400 words of prose). Reading time is shown to the student,
        # so it has to be the number that matches what she scrolls through.
        "words": len(re.findall(r"[A-Za-z'’\-]+", body)),
        "source_word_count": word_count,
        "body": body,
        "headings": headings,
        "figures": figures,
    }


def lexile(level: str) -> int:
    """Sort key. MAX is the original article and always sorts last."""
    return 10_000 if level == "MAX" else int(level.rstrip("L"))


def main() -> int:
    if not os.path.isdir(SRC):
        print("no source corpus at %s" % SRC, file=sys.stderr)
        return 1

    by_slug: dict[str, list[dict]] = defaultdict(list)
    for name in sorted(os.listdir(SRC)):
        if not name.endswith(".md"):
            continue
        slug = re.sub(r"-(MAX|\d+L)\.md$", "", name)
        if slug == name[:-3]:
            print("unparseable filename: %s" % name, file=sys.stderr)
            return 1
        parsed = parse_file(os.path.join(SRC, name))
        parsed["slug"] = slug
        by_slug[slug].append(parsed)

    out = []
    for slug in sorted(by_slug):
        levels = sorted(by_slug[slug], key=lambda p: lexile(p["level"]))
        titles = {p["title"] for p in levels}
        out.append(
            {
                "slug": slug,
                # The rewrites occasionally differ by a word (「ring of fire」
                # vs 「ring of fire solar」); the original's is the real title.
                "title": levels[-1]["title"],
                "title_variants": sorted(titles) if len(titles) > 1 else [],
                "levels": levels,
            }
        )

    json.dump(out, sys.stdout, ensure_ascii=False, indent=2)
    print()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Where the source corpus lives, and how to walk it.

Every script in this directory reads its inputs through here rather than
hardcoding a path, so adding a batch is one entry in `sources.json` instead of
an edit in four files that can drift apart.

The two batches arrived in different shapes — one flat directory of markdown
next to one shared `images/`, one directory per story with its own `images/`
(and one story with an extra `MD/` level inside that). Nothing downstream cares:
a story's slug and its difficulty both come from the FILENAME, so the scripts
walk each batch recursively and treat depth as noise.
"""
from __future__ import annotations

import json
import os
import re

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.abspath(os.path.join(HERE, "..", ".."))

# "asian-games-830L.md" / "pro-con-data-centers-MAX.md"
LEVEL_SUFFIX = re.compile(r"^(?P<slug>.+)-(?P<level>MAX|\d+L)\.md$")


def batches() -> list[dict]:
    """The registered batches, with `path` resolved to an absolute directory."""
    with open(os.path.join(HERE, "sources.json"), encoding="utf-8") as fh:
        entries = json.load(fh)["batches"]
    for b in entries:
        b["dir"] = os.path.join(ROOT, b["path"])
    return entries


def markdown_files() -> list[tuple[str, str, str, str]]:
    """Every level file across every batch, as (batch, slug, level, path).

    Sorted by slug then filename so a rebuild is byte-identical run to run.
    Anything under an `images/` directory is skipped; a `.md` whose name does
    not end in a level suffix is a hard error rather than a silent skip,
    because that is what a broken export looks like.
    """
    out = []
    for b in batches():
        if not os.path.isdir(b["dir"]):
            raise SystemExit("no source corpus at %s (batch %s)" % (b["dir"], b["name"]))
        for root, dirs, files in os.walk(b["dir"]):
            dirs[:] = [d for d in dirs if d != "images"]
            for name in files:
                if not name.endswith(".md") or name.startswith("."):
                    continue
                m = LEVEL_SUFFIX.match(name)
                if not m:
                    raise SystemExit(
                        "%s: filename carries no difficulty suffix (expected …-930L.md or …-MAX.md)"
                        % os.path.join(root, name)
                    )
                out.append((b["name"], m.group("slug"), m.group("level"), os.path.join(root, name)))
    out.sort(key=lambda t: (t[1], t[3]))
    return out


def image_files() -> list[tuple[str, str]]:
    """Every source photograph across every batch, as (batch, path).

    A picture is anything sitting in a directory named `images`. Sorted by
    basename: the object key is derived from it, and both batches were checked
    to have no basename in common.
    """
    out = []
    for b in batches():
        for root, _, files in os.walk(b["dir"]):
            if os.path.basename(root) != "images":
                continue
            for name in files:
                if name.startswith("."):
                    continue
                out.append((b["name"], os.path.join(root, name)))
    out.sort(key=lambda t: os.path.basename(t[1]))
    return out

#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Prepare the library's photographs for the CDN.

The export ships press originals — the first batch alone was 177 MB across 60
files, the largest a 13 MB PNG at 3754px wide. Serving those to a student on a
phone would spend her data on pixels a 700px column throws away, so every
picture is resized to fit MAX_EDGE and re-encoded as WebP.

Every registered batch is processed in one pass, because the manifest this
writes is the whole library's, not one batch's: build.py looks a figure up by
its source filename with no idea which batch it came from.

Output
  dist/images/<name>.webp   what upload-reading-images.sh PUTs to OSS
  dist/images.json          {source filename: {key, width, height, bytes}}

The manifest is what build.py reads to fill each figure's object key and its
intrinsic size. Width and height travel with the article so the room can
reserve the space before the picture arrives and the text below it does not
jump once it does.

Run:  python3 deploy/reading-library/make_images.py
"""
from __future__ import annotations

import json
import os
import sys

from PIL import Image

import sources

HERE = os.path.dirname(os.path.abspath(__file__))
DIST = os.path.join(HERE, "dist")
OUT = os.path.join(DIST, "images")

# The article column is ~700 CSS px; 1600 covers it at 2x on a retina screen
# with room to spare, and nothing in this corpus benefits from more.
MAX_EDGE = 1600
QUALITY = 82
# Object keys live under the web_resource scope's `web/` prefix (see the scope
# table in apps/api/internal/api/oss.go). `v1` is here so a re-encode can ship
# to fresh keys instead of waiting out the CDN's cache on the old ones.
KEY_PREFIX = "web/reading/v1"


def main() -> int:
    os.makedirs(OUT, exist_ok=True)

    manifest: dict[str, dict] = {}
    seen: dict[str, str] = {}
    total_before = total_after = 0
    for _batch, path in sources.image_files():
        name = os.path.basename(path)
        stem, _ = os.path.splitext(name)
        if stem in seen:
            # Object keys are derived from the basename, so two batches using
            # the same one would overwrite each other on the CDN and put the
            # wrong photograph in one of the two articles.
            print("two sources share the image name %s:\n  %s\n  %s"
                  % (name, seen[stem], path), file=sys.stderr)
            return 1
        seen[stem] = path
        with Image.open(path) as im:
            im = im.convert("RGB")
            w, h = im.size
            scale = min(1.0, MAX_EDGE / max(w, h))
            if scale < 1.0:
                im = im.resize((round(w * scale), round(h * scale)), Image.LANCZOS)
            dest = os.path.join(OUT, stem + ".webp")
            im.save(dest, "WEBP", quality=QUALITY, method=6)
            width, height = im.size
        before, after = os.path.getsize(path), os.path.getsize(dest)
        total_before += before
        total_after += after
        manifest[name] = {
            "key": "%s/%s.webp" % (KEY_PREFIX, stem),
            "width": width,
            "height": height,
            "bytes": after,
        }
        print("%-52s %5dx%-5d %6.1fMB -> %5.0fKB" % (name, width, height, before / 1e6, after / 1e3))

    with open(os.path.join(DIST, "images.json"), "w", encoding="utf-8") as fh:
        json.dump(manifest, fh, ensure_ascii=False, indent=2, sort_keys=True)
        fh.write("\n")

    print("\n%d images · %.1f MB -> %.1f MB" % (len(manifest), total_before / 1e6, total_after / 1e6))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

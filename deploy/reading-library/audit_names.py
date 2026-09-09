#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Flag proper nouns that are spelled two ways inside one story.

The five levels of a story are rewrites of the same reporting, so a person or
a place named in one is almost always named in the others. When two spellings
of the same name differ by a single character, one of them is an OCR slip —
and a name is exactly the kind of error a student will carry into a citation.

Reads parse.py's output on stdin:  python3 parse.py | python3 audit_names.py
"""
import json
import re
import sys
from collections import defaultdict

WORD = re.compile(r"\b[A-Z][A-Za-z’'\-]{2,}\b")

# Sentence-initial words are capitalised by grammar, not by being names, and
# they dominate the noise. These are the ones that actually showed up.
COMMON = {
    "The", "This", "That", "These", "Those", "There", "Their", "They", "Then",
    "But", "And", "For", "From", "With", "When", "Where", "What", "Which",
    "While", "After", "Before", "Because", "Some", "Many", "Most", "More",
    "One", "Two", "Three", "Now", "Not", "But", "His", "Her", "Its", "Was",
    "Were", "Have", "Has", "Had", "About", "Also", "All", "Another", "Any",
    "Are", "Both", "Each", "Every", "Few", "Her", "How", "However", "She",
    "Such", "Than", "Their", "Them", "Very", "You", "Your", "Been", "Being",
}


def close(a: str, b: str) -> bool:
    """True when a and b differ by one insertion, deletion or substitution."""
    if abs(len(a) - len(b)) > 1:
        return False
    if len(a) == len(b):
        return sum(x != y for x, y in zip(a, b)) == 1
    short, long = (a, b) if len(a) < len(b) else (b, a)
    for i in range(len(long)):
        if long[:i] + long[i + 1:] == short:
            return True
    return False


def main() -> int:
    articles = json.load(sys.stdin)
    flagged = 0
    for a in articles:
        counts: dict[str, int] = defaultdict(int)
        for lvl in a["levels"]:
            text = lvl["body"] + "\n" + "\n".join(f["caption"] for f in lvl["figures"])
            for w in WORD.findall(text):
                if w not in COMMON:
                    counts[w] += 1
        names = sorted(counts)
        for i, x in enumerate(names):
            for y in names[i + 1:]:
                if close(x, y):
                    flagged += 1
                    print("%-46s %s (%d) vs %s (%d)" % (a["slug"], x, counts[x], y, counts[y]))
    print("near-duplicate proper nouns: %d" % flagged)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

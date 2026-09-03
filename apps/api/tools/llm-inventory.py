#!/usr/bin/env python3
"""Regenerate appendices A and B of docs/2026-09-03-llm-routing-and-prompt-handbook.md.

    cd apps/api && python3 tools/llm-inventory.py > /tmp/tables.md

Then replace everything from "## 附录 A" to the end of the handbook with the output.

Why a generator instead of a hand-written list: the handbook's value is that a
colleague can trust it, and a 67-row table maintained by hand is wrong within a
week. The description column is each function's OWN doc comment, so the doc stays
true as long as the code comments do — and if a comment is missing, the table
says "—" rather than inventing a description.
"""
import collections
import os
import re
import sys

ROOT = os.path.dirname(os.path.abspath(os.path.join(__file__, "..")))
API = os.path.join(ROOT, "api") if os.path.basename(ROOT) != "api" else ROOT
API = os.path.abspath(os.path.join(os.path.dirname(os.path.abspath(__file__)), ".."))

CLASS_RE = re.compile(r"gateway\.Class([A-Z][a-zA-Z]*)")
FUNC_RE = re.compile(r"^func\s+(?:\([^)]*\)\s*)?([A-Za-z0-9_]+)\s*\(")
PROMPT_RE = re.compile(
    r"^\s*(?:const\s+)?([a-zA-Z][A-Za-z0-9_]*"
    r"(?:System|Prompt|Rubric|Posture|Guide|Instructions))\s*(?:=|\s+string\s*=)\s*`")

# Excluded: tests, the bench fixtures, the gateway's own machinery, and
# proposal_track.go — route/routeE/routeFn are DEFINED there, so its own lines
# are the seam itself rather than call sites.
SKIP = ("_test.go", "benchcases.go", "catalog.go", "keyresolver.go",
        "provider.go", "report.go", "case.go", "judge.go", "proposal_track.go")

CLASS_ZH = {
    "Reflex": "一个标签或一次路由判断，无自由文本",
    "Dialogue": "学生当场看得见的一轮",
    "Compose": "从已陈述的输入派生一个 schema 产物",
    "Review": "判学生的成果，判错有代价",
    "Assess": "过程评估 / 回顾 / 周报，绝不降级",
    "Digest": "长输入短输出，压缩不判断",
}
ORDER = ["Reflex", "Dialogue", "Compose", "Review", "Assess", "Digest"]


def go_files():
    for base, _dirs, files in os.walk(os.path.join(API, "internal")):
        if "/sqlc/" in base:
            continue
        for f in sorted(files):
            if f.endswith(".go") and not any(f.endswith(s) for s in SKIP):
                yield os.path.join(base, f)


def doc_above(lines, i):
    """The doc comment immediately above line i, first sentence only."""
    out = []
    j = i - 1
    while j >= 0 and lines[j].lstrip().startswith("//"):
        out.append(lines[j].lstrip()[2:].strip())
        j -= 1
    out.reverse()
    text = " ".join(d for d in out if d)
    return re.split(r"(?<=[.。])\s", text)[0] if text else ""


def enclosing(lines, idx):
    for i in range(idx, -1, -1):
        m = FUNC_RE.match(lines[i])
        if m:
            return m.group(1), doc_above(lines, i)
    return "", ""


def cell(text, limit):
    text = (text or "—").replace("|", "\\|")
    return text[:limit] + "…" if len(text) > limit else text


sites = collections.defaultdict(list)
prompts = collections.defaultdict(list)
for path in go_files():
    rel = os.path.relpath(path, API)
    lines = open(path, encoding="utf-8").read().split("\n")
    for i, line in enumerate(lines):
        if ("a.route" in line or "Cat.Resolve" in line) and CLASS_RE.search(line):
            fn, doc = enclosing(lines, i)
            sites[CLASS_RE.search(line).group(1)].append((rel, i + 1, fn, doc))
        m = PROMPT_RE.match(line)
        if m:
            prompts[rel].append((m.group(1), i + 1, doc_above(lines, i)))

w = sys.stdout.write
total = sum(len(v) for v in sites.values())
w(f"## 附录 A · 全部 LLM 调用点（{total} 处，按档分组）\n")
for cls in ORDER:
    rows = sorted(sites.get(cls, []))
    w(f"\n### `{cls.lower()}` — {CLASS_ZH[cls]}\n\n共 {len(rows)} 处。\n\n")
    w("| 调用点 | 位置 | 这次调用在做什么（取自代码自己的注释） |\n|---|---|---|\n")
    for rel, ln, fn, doc in rows:
        w(f"| `{fn}` | `{rel}:{ln}` | {cell(doc, 150)} |\n")

w(f"\n\n## 附录 B · Prompt 存放位置（{sum(len(v) for v in prompts.values())} 个常量）\n")
for rel in sorted(prompts):
    w(f"\n**`{rel}`**\n\n| 常量 | 行 | 说明 |\n|---|---|---|\n")
    for name, ln, doc in sorted(prompts[rel], key=lambda x: x[1]):
        w(f"| `{name}` | {ln} | {cell(doc, 140)} |\n")

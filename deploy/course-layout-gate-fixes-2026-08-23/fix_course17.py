#!/usr/bin/env python3
"""Give course-17's PPT-sized figures the whole screen.

課 17（数据分析）ships 20 slide-sized figures. On seven slices a figure shares
the screen with its question, so the figure is squeezed into whatever vertical
space the question leaves — exactly the self-letterboxing that made the other
courses' interactions unreadable, only here it is the LAYOUT doing it.

Two changes:

  MODAL (5 slices: #4 #7 #11 #12 #13)
      One figure + one question in a `split-horizontal`. The layout becomes
      `full` with the figure taking the slot, and the question is authored
      `presentation: "modal"` (§9.8) so it arrives in a dialog over the figure.
      The slot keeps only a compact launcher, so the question is still visible
      and re-openable — the student can dismiss it to study the figure.

  SPLIT (2 slices: #9 #10)
      richText + TWO figures + a question crammed into one `grid`, which stacks
      vertically — every one of the four got a quarter of the screen. Each
      becomes three slices: one figure, the other figure, then the explanation
      beside the question. The two figure slices are pure-reading pages that
      complete on entry (the runtime supports an initial terminal step), so
      下一步 is live immediately.

Adding slices changes the course's step_count (13 → 17) and, like any
definition edit, changes the content hash — which resets in-progress course-17
sessions.

    python3 fix_course17.py           # write definitions/course-17.json
    python3 fix_course17.py --apply   # PUT it
"""
import copy
import json
import os
import subprocess
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.abspath(os.path.join(HERE, "..", ".."))
API = "https://mind-api.uni-robot.cn/api/v1"
OUT = os.path.join(HERE, "definitions")
SLUG = "course-17"

# Slices whose single figure + question become figure-full-slot + modal question.
MODAL_SLICES = {
    "slice-sample-privacy-boundary",
    "slice-chart-choice",
    "slice-reliability-claim-boundary",
    "slice-results-paragraph-template",
    "slice-ai-check-transfer",
}

# Slices that split into [figure, figure, explanation|question].
SPLIT_SLICES = {
    "slice-two-group-crosstab": [
        ("slice-two-group-figure", "两组比较：先看这张图"),
        ("slice-crosstab-figure", "分类比例：再看这张图"),
    ],
    "slice-correlation-regression": [
        ("slice-correlation-figure", "相关：先看这张图"),
        ("slice-regression-figure", "回归：再看这张图"),
    ],
}

NAV = {"revisit": "restore-completed-state", "autoNext": False,
       "previous": "allowed", "manualNext": "after-completion"}


def admin_key() -> str:
    for line in open(os.path.join(ROOT, ".deploy-local", "env.prod"), encoding="utf-8"):
        if line.startswith("OSS_ADMIN_KEY="):
            return line.split("=", 1)[1].strip()
    sys.exit("OSS_ADMIN_KEY not found in .deploy-local/env.prod")


def curl(args):
    return subprocess.run(["curl", "-sS"] + args, capture_output=True, text=True).stdout


def block_of(slice_, type_):
    for b in slice_.get("blocks") or []:
        if b["type"] == type_:
            return b
    return None


def question_of(slice_):
    for b in slice_.get("blocks") or []:
        if b["type"] in ("singleChoice", "fillBlank"):
            return b
    return None


def figure_slice(slice_id, title, images_block, item, objective_ids, seconds):
    """A pure-reading page: one figure, the whole slot, completes on entry."""
    bid = f"{slice_id}-image"
    return {
        "id": slice_id,
        "title": title,
        "objectiveIds": list(objective_ids),
        "estimatedSeconds": seconds,
        "blocks": [{
            "id": bid,
            "type": "images",
            "presentation": "single",
            "items": [dict(item, id=f"{bid}-item")],
        }],
        "layout": {"preset": "full", "slots": [{"id": "main", "blockIds": [bid]}]},
        "narrations": [],
        "workflow": {
            "version": "1.0",
            "initialStepId": "read",
            "initialState": {"visibleBlockIds": [bid], "enabledBlockIds": [bid]},
            "steps": [{"id": "read", "enterActions": [{"type": "completeSlice"}], "transitions": []}],
        },
        "navigation": dict(NAV),
    }


def to_modal(slice_, log):
    """Figure takes the whole slot; the question moves into a dialog over it."""
    fig = block_of(slice_, "images")
    q = question_of(slice_)
    if not fig or not q:
        return
    q["presentation"] = "modal"
    others = [b["id"] for b in slice_["blocks"] if b["id"] not in (fig["id"], q["id"])]
    slice_["layout"] = {"preset": "full",
                        "slots": [{"id": "main", "blockIds": [fig["id"], *others, q["id"]]}]}
    log.append(f"{slice_['id']}: figure -> full slot; '{q['id']}' -> presentation:modal")


def split(slice_, names, log):
    """richText + two figures + question -> figure, figure, explanation|question."""
    fig = block_of(slice_, "images")
    rich = block_of(slice_, "richText") or block_of(slice_, "text")
    q = question_of(slice_)
    items = fig.get("items") or []
    if not fig or not q or not rich or len(items) < 2:
        return [slice_]

    per = max(int((slice_.get("estimatedSeconds") or 180) / 3), 30)
    figures = [figure_slice(sid, title, fig, item, slice_.get("objectiveIds") or [], per)
               for (sid, title), item in zip(names, items)]

    tail = copy.deepcopy(slice_)
    tail["blocks"] = [rich, q]
    tail["estimatedSeconds"] = per
    tail["layout"] = {"preset": "split-horizontal", "ratio": "1:1",
                      "slots": [{"id": "left", "blockIds": [rich["id"]]},
                                {"id": "right", "blockIds": [q["id"]]}]}
    init = tail["workflow"].setdefault("initialState", {})
    init["visibleBlockIds"] = [rich["id"], q["id"]]
    init["enabledBlockIds"] = [q["id"]]
    q.pop("presentation", None)  # no figure here to sit under — inline is right

    log.append(f"{slice_['id']}: split into {[s['id'] for s in figures]} + itself "
               f"(richText | question)")
    return [*figures, tail]


def main() -> int:
    apply = "--apply" in sys.argv
    key = admin_key()
    os.makedirs(OUT, exist_ok=True)

    meta = {c["slug"]: c for c in json.loads(
        curl(["-H", f"Authorization: Bearer {key}", f"{API}/admin/courses"]))["courses"]}
    doc = json.loads(curl(["-H", f"Authorization: Bearer {key}",
                           f"{API}/admin/courses/{SLUG}/definition"]))["definition"]
    before = copy.deepcopy(doc)

    log = []
    for part in doc["course"]["parts"]:
        out = []
        for s in part["slices"]:
            if s["id"] in MODAL_SLICES:
                to_modal(s, log)
                out.append(s)
            elif s["id"] in SPLIT_SLICES:
                out.extend(split(s, SPLIT_SLICES[s["id"]], log))
            else:
                out.append(s)
        part["slices"] = out

    for line in log:
        print("  " + line)
    total = sum(len(p["slices"]) for p in doc["course"]["parts"])
    print(f"\n{len(log)} slices changed; course is now {total} slices "
          f"(was {sum(len(p['slices']) for p in before['course']['parts'])})")

    json.dump(doc, open(os.path.join(OUT, f"{SLUG}.json"), "w", encoding="utf-8"),
              ensure_ascii=False, indent=1)
    json.dump(before, open(os.path.join(OUT, f"{SLUG}.before.json"), "w", encoding="utf-8"),
              ensure_ascii=False, indent=1)

    if apply:
        m = meta.get(SLUG, {})
        body = {"definition": doc, "blurb": m.get("blurb", ""),
                "cardIds": m.get("card_ids") or [], "category": m.get("category") or ""}
        if m.get("introduction") is not None:
            body["introduction"] = m["introduction"]
        payload = os.path.join(OUT, f".{SLUG}.payload.json")
        json.dump(body, open(payload, "w", encoding="utf-8"), ensure_ascii=False)
        res = curl(["-X", "PUT", "-H", f"Authorization: Bearer {key}",
                    "-H", "Content-Type: application/json",
                    "--data-binary", f"@{payload}",
                    f"{API}/admin/courses/{SLUG}/definition"])
        os.remove(payload)
        print(f"PUT -> {res.strip()[:200]}")
    else:
        print("(dry run; --apply to PUT)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

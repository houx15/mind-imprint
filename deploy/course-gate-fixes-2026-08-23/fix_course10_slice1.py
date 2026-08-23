#!/usr/bin/env python3
"""course-10 · piece-1-1-first-reaction (slice 1) — stop gating the slice on a
passive reference panel.

The slice shows two interactive-HTML blocks side by side:

  left  `p111-paper-homepages`  — the A/B paper screenshots. Reference material.
                                  Its own status line reads "A/B 原文截图和提示
                                  已并排显示；右侧继续作答", i.e. it TELLS the
                                  student the answering happens on the right.
  right `p114-opening`          — the actual three-page task (first reaction →
                                  what to check → write one checkable sentence).

The authored workflow required `interaction.completed` from BOTH before
reaching its `completeSlice` step. A student does the task on the right, presses
完成任务, and 下一步 is still grey — because an unmarked 完成 button on the
reference panel has not been clicked, and nothing on screen says it must be.
With `manualNext: "after-completion"` there is no other exit. Verified live on
Phoebe's account: right-only → 下一步 disabled; both → enabled.

The panel stays visible and enabled and still emits its events (they remain
recorded as process data) — it simply no longer GATES completion. The real task
does.

Reads the live definition + its catalog fields, rewrites that one slice's
workflow, and PUTs it back.

  IMPORTANT: PUT /admin/courses/{slug}/definition overwrites blurb / cardIds /
  category / introduction with whatever the request carries, so those are read
  back from GET /admin/courses and re-sent verbatim. Changing the definition
  also changes its hash, which RESETS in-progress sessions for this course.

  OSS_ADMIN_KEY is read from .deploy-local/env.prod (gitignored); no secret
  lives in this file.

    python3 fix_course10_slice1.py --dry-run   # print the diff, change nothing
    python3 fix_course10_slice1.py             # apply
"""
import json
import os
import subprocess
import sys

API = os.environ.get("MIND_API", "https://mind-api.uni-robot.cn")
SLUG = "course-10"
SLICE_ID = "piece-1-1-first-reaction"
TASK_BLOCK = "p114-opening"
PANEL_BLOCK = "p111-paper-homepages"

NEW_WORKFLOW = {
    "version": "1.0",
    "initialStepId": "wait-task",
    "initialState": {
        "visibleBlockIds": [PANEL_BLOCK, TASK_BLOCK],
        "enabledBlockIds": [PANEL_BLOCK, TASK_BLOCK],
    },
    "steps": [
        {
            "id": "wait-task",
            # Both stay enabled: the panel is still fully usable and still
            # reports its own events, it just isn't a gate any more.
            "enterActions": [
                {"type": "enable", "targetId": PANEL_BLOCK},
                {"type": "enable", "targetId": TASK_BLOCK},
            ],
            "transitions": [
                {"on": {"type": "interaction.completed", "sourceId": TASK_BLOCK}, "to": "finish"}
            ],
        },
        {
            "id": "finish",
            "enterActions": [{"type": "clearFocus"}, {"type": "completeSlice"}],
            "transitions": [],
        },
    ],
}


def admin_key() -> str:
    root = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
    env = os.path.join(root, ".deploy-local", "env.prod")
    key = os.environ.get("OSS_ADMIN_KEY", "")
    if not key and os.path.exists(env):
        for line in open(env, encoding="utf-8"):
            if line.startswith("OSS_ADMIN_KEY="):
                key = line.split("=", 1)[1].strip()
                break
    if not key:
        sys.exit("OSS_ADMIN_KEY not set (env or .deploy-local/env.prod)")
    return key


def curl(key: str, url: str, method: str = "GET", body: str | None = None) -> str:
    cmd = ["curl", "-fsS", "-X", method, "-H", f"Authorization: Bearer {key}"]
    if body is not None:
        cmd += ["-H", "Content-Type: application/json", "--data-binary", "@-"]
    cmd.append(url)
    r = subprocess.run(cmd, input=body, capture_output=True, text=True)
    if r.returncode != 0:
        sys.exit(f"{method} {url} failed: {r.stderr.strip()}")
    return r.stdout


def main() -> int:
    dry = "--dry-run" in sys.argv
    key = admin_key()

    doc = json.loads(curl(key, f"{API}/api/v1/admin/courses/{SLUG}/definition"))
    definition = doc["definition"]
    listing = json.loads(curl(key, f"{API}/api/v1/admin/courses"))
    meta = next((c for c in listing["courses"] if c["slug"] == SLUG), None)
    if meta is None:
        sys.exit(f"{SLUG} not found in /admin/courses")

    target = None
    for part in definition["course"]["parts"]:
        for sl in part.get("slices", []):
            if sl.get("id") == SLICE_ID:
                target = sl
    if target is None:
        sys.exit(f"slice {SLICE_ID} not found in {SLUG}")

    ids = {b.get("id") for b in target.get("blocks", [])}
    missing = {TASK_BLOCK, PANEL_BLOCK} - ids
    if missing:
        sys.exit(f"expected blocks missing from {SLICE_ID}: {sorted(missing)}")

    if target["workflow"] == NEW_WORKFLOW:
        print("already applied — nothing to do")
        return 0

    print("--- current workflow ---")
    print(json.dumps(target["workflow"], ensure_ascii=False, indent=1))
    print("--- new workflow ---")
    print(json.dumps(NEW_WORKFLOW, ensure_ascii=False, indent=1))
    if dry:
        print("\n(dry run — nothing sent)")
        return 0

    target["workflow"] = NEW_WORKFLOW
    payload = {
        "definition": definition,
        "blurb": meta.get("blurb", ""),
        "cardIds": meta.get("card_ids") or [],
        "category": meta.get("category") or "",
        "introduction": meta.get("introduction"),
    }
    curl(key, f"{API}/api/v1/admin/courses/{SLUG}/definition", "PUT",
         json.dumps(payload, ensure_ascii=False))
    print(f"\nPUT ok — {SLUG} slice 1 now completes on {TASK_BLOCK} alone.")
    print("NOTE: the definition hash changed, so in-progress sessions for this course reset.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

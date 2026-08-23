#!/usr/bin/env python3
"""Rewrite published CourseDefinition 2.0 workflows that can dead-end a student.

Six transforms, each addressing a gate pattern found by sweeping every
published course. All of them keep the pedagogy and only change what BLOCKS
progression — correctness, narration and video all still happen and are still
recorded (铁律④: friction becomes signal, never a wall).

T1 answer.correct -> block.completed
    A transition that fires only on a CORRECT answer strands anyone who answers
    wrong: the block locks or exhausts, `answer.correct` never comes, and the
    slice has no other exit.

T2 submit-correct -> submit-correct-or-exhausted(2) / submit-any
    `submit-correct` means unlimited attempts and NO completion until the answer
    is right (see the renderer's evaluateSubmission). On a free-text fillBlank
    that is effectively unwinnable. Graded blocks keep grading but now complete
    after a second attempt; ungraded ones complete on submission.

T3 order-trap -> order-independent gate
    A chain `wait-A -> wait-B -> finish` over blocks that are BOTH live from the
    start. Answering B first locks B (submit-any locks on completion), so when
    the workflow later waits on B its event can never fire again — a permanent
    stick, reproduced live on course-30 #6. Rebuilt as the product automaton
    over the gating blocks, so any answering order reaches the same finish.

T4 inert-start -> the interaction is live immediately
    `enabledBlockIds: []` plus a first step whose only exit is `narration.ended`
    leaves the interaction on screen but dead for the whole narration (~45s).
    Students click it, nothing happens, they leave. Narration still plays; it
    just no longer gates the block.

T5 wait-only -> completes on entry
    `narration.ended -> timer.elapsed(30-80s) -> completeSlice` gives a reading
    page a hidden countdown with 下一步 greyed out and no indication why. These
    become pure-reading slices that complete on entry (the runtime supports an
    initial terminal step), so 下一步 is live at once and the student reads for
    as long as they like.

T6 watch-to-end -> add an explicit learner exit
    A video gated `video-ended-and-interactions-completed` must reach the real
    `ended` event AND clear every required cue. On course-23's 80-minute film,
    seeking past the cues at 6:47/7:59 leaves it unreachable. The existing paths
    are kept and a `student.continue` exit is added alongside them.

    python3 fix_gates.py           # write definitions/ + a diff report
    python3 fix_gates.py --apply   # PUT them to prod
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

# course-23's 80-minute film and course-33's zero-duration entry video.
VIDEO_FAILSAFE = {("course-23", "watch-the-film"), ("course-33", "video-entry")}

# Which transforms run on which course, and nothing else. Editing a definition
# changes its content hash, which RESETS every in-progress session for that
# course — so the blast radius is deliberately narrow:
#
#   DEADEND  T1/T2/T3/T6 only. These four fix gates that PERMANENTLY trap a
#            student (proven live on course-30 #6). Anyone actually stuck on one
#            of them has no progress to lose, so the reset is worth it. Applied
#            to every published course where the sweep found such a gate.
#   REPORTED T4/T5 as well — the narration/hidden-timer gates that read as
#            broken but do eventually clear. Judgement calls about pacing, so
#            they are applied ONLY to the two courses these were reported on
#            rather than to all 31 courses that share the pattern.
DEADEND = {"t1", "t2", "t3", "t6"}
REPORTED = DEADEND | {"t4", "t5"}

POLICY = {
    "course-03": DEADEND, "course-04": DEADEND, "course-06": DEADEND,
    "course-11": DEADEND, "course-12": DEADEND, "course-18": DEADEND,
    "course-19": DEADEND, "course-23": DEADEND, "course-30": DEADEND,
    "course-33": DEADEND,
    "course-25": REPORTED, "course-26": REPORTED,
}


def admin_key() -> str:
    env = os.path.join(ROOT, ".deploy-local", "env.prod")
    for line in open(env, encoding="utf-8"):
        if line.startswith("OSS_ADMIN_KEY="):
            return line.split("=", 1)[1].strip()
    sys.exit("OSS_ADMIN_KEY not found in .deploy-local/env.prod")


def curl(args):
    return subprocess.run(["curl", "-sS"] + args, capture_output=True, text=True).stdout


def admin_get(key, path):
    return curl(["-H", f"Authorization: Bearer {key}", f"{API}{path}"])


# ---------------------------------------------------------------- transforms


def gating_chain(wf):
    """The linear list of block ids the workflow consumes, in order, plus the
    waiting steps and the final step it ends at. ([], None, None) if not linear."""
    steps = {st["id"]: st for st in wf["steps"]}
    order, waits = [], []
    sid, seen = wf.get("initialStepId"), set()
    while sid in steps and sid not in seen:
        seen.add(sid)
        trs = steps[sid].get("transitions") or []
        if not trs:
            return order, waits, sid
        if len(trs) != 1:
            return [], [], None
        on = trs[0].get("on") or {}
        if on.get("type") not in ("block.completed", "answer.submitted"):
            return [], [], None
        order.append(on.get("sourceId"))
        waits.append(steps[sid])
        sid = trs[0].get("to")
    return order, waits, sid


def t1_answer_correct(slice_, log):
    for st in slice_["workflow"]["steps"]:
        for tr in st.get("transitions") or []:
            if (tr.get("on") or {}).get("type") == "answer.correct":
                tr["on"]["type"] = "block.completed"
                log.append(f"T1 {st['id']}: answer.correct -> block.completed")


def t2_completion_rules(slice_, log):
    for b in slice_.get("blocks") or []:
        if (b.get("completion") or {}).get("rule") != "submit-correct":
            continue
        graded = (b.get("assessment") or {}).get("mode") == "graded"
        b["completion"] = ({"rule": "submit-correct-or-exhausted", "maxAttempts": 2}
                           if graded else {"rule": "submit-any"})
        log.append(f"T2 {b['id']}: submit-correct -> {b['completion']['rule']}")


def t3_order_trap(slice_, log):
    wf = slice_["workflow"]
    order, waits, final_id = gating_chain(wf)
    if len(order) < 2 or final_id is None:
        return
    enabled = set((wf.get("initialState") or {}).get("enabledBlockIds") or [])
    if not all(b in enabled for b in order):
        return  # a genuine staged reveal, not an order trap
    steps = {st["id"]: st for st in wf["steps"]}
    final_step = steps[final_id]
    gating = set(order)

    def keep(actions):
        """Drop enable/focus of the gating blocks; those are live from the start
        and the product states manage focus themselves."""
        out = []
        for a in actions or []:
            if a.get("type") == "enable" and a.get("targetId") in gating:
                continue
            if a.get("type") == "focus" and (a.get("target") or {}).get("blockId") in gating:
                continue
            out.append(a)
        return out

    def name(done):
        if len(done) == len(order):
            return final_id
        return "gate-" + "".join("1" if b in done else "0" for b in order)

    new_steps, queue, made = [], [frozenset()], set()
    while queue:
        done = queue.pop(0)
        if done in made:
            continue
        made.add(done)
        if len(done) == len(order):
            continue
        remaining = [b for b in order if b not in done]
        actions = keep(waits[len(done)]["enterActions"])
        actions.append({"type": "focus", "target": {"blockId": remaining[0]}})
        new_steps.append({
            "id": name(done),
            "enterActions": actions,
            "transitions": [{"on": {"type": "block.completed", "sourceId": b},
                             "to": name(done | {b})} for b in remaining],
        })
        for b in remaining:
            queue.append(done | {b})

    new_steps.append({"id": final_id,
                      "enterActions": final_step["enterActions"],
                      "transitions": final_step.get("transitions") or []})
    wf["steps"] = new_steps
    wf["initialStepId"] = name(frozenset())
    log.append(f"T3 order-independent gate over {order} ({len(new_steps)} steps)")


def t4_inert_start(slice_, log):
    """First step waits ONLY on narration.ended and the next step just enables a
    block: merge them so the block is live from the start."""
    wf = slice_["workflow"]
    steps = {st["id"]: st for st in wf["steps"]}
    first = steps.get(wf.get("initialStepId"))
    if not first:
        return
    trs = first.get("transitions") or []
    if len(trs) != 1 or (trs[0].get("on") or {}).get("type") != "narration.ended":
        return
    nxt = steps.get(trs[0]["to"])
    if not nxt:
        return
    enables = [a["targetId"] for a in (nxt.get("enterActions") or [])
               if a.get("type") in ("enable", "show")]
    if not enables:
        return
    first["enterActions"] = (first.get("enterActions") or []) + list(nxt.get("enterActions") or [])
    first["transitions"] = nxt.get("transitions") or []
    wf["steps"] = [st for st in wf["steps"] if st["id"] != nxt["id"]]
    init = wf.setdefault("initialState", {})
    en = init.setdefault("enabledBlockIds", [])
    vis = init.setdefault("visibleBlockIds", [])
    for b in enables:
        if b not in en:
            en.append(b)
        if b not in vis:
            vis.append(b)
    log.append(f"T4 merged '{nxt['id']}' into '{first['id']}'; live from entry: {enables}")


def t5_wait_only(slice_, log):
    """narration -> timer -> completeSlice: nothing to DO. Complete on entry."""
    wf = slice_["workflow"]
    steps = {st["id"]: st for st in wf["steps"]}
    first = steps.get(wf.get("initialStepId"))
    if not first:
        return
    chain, sid, seen = [], wf.get("initialStepId"), set()
    while sid in steps and sid not in seen:
        seen.add(sid)
        chain.append(steps[sid])
        trs = steps[sid].get("transitions") or []
        if not trs:
            break
        if len(trs) != 1:
            return
        if (trs[0].get("on") or {}).get("type") not in ("narration.ended", "timer.elapsed"):
            return
        sid = trs[0]["to"]
    if len(chain) < 2:
        return
    terminal = chain[-1]
    if not any(a.get("type") == "completeSlice" for a in terminal.get("enterActions") or []):
        return
    keep = [a for st in chain for a in (st.get("enterActions") or [])
            if a.get("type") not in ("startTimer", "cancelTimer")]
    wf["steps"] = [{"id": first["id"], "enterActions": keep, "transitions": []}]
    wf["initialStepId"] = first["id"]
    log.append(f"T5 wait-only -> completes on entry (dropped {len(chain) - 1} waiting steps)")


def t6_video_failsafe(slug, slice_, log):
    if (slug, slice_["id"]) not in VIDEO_FAILSAFE:
        return
    wf = slice_["workflow"]
    steps = {st["id"]: st for st in wf["steps"]}
    first = steps.get(wf.get("initialStepId"))
    if not first:
        return
    targets = {tr["to"] for tr in first.get("transitions") or []}
    if not targets:
        return
    to = sorted(targets)[0]
    if any((tr.get("on") or {}).get("type") == "student.continue"
           for tr in first["transitions"]):
        return
    first["transitions"].append({"on": {"type": "student.continue"}, "to": to})
    log.append(f"T6 added a student.continue exit on '{first['id']}' -> '{to}'")


def transform(slug, doc, allowed):
    report = []
    for part in doc["course"]["parts"]:
        for s in part["slices"]:
            log = []
            # Order matters: the restructuring transforms run before the local
            # ones, so T3 sees the chain T4/T5 leave behind.
            if "t5" in allowed:
                t5_wait_only(s, log)
            if "t4" in allowed:
                t4_inert_start(s, log)
            if "t1" in allowed:
                t1_answer_correct(s, log)
            if "t3" in allowed:
                t3_order_trap(s, log)
            if "t2" in allowed:
                t2_completion_rules(s, log)
            if "t6" in allowed:
                t6_video_failsafe(slug, s, log)
            if log:
                report.append((s["id"], log))
    return report


# --------------------------------------------------------------------- main


def main() -> int:
    apply = "--apply" in sys.argv
    key = admin_key()
    os.makedirs(OUT, exist_ok=True)

    meta = {c["slug"]: c for c in json.loads(admin_get(key, "/admin/courses"))["courses"]}
    slugs = [s for s in sys.argv[1:] if not s.startswith("--")] or sorted(POLICY)
    unknown = [s for s in slugs if s not in POLICY]
    if unknown:
        sys.exit(f"no policy for {unknown} — add it to POLICY deliberately")

    total = 0
    for slug in slugs:
        raw = admin_get(key, f"/admin/courses/{slug}/definition")
        try:
            doc = json.loads(raw)["definition"]
        except Exception:
            print(f"{slug}: no 2.0 definition, skipped")
            continue
        before = copy.deepcopy(doc)
        report = transform(slug, doc, POLICY[slug])
        if not report:
            continue
        total += sum(len(v) for _, v in report)
        print(f"\n### {slug}  {meta.get(slug, {}).get('title', '')}")
        for sid, log in report:
            print(f"  {sid}")
            for line in log:
                print(f"      {line}")
        json.dump(doc, open(os.path.join(OUT, f"{slug}.json"), "w", encoding="utf-8"),
                  ensure_ascii=False, indent=1)
        json.dump(before, open(os.path.join(OUT, f"{slug}.before.json"), "w", encoding="utf-8"),
                  ensure_ascii=False, indent=1)

        if apply:
            m = meta.get(slug, {})
            body = {
                "definition": doc,
                "blurb": m.get("blurb", ""),
                "cardIds": m.get("card_ids") or [],
                "category": m.get("category") or "",
            }
            if m.get("introduction") is not None:
                body["introduction"] = m["introduction"]
            payload = os.path.join(OUT, f".{slug}.payload.json")
            json.dump(body, open(payload, "w", encoding="utf-8"), ensure_ascii=False)
            res = curl(["-X", "PUT", "-H", f"Authorization: Bearer {key}",
                        "-H", "Content-Type: application/json",
                        "--data-binary", f"@{payload}",
                        f"{API}/admin/courses/{slug}/definition"])
            os.remove(payload)
            print(f"  PUT -> {res.strip()[:200]}")

    print(f"\n{total} gate changes across {len(slugs)} courses"
          f"{' — APPLIED' if apply else ' (dry run; --apply to PUT)'}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

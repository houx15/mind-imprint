#!/usr/bin/env python3
"""course-05 · piece-2-1 (slice 3) — the OPCVL lab's completion gate.

`stepDone` required the student to pick the CORRECT option on the two
multiple-choice steps before the step counted as done; `allDone()` gates the
完成任务 button, the button is the only producer of the block's
`interaction.completed`, and the slice's `manualNext` is "after-completion".
So a student who could not land on the intended answer had 完成任务 grey and
下一步 grey with nothing else on screen to do — comprehension was gating
PROGRESSION.

The gate becomes "did you answer", not "were you right". Correctness is still
computed, still shown in the per-step feedback, and still recorded in the
`completed` payload (`item.isCorrect` / `item.correctAnswer` are unchanged), so
the signal reaches the evaluation pipeline instead of becoming a roadblock —
the same correction already applied to course-20's two writer interactions.

The file is 2.5 MB (embedded imagery), so it is patched in place from the live
copy rather than committed twice. The anchor is an exact string; if it is
missing the script refuses to write anything.

    python3 patch_opcvl_lab.py IN.html OUT.html            # apply
    python3 patch_opcvl_lab.py --reverse IN.html OUT.html  # roll back
"""
import sys

BEFORE = '      const choiceDone = !step.choice || ans.choice === step.choice.correct;\n'
AFTER = (
    "      // Gate on HAVING ANSWERED, never on being right: correctness is a\n"
    "      // recorded signal (see item.isCorrect in the completed payload), not a\n"
    "      // barrier — a wrong pick must never leave the student with no exit.\n"
    "      const choiceDone = !step.choice || !!ans.choice;\n"
)


def main() -> int:
    args = sys.argv[1:]
    reverse = "--reverse" in args
    args = [a for a in args if a != "--reverse"]
    if len(args) != 2:
        print(__doc__)
        return 2
    src_path, out_path = args
    src = open(src_path, encoding="utf-8").read()

    old, new = (AFTER, BEFORE) if reverse else (BEFORE, AFTER)
    if old not in src:
        if new in src:
            print(f"already {'reverted' if reverse else 'patched'}: {src_path}")
            return 3
        print(f"ANCHOR NOT FOUND in {src_path} — refusing to write", file=sys.stderr)
        return 1
    if src.count(old) != 1:
        print(f"anchor appears {src.count(old)}x — expected exactly 1", file=sys.stderr)
        return 1

    open(out_path, "w", encoding="utf-8").write(src.replace(old, new, 1))
    print(f"{'reverted' if reverse else 'patched'} -> {out_path}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

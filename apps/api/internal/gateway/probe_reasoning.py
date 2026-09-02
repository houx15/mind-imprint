#!/usr/bin/env python3
"""Measure, per model, whether reasoning can be turned OFF and whether its
EFFORT can be dialled — on the channel we actually use.

Why measure instead of read the docs: a reasoning knob the upstream accepts and
silently ignores produces a perfectly valid answer, just a slow and expensive
one. Only the reasoning-token count can tell "honored" from "ignored", which is
the whole reason models.json carries thinkingOff per PROVIDER rather than
per model.

  DASHSCOPE_API_KEY=... python3 probe_reasoning.py [model ...]

Prints one row per model: reasoning tokens under each setting, and a verdict.
"""

import json
import os
import sys
import urllib.error
import urllib.request

BASE = os.environ.get(
    "DASHSCOPE_BASE_URL",
    "https://llm-wjdjxs6f0x41w996.cn-beijing.maas.aliyuncs.com/compatible-mode/v1",
)
KEY = os.environ.get("DASHSCOPE_API_KEY", "")

DEFAULT_MODELS = [
    "deepseek-v4-pro",
    "deepseek-v4-flash",
    "qwen3.8-max",
    "qwen3.7-max",
    "kimi-k3",
    "kimi-k2.6",
    "kimi-k2.7-code",
    "glm-5.2",
    "glm-5.1",
    "ZHIPU/GLM-5.3",
    "glm-5.3-flash",
    "ZHIPU/GLM-5.3-Flash",
]

# Hard enough that a reasoning model will actually spend tokens on it, short
# enough that a non-reasoning answer is still correct.
QUESTION = (
    "A shelf holds 7 boxes. Each box holds 13 pens, except two boxes that hold "
    "9 each. How many pens in total? Answer with the number only."
)


def call(model: str, extra: dict) -> tuple[int | None, int | None, str | None]:
    """Return (reasoning_tokens, completion_tokens, error)."""
    body = {
        "model": model,
        "messages": [{"role": "user", "content": QUESTION}],
        "max_tokens": 2000,
        **extra,
    }
    req = urllib.request.Request(
        f"{BASE}/chat/completions",
        data=json.dumps(body).encode(),
        headers={"Content-Type": "application/json", "Authorization": f"Bearer {KEY}"},
    )
    try:
        with urllib.request.urlopen(req, timeout=180) as r:
            d = json.load(r)
    except urllib.error.HTTPError as e:
        try:
            msg = json.load(e).get("error", {}).get("message", "")
        except Exception:
            msg = e.reason
        return None, None, f"HTTP {e.code}: {str(msg)[:60]}"
    except Exception as e:  # network, timeout
        return None, None, str(e)[:60]
    usage = d.get("usage", {})
    details = usage.get("completion_tokens_details") or {}
    return details.get("reasoning_tokens", 0), usage.get("completion_tokens"), None


def cell(v: int | None, err: str | None) -> str:
    return "err" if err else ("-" if v is None else str(v))


def median(xs: list[int]) -> int:
    s = sorted(xs)
    return s[len(s) // 2]


def sample(model: str, extra: dict, n: int) -> tuple[list[int], str | None]:
    """n repeats of one setting. Reasoning-token counts vary a lot run to run."""
    out: list[int] = []
    for _ in range(n):
        r, _, e = call(model, extra)
        if e:
            return out, e
        out.append(r or 0)
    return out, None


def main() -> None:
    if not KEY:
        sys.exit("DASHSCOPE_API_KEY not set")
    args = [a for a in sys.argv[1:] if not a.startswith("-n")]
    n = next((int(a[2:]) for a in sys.argv[1:] if a.startswith("-n")), 3)
    models = args or DEFAULT_MODELS
    print(f"reasoning tokens, median of n={n} per setting\n")
    print(f"{'model':<22} {'default':>8} {'off':>6} {'low':>6} {'high':>6}   verdict")
    print("-" * 82)
    for m in models:
        base, base_e = sample(m, {}, n)
        off, off_e = sample(m, {"enable_thinking": False}, n)
        low, low_e = sample(m, {"reasoning_effort": "low"}, n)
        high, high_e = sample(m, {"reasoning_effort": "high"}, n)

        base_m = median(base) if base else None
        off_m = median(off) if off else None
        low_m = median(low) if low else None
        high_m = median(high) if high else None

        if base_e:
            verdict = f"unavailable — {base_e}"
        elif not base_m:
            verdict = "no reasoning by default"
        else:
            parts = ["off: WORKS" if not off_e and not off_m else "off: IGNORED"]
            # 🚨 Only a MONOTONIC rise counts. A single pair can differ by a
            # third in either direction purely by luck, so the first version of
            # this script cheerfully reported "effort: WORKS (140->103)" — high
            # spending LESS than low, which is not a control, it is noise wearing
            # a verdict's clothes. Requiring every low sample below every high
            # sample is what makes a claim here worth anything.
            if low_e or high_e:
                parts.append("effort: rejected")
            elif low and high and max(low) < min(high):
                parts.append(f"effort: WORKS ({low_m}->{high_m})")
            else:
                parts.append(f"effort: no effect ({low_m}~{high_m})")
            verdict = ", ".join(parts)

        print(
            f"{m:<22} {cell(base_m, base_e):>8} {cell(off_m, off_e):>6} "
            f"{cell(low_m, low_e):>6} {cell(high_m, high_e):>6}   {verdict}"
        )


if __name__ == "__main__":
    main()

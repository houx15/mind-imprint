#!/usr/bin/env python3
"""Is the "native controls over a segmented video" trap anywhere else?

course-12's sift-check.html attached the browser's native <video> controls
while a timeupdate clamp refused to honour the native timeline. This sweeps
every published course's interactiveHtml assets for the same shape.

These files inline their recordings as base64 data: URIs (megabytes), but the
script sits at the END of the file — so we Range-fetch only the tail and grep
that, instead of pulling ~100MB.

    python3 sweep_native_controls.py
"""
import json
import os
import subprocess
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.abspath(os.path.join(HERE, "..", ".."))
API = "https://mind-api.uni-robot.cn/api/v1"
EMAIL = "phoebe@demo.mindimprint.local"
PASSWORD = "phoebe-dev-pass"
COOKIES = os.path.join(HERE, ".cookies.txt")
TAIL = 400_000  # bytes from the end — comfortably covers the script block


def admin_key() -> str:
    for line in open(os.path.join(ROOT, ".deploy-local", "env.prod"), encoding="utf-8"):
        if line.startswith("OSS_ADMIN_KEY="):
            return line.split("=", 1)[1].strip()
    sys.exit("OSS_ADMIN_KEY not found")


def curl(args):
    return subprocess.run(["curl", "-sS"] + args, capture_output=True, text=True).stdout


def main() -> int:
    key = admin_key()
    slugs = [c["slug"] for c in json.loads(curl(
        ["-H", f"Authorization: Bearer {key}", f"{API}/admin/courses"]))["courses"]]
    out = curl(["-c", COOKIES, "-X", "POST", "-H", "Content-Type: application/json",
                "-d", json.dumps({"email": EMAIL, "password": PASSWORD}), f"{API}/auth/signin"])
    if '"error"' in out:
        sys.exit(f"signin failed: {out[:200]}")

    findings, scanned = [], 0
    for slug in slugs:
        raw = curl(["-H", f"Authorization: Bearer {key}", f"{API}/admin/courses/{slug}/definition"])
        try:
            doc = json.loads(raw)["definition"]
        except Exception:
            continue
        paths = sorted({b["source"] for p in doc["course"]["parts"] for s in p["slices"]
                        for b in s["blocks"] if b.get("type") == "interactiveHtml" and b.get("source")})
        if not paths:
            continue
        signed = curl(["-b", COOKIES, "-X", "POST", "-H", "Content-Type: application/json",
                       "-d", json.dumps({"paths": paths}), f"{API}/courses/{slug}/asset-urls"])
        try:
            urls = json.loads(signed)["assetUrls"]
        except Exception:
            print(f"  ?? {slug}: asset-urls failed", file=sys.stderr)
            continue
        for rel, url in urls.items():
            tail = curl(["-r", f"-{TAIL}", url])
            scanned += 1
            has_ctrl = "setAttribute('controls'" in tail or 'setAttribute("controls"' in tail
            has_seg = "seg.end" in tail or "SEG[" in tail
            if has_ctrl and has_seg:
                findings.append((slug, rel))
            elif has_ctrl:
                findings.append((slug, rel + "   [controls, but no segment clamp — fine]"))

    print(f"\nscanned {scanned} interactiveHtml assets across {len(slugs)} courses")
    if not findings:
        print("no other asset attaches native <video> controls.")
    for slug, rel in findings:
        print(f"  {slug}  {rel}")
    if os.path.exists(COOKIES):
        os.remove(COOKIES)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

#!/usr/bin/env python3
"""Download the live sift-check.html into original/ so the patch has a base.

Reads it the way a student does (the asset-urls endpoint is session-protected,
so an admin bearer will not do) using the public demo account documented in
apps/web/src/shell/AppShell.tsx.
"""
import json
import os
import subprocess
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
API = "https://mind-api.uni-robot.cn/api/v1"
SLUG = "course-12"
REL = "interactions/html/sift-check.html"
EMAIL = "phoebe@demo.mindimprint.local"
PASSWORD = "phoebe-dev-pass"
COOKIES = os.path.join(HERE, ".cookies.txt")


def curl(args):
    return subprocess.run(["curl", "-sS"] + args, capture_output=True, text=True).stdout


def main() -> int:
    out = curl(["-c", COOKIES, "-X", "POST", "-H", "Content-Type: application/json",
                "-d", json.dumps({"email": EMAIL, "password": PASSWORD}), f"{API}/auth/signin"])
    if '"error"' in out:
        sys.exit(f"signin failed: {out[:300]}")
    out = curl(["-b", COOKIES, "-X", "POST", "-H", "Content-Type: application/json",
                "-d", json.dumps({"paths": [REL]}), f"{API}/courses/{SLUG}/asset-urls"])
    try:
        url = json.loads(out)["assetUrls"][REL]
    except Exception:
        sys.exit(f"asset-urls failed: {out[:400]}")
    dest_dir = os.path.join(HERE, "original")
    os.makedirs(dest_dir, exist_ok=True)
    dest = os.path.join(dest_dir, "sift-check.html")
    code = subprocess.run(["curl", "-sS", "-o", dest, "-w", "%{http_code}", url],
                          capture_output=True, text=True).stdout.strip()
    os.path.exists(COOKIES) and os.remove(COOKIES)
    print(f"{code}  {REL} -> {dest} ({os.path.getsize(dest)}B)")
    return 0 if code.startswith("2") else 1


if __name__ == "__main__":
    raise SystemExit(main())

#!/usr/bin/env python3
"""Upload the patched interaction HTML through the sanctioned authoring endpoint.

Each file in patched/ is named `<slug>__<basename>.html` and goes back to its
existing object key `courses/<slug>/interactions/html/<basename>` — the SAME key
it was read from, so no CourseDefinition changes, no definition-hash change and
no session reset for these.

Uploading is NOT deploying: mind-oss.uni-robot.cn answers with
`X-Swift-CacheTime: 2592000` (30 days), so a same-key replacement keeps serving
the old bytes from the edge until the CDN directory is refreshed. Run
cdn_refresh.py (or the console's 刷新预热 → 目录刷新) afterwards, then re-read.

    python3 upload_assets.py            # dry run: show what would go where
    python3 upload_assets.py --apply
"""
import json
import os
import subprocess
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.abspath(os.path.join(HERE, "..", ".."))
API = os.environ.get("MIND_API", "https://mind-api.uni-robot.cn")
PATCHED = os.path.join(HERE, "patched")


def admin_key() -> str:
    for line in open(os.path.join(ROOT, ".deploy-local", "env.prod"), encoding="utf-8"):
        if line.startswith("OSS_ADMIN_KEY="):
            return line.split("=", 1)[1].strip()
    sys.exit("OSS_ADMIN_KEY not found in .deploy-local/env.prod")


def main() -> int:
    apply = "--apply" in sys.argv
    key = admin_key()
    files = sorted(f for f in os.listdir(PATCHED) if f.endswith(".html"))
    dirs = set()
    ok = 0
    for name in files:
        slug, base = name.split("__", 1)
        rel = f"interactions/html/{base}"
        path = os.path.join(PATCHED, name)
        size = os.path.getsize(path)
        dirs.add(f"https://mind-oss.uni-robot.cn/courses/{slug}/interactions/html/")
        if not apply:
            print(f"  would PUT courses/{slug}/{rel}  ({size}B)")
            continue
        presign = subprocess.run(
            ["curl", "-fsS", "-X", "POST",
             "-H", f"Authorization: Bearer {key}",
             "-H", "Content-Type: application/json",
             "-d", json.dumps({"relativePath": rel, "contentType": "text/html", "size": size}),
             f"{API}/api/v1/admin/courses/{slug}/asset-upload-url"],
            capture_output=True, text=True)
        if presign.returncode != 0:
            print(f"  PRESIGN FAILED {slug}/{rel}: {presign.stderr[:200]}", file=sys.stderr)
            continue
        info = json.loads(presign.stdout)
        put = subprocess.run(
            ["curl", "-sS", "-o", os.path.join(HERE, ".put.out"), "-w", "%{http_code}",
             "-X", "PUT", "-H", "Content-Type: text/html",
             "--data-binary", f"@{path}", info["putUrl"]],
            capture_output=True, text=True)
        code = put.stdout.strip()
        print(f"  {code}  {info['objectKey']}  ({size}B)")
        if code.startswith("2"):
            ok += 1
    stray = os.path.join(HERE, ".put.out")
    if os.path.exists(stray):
        os.remove(stray)
    print(f"\n{ok if apply else len(files)} of {len(files)} files"
          f"{' uploaded' if apply else ' would upload'}")
    print("\nCDN directories to refresh (uploading alone does NOT reach students):")
    for d in sorted(dirs):
        print("  " + d)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

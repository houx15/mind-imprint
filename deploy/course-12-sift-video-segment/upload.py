#!/usr/bin/env python3
"""Upload the patched sift-check.html back to its EXISTING object key.

Same key it was read from, so the CourseDefinition is untouched: no
definition-hash change and no in-progress session reset.

Uploading is NOT deploying. mind-oss.uni-robot.cn answers with
`X-Swift-CacheTime: 2592000` (30 days), so the edges keep serving the old
bytes until the CDN directory below is refreshed — and edges can disagree with
each other, so a `curl` that returns the new bytes does NOT prove students see
them. Verify in a real browser afterwards.

    python3 upload.py            # dry run
    python3 upload.py --apply
"""
import json
import os
import subprocess
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.abspath(os.path.join(HERE, "..", ".."))
API = os.environ.get("MIND_API", "https://mind-api.uni-robot.cn")
SLUG = "course-12"
REL = "interactions/html/sift-check.html"
SRC = os.path.join(HERE, "patched", "course-12__sift-check.html")


def admin_key() -> str:
    for line in open(os.path.join(ROOT, ".deploy-local", "env.prod"), encoding="utf-8"):
        if line.startswith("OSS_ADMIN_KEY="):
            return line.split("=", 1)[1].strip()
    sys.exit("OSS_ADMIN_KEY not found in .deploy-local/env.prod")


def main() -> int:
    if not os.path.exists(SRC):
        sys.exit(f"missing {SRC} — run patch.py first")
    size = os.path.getsize(SRC)
    if "--apply" not in sys.argv:
        print(f"  would PUT courses/{SLUG}/{REL}  ({size}B)")
        return 0
    key = admin_key()
    presign = subprocess.run(
        ["curl", "-fsS", "-X", "POST",
         "-H", f"Authorization: Bearer {key}",
         "-H", "Content-Type: application/json",
         "-d", json.dumps({"relativePath": REL, "contentType": "text/html", "size": size}),
         f"{API}/api/v1/admin/courses/{SLUG}/asset-upload-url"],
        capture_output=True, text=True)
    if presign.returncode != 0:
        sys.exit(f"presign failed: {presign.stderr[:300]}")
    info = json.loads(presign.stdout)
    out = os.path.join(HERE, ".put.out")
    put = subprocess.run(
        ["curl", "-sS", "-o", out, "-w", "%{http_code}", "-X", "PUT",
         "-H", "Content-Type: text/html", "--data-binary", f"@{SRC}", info["putUrl"]],
        capture_output=True, text=True)
    code = put.stdout.strip()
    print(f"  {code}  {info['objectKey']}  ({size}B)")
    if os.path.exists(out):
        os.remove(out)
    print("\nCDN directory to refresh (uploading alone does NOT reach students):")
    print(f"  https://mind-oss.uni-robot.cn/courses/{SLUG}/interactions/html/")
    return 0 if code.startswith("2") else 1


if __name__ == "__main__":
    raise SystemExit(main())

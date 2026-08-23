#!/usr/bin/env python3
"""Purge Aliyun CDN edge caches for one or more paths.

Re-uploading a course asset under its existing object key does NOT reach
students on its own: the CDN answers from the edge for a long time (observed
`X-Swift-CacheTime: 2592000` — 30 days — on mind-oss.uni-robot.cn), so a fresh
upload sits in OSS while every student keeps getting the stale copy. Every
same-key asset fix therefore has to be followed by a refresh.

Wraps CDN `RefreshObjectCaches` (2018-05-10) with the standard Aliyun RPC
signature. Credentials come from the gitignored .deploy-local/env.prod
(OSS_ACCESS_KEY_ID / OSS_ACCESS_KEY_SECRET); nothing secret lives here. If that
RAM user has no CDN permission the API says so and you purge from the console
instead (刷新预热 → 目录刷新).

    python3 cdn_refresh.py                       # the three dirs this sweep touched
    python3 cdn_refresh.py https://host/a/b/     # explicit paths
    python3 cdn_refresh.py --file https://host/a/b/x.html   # File instead of Directory
"""
import base64
import hashlib
import hmac
import json
import os
import subprocess
import sys
import urllib.parse
import uuid
from datetime import datetime, timezone

DEFAULT_DIRS = [
    "https://mind-oss.uni-robot.cn/courses/course-33/interactions/html/",
    "https://mind-oss.uni-robot.cn/courses/course-05/interactions/html/",
    "https://mind-oss.uni-robot.cn/courses/course-20/interactions/html/",
]


def creds() -> tuple[str, str]:
    root = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
    env = os.path.join(root, ".deploy-local", "env.prod")
    kid = os.environ.get("OSS_ACCESS_KEY_ID", "")
    sec = os.environ.get("OSS_ACCESS_KEY_SECRET", "")
    if (not kid or not sec) and os.path.exists(env):
        for line in open(env, encoding="utf-8"):
            if line.startswith("OSS_ACCESS_KEY_ID=") and not kid:
                kid = line.split("=", 1)[1].strip()
            elif line.startswith("OSS_ACCESS_KEY_SECRET=") and not sec:
                sec = line.split("=", 1)[1].strip()
    if not kid or not sec:
        sys.exit("OSS_ACCESS_KEY_ID / OSS_ACCESS_KEY_SECRET not found "
                 "(env or .deploy-local/env.prod)")
    return kid, sec


def enc(s: str) -> str:
    """Aliyun's percent-encoding: RFC3986, with +/*/%7E fixed up."""
    return (urllib.parse.quote(str(s), safe="")
            .replace("+", "%20").replace("*", "%2A").replace("%7E", "~"))


def call(action: str, params: dict, kid: str, sec: str) -> dict:
    q = {
        "Action": action,
        "Format": "JSON",
        "Version": "2018-05-10",
        "AccessKeyId": kid,
        "SignatureMethod": "HMAC-SHA1",
        "SignatureVersion": "1.0",
        "SignatureNonce": uuid.uuid4().hex,
        "Timestamp": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
        **params,
    }
    canonical = "&".join(f"{enc(k)}={enc(q[k])}" for k in sorted(q))
    to_sign = f"GET&{enc('/')}&{enc(canonical)}"
    sig = base64.b64encode(
        hmac.new((sec + "&").encode(), to_sign.encode(), hashlib.sha1).digest()
    ).decode()
    url = f"https://cdn.aliyuncs.com/?Signature={enc(sig)}&{canonical}"
    r = subprocess.run(["curl", "-sS", url], capture_output=True, text=True)
    try:
        return json.loads(r.stdout)
    except json.JSONDecodeError:
        return {"raw": r.stdout, "stderr": r.stderr}


def main() -> int:
    args = [a for a in sys.argv[1:] if a != "--file"]
    obj_type = "File" if "--file" in sys.argv else "Directory"
    paths = args or DEFAULT_DIRS
    kid, sec = creds()

    # RefreshObjectCaches takes newline-separated paths, up to 1000 per call.
    res = call("RefreshObjectCaches",
               {"ObjectPath": "\n".join(paths), "ObjectType": obj_type},
               kid, sec)
    if "RefreshTaskId" in res:
        print(f"refresh queued ({obj_type}): task {res['RefreshTaskId']}")
        for p in paths:
            print(f"  {p}")
        print("\nAliyun processes refreshes asynchronously — re-check the asset in a minute.")
        return 0
    print("refresh FAILED:", json.dumps(res, ensure_ascii=False)[:600], file=sys.stderr)
    print("\nFall back to the console: CDN → 刷新预热 → 目录刷新.", file=sys.stderr)
    return 1


if __name__ == "__main__":
    raise SystemExit(main())

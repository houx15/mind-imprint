"""One real lite reading session against production, to price an e2e run.

Registers its own fresh student (same shape as e2e/freshAccount.ts), pastes a
real article, and takes several coach turns. Afterwards the cost is read back
from llm_call for exactly this atom, so the number is the bill, not a model.
"""
import json
import secrets
import sys
import time
import urllib.error
import urllib.request

API = "https://mind-api.uni-robot.cn"
ORIGIN = "https://mind-web.uni-robot.cn"
JOIN = sys.argv[1] if len(sys.argv) > 1 else "G624-UXFE"

ARTICLE = """过去二十年，中国在可再生能源上的投入规模没有先例。政府补贴、地方配套和制造业产能同时发力，把太阳能与风电的建设成本推到了历史最低点。

根据国际能源署的统计，2023 年全球新增的太阳能发电装机中，超过一半位于中国境内。这一比例在过去五年里持续上升，并没有因为补贴退坡而回落。

中国的可再生能源新增装机量连续八年位居世界第一。仅 2023 年一年新增的风电与光伏装机，就超过了欧盟同期新增总量的两倍。

与此同时，中国仍然是全球二氧化碳排放总量最大的国家，2023 年约占全球总排放的三成。煤电在发电结构中的占比虽然下降，但绝对发电量仍在增长，以满足工业用电需求。

研究者对这两个事实如何共存有不同解释。一种观点认为，制造业外迁使得发达国家把排放转移到了中国；另一种观点强调，能源转型需要时间，新增装机要经过若干年才能置换掉存量煤电。

无论采用哪种口径，一个技术性的区别都不应被略过：装机容量衡量的是发电能力，而不是实际发出的电量。一台装机容量很大的风机，如果风况不佳或并网受限，实际发电量可能远低于它的铭牌数字。

因此，用新增装机量来衡量一个国家的能源转型进展，会系统性地高估进度。更能反映真实情况的指标，是可再生能源在总发电量中的占比，以及它替代了多少煤电。
"""


def call(method, path, data=None, cookie=None, timeout=180):
    req = urllib.request.Request(API + path, method=method)
    req.add_header("Origin", ORIGIN)
    if data is not None:
        req.add_header("Content-Type", "application/json")
        req.data = json.dumps(data).encode()
    if cookie:
        req.add_header("Cookie", cookie)
    try:
        with urllib.request.urlopen(req, timeout=timeout) as r:
            return r.status, r.read().decode(), r.headers.get_all("Set-Cookie") or []
    except urllib.error.HTTPError as e:
        return e.code, e.read().decode()[:300], []


tag = secrets.token_hex(4)
email = f"cost-walk-{tag}@demo.mindimprint.local"
pw = f"cost-walk-{tag}-pass"

st, body, _ = call("POST", "/api/v1/auth/signup",
                   {"email": email, "password": pw,
                    "display_name": "成本走查", "join_code": JOIN})
print(f"signup  {st} {body[:120]}")
if st >= 400:
    sys.exit(1)

st, body, cookies = call("POST", "/api/v1/auth/signin", {"email": email, "password": pw})
print(f"signin  {st}")
cookie = "; ".join(c.split(";")[0] for c in cookies)

st, body, _ = call("POST", "/api/v1/readings",
                   {"title": "中国的能源转型：投入与结果", "lang": "zh"}, cookie)
rid = json.loads(body).get("id") if st < 400 else None
print(f"reading {st} id={rid}")
if not rid:
    sys.exit(1)

st, body, _ = call("PUT", f"/api/v1/readings/{rid}/source",
                   {"text": ARTICLE, "title": "中国的能源转型：投入与结果"}, cookie)
print(f"source  {st} {body[:120]}")

turns = [
    "开始吧",
    "我觉得作者想说中国在可再生能源上投入很大",
    "装机量连续八年第一，这说明投入是真的很大",
    "我不太确定装机容量衡量的到底是什么",
    "哦，它说的是发电能力，不是真正发出来的电",
]
for msg in turns:
    t0 = time.time()
    st, body, _ = call("POST", f"/api/v1/readings/{rid}/coach", {"text": msg}, cookie)
    print(f"coach   {st} {time.time() - t0:5.1f}s  <- {msg}")
    if st >= 400:
        print("   ", body[:200])

print(f"\nATOM={rid}")

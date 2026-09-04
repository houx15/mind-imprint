import { test } from "@playwright/test";
import { runCamp } from "./runner";
import { STUDENTS } from "./students";

/**
 * 营地走查 —— 四个学生，四天，全程由一个**不知道产品长什么样**的模型来操作。
 *
 * 这不是一条要绿的测试。它不断言任何东西，它产出 `e2e/.camp/<学生>.json`：
 * 每一步她看懂了什么、有多清楚、有没有被托住、卡在哪儿。要看的是那份记录。
 *
 * 跑法（每个学生自己一条，可以只跑一个）：
 *
 *   bash apps/lite-web/e2e/run-stack.sh camp/camp.spec.ts -g 林知遥
 *
 * 🚨 四个学生跑在同一个库上，但**各自是各自的账号**（`freshStudent` 走真的注册
 * 口），所以互不干扰，顺序也无所谓。
 */

// 四个学生各自是各自的账号、各自的浏览器上下文，互不相干，所以并行跑。
// 一个学生四天是四十分钟量级，串起来要三个小时。
test.describe.configure({ mode: "parallel", retries: 0 });

// 🚨 trace 关掉。一天几十步、每步一张全屏截图，trace zip 会大到在收尾时写坏
// （2026-09-04 实测：`End of central directory record signature not found`，
// 走查本身是好的，红在收尾那一下）。这条 walk 的产出是 `.camp/*.json`，不是 trace。
test.use({ trace: "off", video: "off" });

const DAYS = (process.env.CAMP_DAYS ?? "1,2,3,4").split(",").map(Number);

for (const s of STUDENTS) {
  test(`营地 · ${s.name}`, async ({ browser }) => {
    // 一个学生四天，慢的时候一天二十多分钟。
    test.setTimeout(Number(process.env.CAMP_TEST_MS ?? 100 * 60_000));
    const log = await runCamp(browser, s, DAYS);
    const got = log.days.filter((d) => d.done).length;
    console.log(`[${s.key}] ${s.name}：四天里走到了 ${got} 天`);
  });
}

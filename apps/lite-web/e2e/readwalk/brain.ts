import fs from "node:fs";
import path from "node:path";
import { renderReadScreen, type ReadAffordances } from "./screen";

/**
 * 扮演学生的那个模型 —— 读英文文章的那一个。
 *
 * 和营地那份（`e2e/camp/brain.ts`）同一条纪律，只换了场景和动作表：
 *
 * 🚨 **换一家的模型当学生。** 印记走的是 `dialogue` 档（deepseek）。学生要是
 * 同一个脑子，它会「猜到」印记想听什么，把指令里说不清的地方自动补上 ——
 * 那样走完全程只证明这条路存在，不证明一个不知道路的人走得通。
 *
 * 🚨 **它只拿得到 screen.ts 返回的那一屏**：一屏字、正文、按钮、板。
 * 不给段 id、不给接口、不告诉它带读有几步。
 *
 * 学生设成一个英语中等的中国高中生：读得懂大意，长句和不认识的词会卡。
 * 这正是分级阅读要服务的那个人 —— 一个英语很好的学生走通了，说明不了什么。
 */

const BRAIN_MODEL = process.env.READ_BRAIN_MODEL ?? "qwen3.8-max";
const BASE =
  "https://llm-wjdjxs6f0x41w996.cn-beijing.maas.aliyuncs.com/compatible-mode/v1/chat/completions";

/** key 从环境变量或 `apps/api/.env.local` 读。和 camp/brain.ts 同一条：这是一件
 *  跟运行中的系统分开的走查工具，key 不进浏览器、不进 git、不进日志。
 *
 *  🚨 要找**两个**地方。`.env.local` 是 gitignore 的，所以它只存在于主检出里；
 *  在 worktree 里跑的时候 `cwd` 往上两级指向的是 worktree，那里没有这个文件。
 *  第一次在 worktree 里跑这条 walk 就是这么失败的。 */
function apiKey(): string {
  const fromEnv = (process.env.DASHSCOPE_API_KEY ?? "").trim();
  if (fromEnv) return fromEnv;

  const here = path.resolve(process.cwd(), "..", "..");
  // worktree 长这样：<repo>/.claude/worktrees/<name>。主检出在那之前。
  const mark = `${path.sep}.claude${path.sep}worktrees${path.sep}`;
  const main = here.includes(mark) ? here.slice(0, here.indexOf(mark)) : "";
  const tried: string[] = [];
  for (const root of [here, main].filter(Boolean)) {
    const file = path.join(root, "apps/api/.env.local");
    tried.push(file);
    if (!fs.existsSync(file)) continue;
    for (const line of fs.readFileSync(file, "utf8").split("\n")) {
      if (line.startsWith("#") || !line.includes("=")) continue;
      if (line.slice(0, line.indexOf("=")).trim() === "DASHSCOPE_API_KEY") {
        const v = line.slice(line.indexOf("=") + 1).trim();
        if (v) return v;
      }
    }
  }
  // 🚨 只报路径，不报内容。
  throw new Error(
    `找不到 DASHSCOPE_API_KEY —— 没有学生可演。找过：${tried.join("、")}；` +
      `也可以直接用环境变量 DASHSCOPE_API_KEY 跑。`,
  );
}

export type ReadAction =
  | { kind: "say"; text: string }
  | { kind: "click"; button: number }
  | { kind: "pick"; paragraph: number; sentence: string }
  | { kind: "place"; chip: number; bin: number }
  | { kind: "wait" }
  | { kind: "stuck"; why: string }
  | { kind: "leave" };

export type ReadBeat = {
  read: string;
  /** 我知道现在该干什么吗。1=完全不知道，5=一清二楚。 */
  clarity: number;
  /** 这一屏在帮我读懂这篇英文吗。1=它只是在考我，5=它真的在教我。 */
  taught: number;
  /** 具体哪一处看不懂 / 本该有东西却没有。没有就空着。 */
  snag: string;
  action: ReadAction;
};

const SYSTEM = `你在扮演一个真实的中国高中生，正在一个中文界面的网站上读一篇**英文**文章。

你的英语水平：能看懂大意，但长句子会绕晕，遇到不认识的词会卡住。你不是英语很好的
那种学生。**这一点很重要：读不懂就说读不懂，不要装。**

你面前是一个屏幕。你**只能**看到给你的那些字。没人告诉你这个产品有几步、下一步是
什么、某个按钮按下去会发生什么。像一个真的十六七岁的人那样：

- 看不懂就是看不懂。不要装懂，不要猜一个漂亮的说法来圆场。
- 不耐烦是正常的。同一件事被问第二遍、转了半天没动静，你会烦，可以直说。
- **它让你做什么，你就试着真的去做。** 让你从文章里挑一句，你就去正文里挑一句
  （用 pick）；给你一块板，你就把卡片摆进格子（用 place）。别每次都只回一句
  「我读完了」——那不是一个真的学生会做的事。
- 你有自己的想法。让你猜、让你判断的时候，按你自己的理解答，答错很正常。

每一步输出一个 JSON（不要输出别的）：

{
  "read": "一句话说你现在看到的是什么、它在等你做什么（你自己的理解，可以是错的）",
  "clarity": 1到5,
  "taught": 1到5,
  "snag": "哪一处看不懂/找不到/本该有东西却没有；没有就写空字符串",
  "action": { ... }
}

clarity = 我知道现在该干什么吗。1=完全不知道该点哪儿，5=一清二楚。
taught  = 这一屏在**帮我读懂这篇英文**吗。1=它只是在考我、在等我，
          5=它真的让我看懂了刚才没看懂的东西。

action 只能是下面这几种：

{"kind":"say","text":"你要打进对话框的话"}
{"kind":"click","button":序号}
{"kind":"pick","paragraph":段号,"sentence":"你要从那一段里划出来的那句话，逐字照抄"}
{"kind":"place","chip":卡片序号,"bin":格子序号}
{"kind":"wait"}
{"kind":"stuck","why":"我不知道该干什么了，因为……"}
{"kind":"leave"}

注意：
- 按不动（标着「←按不动」）的按钮不要按。
- pick 的 sentence 必须**逐字**抄自那一段的原文（英文原样抄，别翻译、别改标点）。
- 屏幕上有板的时候，先把板上的卡片一张张 place 完，再去点「摆好了」。
- 它显然在生成东西（在转、在打字）就 wait。
- 卡住了就 stuck，不要硬编一个动作。你卡住这件事本身就是我们要的信息。`;

export async function think(args: {
  screen: ReadAffordances;
  recent: string[];
  note?: string;
}): Promise<ReadBeat> {
  const user = [
    `你今天要做的事：在这个网站上把这篇英文文章读完。`,
    ``,
    args.recent.length
      ? `你刚才做过的几步（最近的在最后）：\n${args.recent.map((r) => "  - " + r).join("\n")}`
      : "",
    args.note ? `\n注意：${args.note}` : "",
    ``,
    renderReadScreen(args.screen),
  ]
    .filter(Boolean)
    .join("\n");

  const body = {
    model: BRAIN_MODEL,
    enable_thinking: false,
    temperature: 0.8,
    messages: [
      { role: "system", content: SYSTEM },
      { role: "user", content: user },
    ],
    response_format: { type: "json_object" as const },
  };

  let lastErr = "";
  for (let attempt = 0; attempt < 3; attempt++) {
    try {
      const r = await fetch(BASE, {
        method: "POST",
        headers: { "Content-Type": "application/json", Authorization: `Bearer ${apiKey()}` },
        body: JSON.stringify(body),
      });
      if (!r.ok) {
        // 🚨 报错里绝不带 key。
        lastErr = `HTTP ${r.status} ${(await r.text()).slice(0, 300)}`;
        await sleep(1500 * (attempt + 1));
        continue;
      }
      const j = (await r.json()) as { choices?: { message?: { content?: string } }[] };
      const beat = JSON.parse(firstObject(j.choices?.[0]?.message?.content ?? "")) as ReadBeat;
      if (!beat.action || typeof beat.action.kind !== "string") {
        lastErr = `学生没给出动作`;
        continue;
      }
      beat.clarity = clamp(beat.clarity);
      beat.taught = clamp(beat.taught);
      beat.read ??= "";
      beat.snag ??= "";
      return beat;
    } catch (e) {
      lastErr = e instanceof Error ? e.message : String(e);
      await sleep(1500 * (attempt + 1));
    }
  }
  throw new Error(`扮演学生的模型连着三次没给出动作：${lastErr}`);
}

function clamp(n: unknown): number {
  const v = Math.round(Number(n));
  return Number.isFinite(v) ? Math.min(5, Math.max(1, v)) : 3;
}

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

/** 数括号取第一个配平的对象。 */
function firstObject(s: string): string {
  let start = -1;
  let depth = 0;
  let inStr = false;
  let esc = false;
  for (let i = 0; i < s.length; i++) {
    const c = s[i];
    if (esc) {
      esc = false;
      continue;
    }
    if (inStr && c === "\\") esc = true;
    else if (c === '"') inStr = !inStr;
    else if (inStr) continue;
    else if (c === "{") {
      if (depth === 0) start = i;
      depth++;
    } else if (c === "}") {
      if (depth > 0 && --depth === 0 && start >= 0) return s.slice(start, i + 1);
    }
  }
  return s;
}

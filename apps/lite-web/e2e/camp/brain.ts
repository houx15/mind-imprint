import fs from "node:fs";
import path from "node:path";
import type { Student } from "./students";
import type { Affordances } from "./screen";
import { renderScreen } from "./screen";

/**
 * brain.ts —— 扮演学生的那个模型。
 *
 * ## 为什么要有它
 *
 * `journey-1/2/3` 那三条 walk 里，学生说的每一句话都是我**照着代码**写的：我知道
 * 第二关要三个站，所以我写「有没有几个真的个人网站可以给我看看」；我知道页面上
 * 的字必须逐字出自她的原话，所以我让她把整句话说出来。那三条走通了，证明的是
 * **这条路存在**，不是**一个不知道路的人走得通**。
 *
 * 四天的营里，学生不知道产品的路径。所以这里换一个模型来当学生，
 * 它只拿得到 `screen.ts` 返回的那一屏字和几个按钮——不知道有几关、不知道下一步
 * 是什么、不知道「视觉基调」按下去会发生什么。它看不懂就说看不懂，那句「看不懂」
 * 就是我们要的结果。
 *
 * ## 为什么用 qwen 而不是 deepseek
 *
 * 印记走的是 `dialogue` 档（`dashscope/deepseek-v4-pro`）。学生要是同一个模型，
 * 就成了同一个脑子自问自答——它会「猜到」印记想听什么，把界面上说不清的地方
 * 自动补上。换一家的模型当学生，补不上的地方才会露出来。
 */

const BRAIN_MODEL = process.env.CAMP_BRAIN_MODEL ?? "qwen3.8-max";
const BASE =
  "https://llm-wjdjxs6f0x41w996.cn-beijing.maas.aliyuncs.com/compatible-mode/v1/chat/completions";

/**
 * key 从 `apps/api/.env.local` 里读。
 *
 * 🚨 这不违反「密钥只在服务端」：这是一件**跟运行中的系统分开**的走查工具，跑在
 * 开发机上，key 没有进浏览器、没有进 git、没有进日志（下面任何一处都不打印它）。
 * 和 `cmd/routebench` 同一种东西。
 */
function apiKey(): string {
  const repo = path.resolve(process.cwd(), "..", "..");
  const raw = fs.readFileSync(path.join(repo, "apps/api/.env.local"), "utf8");
  for (const line of raw.split("\n")) {
    if (line.startsWith("#") || !line.includes("=")) continue;
    const k = line.slice(0, line.indexOf("=")).trim();
    if (k === "DASHSCOPE_API_KEY") return line.slice(line.indexOf("=") + 1).trim();
  }
  throw new Error("apps/api/.env.local 里没有 DASHSCOPE_API_KEY —— 营地模拟没有学生可演");
}

export type Action =
  | { kind: "say"; text: string }
  | { kind: "click"; button: number }
  | { kind: "hover"; button: number }
  | { kind: "fill"; field: number; text: string }
  | { kind: "wait" }
  | { kind: "leave" }
  | { kind: "stuck"; why: string };

export type Beat = {
  /** 我现在看懂了什么。 */
  read: string;
  /** 我知道现在该干什么吗。1 = 完全不知道，5 = 一清二楚。 */
  clarity: number;
  /** 这一屏在帮我往前走吗。1 = 它只是在等我，5 = 它明确地托着我。 */
  support: number;
  /** 具体哪一处看不懂 / 哪一处该有东西却没有。没有就留空。 */
  snag: string;
  action: Action;
};

const SYSTEM = `你在扮演一个真实的高中生，正在参加一个四天的项目制学习营。

你面前是一个屏幕。你**只能**看到给你的那段「屏幕上的字」和列出来的按钮、输入框。
你没有说明书，没人告诉你这个产品有几个步骤、下一步该是什么、某个按钮按下去会发生
什么。你要像一个真的十六七岁的人一样：

- 看不懂就是看不懂。**不要装懂，不要猜一个专业的说法来圆场。** 一个词你没见过
  （比如某个工具的名字），你不知道它按下去会发生什么，就照实说。
- 不耐烦是正常的。转了半天没动静、同一句话被问第二遍、要你写一大段字，你会烦。
- 你有自己的事要说。别人问你什么，你按你自己的性格答，不要答成一份完美的作业。
- 你不会读代码，不会打开开发者工具，不会去猜接口。

每一步你要输出一个 JSON（不要输出别的）：

{
  "read": "用一句话说你现在看到的是什么、系统在等你做什么（你自己的理解，可以是错的）",
  "clarity": 1到5的整数,
  "support": 1到5的整数,
  "snag": "如果有哪一处你看不懂、找不到、或者你觉得这里本该有东西却没有，写具体是哪一处；没有就写空字符串",
  "action": { ... }
}

clarity = 我知道现在该干什么吗。1=完全不知道该点哪儿，5=一清二楚。
support = 这一屏在帮我往前走吗。1=它只是在干等我，我得自己想出所有东西；
          5=它明确地给了我下一步、给了我可以判断的材料。

action 只能是下面五种之一：

{"kind":"say","text":"你要打进对话框的话"}        —— 屏幕上有能写字的对话框时用
{"kind":"click","button":序号}                    —— 按一个按钮（用列表里的序号）
{"kind":"hover","button":序号}                    —— 把光标停在某个东西上（不点下去）
{"kind":"fill","field":序号,"text":"写进去的字"}  —— 往某个输入框里写字
{"kind":"wait"}                                   —— 看起来在加载/在等它回话
{"kind":"stuck","why":"我不知道该干什么了，因为……"} —— 你真的走不下去了
{"kind":"leave"}                                  —— 今天这一段你觉得做完了

注意：
- 按不动（标着「←按不动」）的按钮不要按。
- 想说话就用 say，不用先去 fill 那个对话框。
- 只要屏幕在动、或者它显然在生成东西，就用 wait，不要乱按。
- **卡住了就 stuck，不要硬编一个动作**。你卡住这件事本身就是我们要的信息。`;

export async function think(args: {
  student: Student;
  /** 营地老师今天在白板上写的那一句。她只知道这一句。 */
  todayBrief: string;
  screen: Affordances;
  /** 她自己刚才做过的几步，防止绕圈。 */
  recent: string[];
  /** 上一步按完屏幕没变的话，告诉她。 */
  note?: string;
}): Promise<Beat> {
  const { student, todayBrief, screen, recent, note } = args;
  const user = [
    `你是谁：${student.selfIntro}`,
    ``,
    `今天营地老师在白板上写的：「${todayBrief}」`,
    ``,
    recent.length ? `你刚才做过的几步（最近的在最后）：\n${recent.map((r) => "  - " + r).join("\n")}` : "",
    note ? `\n注意：${note}` : "",
    ``,
    renderScreen(screen),
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
        // 🚨 报错里绝不带 key。这里带的是状态码和回包，够判断是限流还是请求坏了。
        lastErr = `HTTP ${r.status} ${(await r.text()).slice(0, 300)}`;
        await sleep(1500 * (attempt + 1));
        continue;
      }
      const j = (await r.json()) as { choices?: { message?: { content?: string } }[] };
      const raw = j.choices?.[0]?.message?.content ?? "";
      const beat = JSON.parse(firstObject(raw)) as Beat;
      if (!beat.action || typeof beat.action.kind !== "string") {
        lastErr = `学生没给出动作：${raw.slice(0, 300)}`;
        continue;
      }
      beat.clarity = clamp(beat.clarity);
      beat.support = clamp(beat.support);
      beat.read = beat.read ?? "";
      beat.snag = beat.snag ?? "";
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

/** 和 `internal/pbl/jsonwire.go` 同一件事：数括号取第一个配平的对象。 */
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

import fs from "node:fs";
import path from "node:path";
import { renderWriteScreen, type WriteAffordances } from "./screen";

/**
 * 扮演学生的那个模型 —— 写一篇文章的那一个。
 *
 * 和 `e2e/readwalk/brain.ts` 同一条纪律，只换了场景和动作表：
 *
 * 🚨 **换一家的模型当学生。** 印记走的是 `dialogue` 档（deepseek）。学生要是
 * 同一个脑子，它会「猜到」印记想听什么，把指令里说不清的地方自动补上 ——
 * 那样走完全程只证明这条路存在，不证明一个不知道路的人走得通。
 *
 * 🚨 **它只拿得到 screen.ts 返回的那一屏。** 不给 id、不给接口、
 * 不告诉它这个房间有几步。
 *
 * # 这条走查和阅读那条最大的不同
 *
 * 阅读那边学生的产出是「点一句、答一句」；写作这边她**必须真的写出字来**，
 * 而那正是这个房间最容易假绿的地方：一个只会回「好的我明白了」的学生，
 * 能把整条路走完，而一个字的作文都没有。所以人设里写死了两件事：
 * 她有一件**具体的、她自己的**事要写，以及她写出来的东西要像一个真的
 * 十六七岁的人写的 —— 有具体的细节，也有真实的毛病（例子没有解释、
 * 句子偏碎、结尾喊口号）。
 *
 * 🚨 **不要把她设成一个会写的学生。** 她要是每段都写得很好，这个房间里所有
 * 的诊断、优先级、板就全都测不到了 —— 那就成了给完美作业打分。
 */

const BRAIN_MODEL = process.env.WRITE_BRAIN_MODEL ?? "qwen3.8-max";
const BASE =
  "https://llm-wjdjxs6f0x41w996.cn-beijing.maas.aliyuncs.com/compatible-mode/v1/chat/completions";

/** key 从环境变量或 `apps/api/.env.local` 读。
 *
 *  🚨 要找**两个**地方。`.env.local` 是 gitignore 的，只存在于主检出里；
 *  在 worktree 里跑的时候 `cwd` 往上两级指向的是 worktree，那里没有这个文件。
 *  （readwalk 第一次在 worktree 里跑就是这么失败的，这里直接抄它的修法。） */
function apiKey(): string {
  const fromEnv = (process.env.DASHSCOPE_API_KEY ?? "").trim();
  if (fromEnv) return fromEnv;

  const here = path.resolve(process.cwd(), "..", "..");
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

export type WriteAction =
  | { kind: "say"; text: string }
  | { kind: "click"; button: number }
  | { kind: "write"; field: number; text: string }
  | { kind: "place"; chip: number; bin: number }
  | { kind: "wait" }
  | { kind: "stuck"; why: string }
  | { kind: "leave" };

export type WriteBeat = {
  read: string;
  /** 我知道现在该干什么吗。1=完全不知道，5=一清二楚。 */
  clarity: number;
  /** 这一屏在**教我写**吗。1=它只是在等我交东西，5=它真的让我知道该怎么改。 */
  taught: number;
  /** 具体哪一处看不懂 / 本该有东西却没有。没有就空着。 */
  snag: string;
  action: WriteAction;
};

/** 两个学生。语言不同，毛病也不同 —— 英文那个要露出中文语序的破绽。 */
export type WriteStudent = { key: string; lang: "zh" | "en"; persona: string; idea: string };

export const WRITE_STUDENTS: WriteStudent[] = [
  {
    key: "zh",
    lang: "zh",
    idea: "我想写学校食堂每天倒掉好多饭这件事",
    persona: [
      "你写中文。你的中文作文水平是中等偏上：能把事情说清楚，但常犯这几个毛病 ——",
      "举了例子就过去了，不解释它为什么能说明你的观点；结尾喜欢喊一句大道理；",
      "有时候一连几个短句，读起来碎。",
      "🚨 这些毛病是你**真实的样子**，不要刻意避开它们，也不要刻意表演它们。",
    ].join("\n"),
  },
  {
    key: "en",
    lang: "en",
    idea: "I want to write about the food waste in our school canteen",
    persona: [
      "你写英文。你是中国学生，英语中等：词汇够用，但写出来的句子常常是**按中文语序拼的**，",
      "而且会漏冠词、主谓不一致、比较句没比完（说 more 却没说比什么）。",
      "🚨 这些毛病是你**真实的样子**，不要刻意避开。你想表达的意思是清楚的，",
      "卡住的是怎么用英文把它说对。",
      "你**跟印记说话用中文**（界面是中文的，你也习惯用中文问问题），",
      "但**文章正文必须写英文**。",
    ].join("\n"),
  },
];

function systemFor(s: WriteStudent): string {
  return `你在扮演一个真实的中国高中生，正在一个中文界面的网站上写一篇文章。

${s.persona}

你要写的那件事：${s.idea}
这是**你自己**关心的事，你手上有一些真实的细节（你见过的场景、你数过的数字、
同学说过的话）。需要举例子的时候，就从这些里面编一个具体的出来 —— 要具体到
有时间、有地点、有数字，不要写「有很多同学都这样」这种空话。

你面前是一个屏幕。你**只能**看到给你的那些字。没人告诉你这个产品有几步、
下一步是什么、某个按钮按下去会发生什么。像一个真的十六七岁的人那样：

- 看不懂就是看不懂。不要装懂，不要猜一个漂亮的说法来圆场。
- 不耐烦是正常的。同一件事被问第二遍、转了半天没动静，你会烦，可以直说。
- **它让你做什么，你就试着真的去做。** 让你写一段，你就真的写出一段
  （用 write）；给你一块板，你就把卡片摆进格子（用 place）。
  🚨 别用「好的我明白了」糊弄过去 —— 这条走查里最没用的就是一个不写字的学生。
- 它给你意见之后，**按它说的去改**，改完把改过的那一段重新写进正文框里。
- 你有自己的想法。它说的你不同意，可以说出来。

每一步输出一个 JSON（不要输出别的）：

{
  "read": "一句话说你现在看到的是什么、它在等你做什么（你自己的理解，可以是错的）",
  "clarity": 1到5,
  "taught": 1到5,
  "snag": "哪一处看不懂/找不到/本该有东西却没有；没有就写空字符串",
  "action": { ... }
}

clarity = 我知道现在该干什么吗。1=完全不知道该点哪儿，5=一清二楚。
taught  = 这一屏在**教我怎么写**吗。1=它只是在等我交东西、或者只说「不够好」，
          5=它让我清楚地知道下一句该写什么、为什么。

action 只能是下面这几种：

{"kind":"say","text":"你要打进对话框的话"}
{"kind":"click","button":序号}
{"kind":"write","field":框的序号,"text":"你要打进那个框里的字"}
{"kind":"place","chip":卡片序号,"bin":格子序号}
{"kind":"wait"}
{"kind":"stuck","why":"我不知道该干什么了，因为……"}
{"kind":"leave"}

注意：
- 按不动（标着「←按不动」）的按钮不要按。
- **write 是往某个框里打字**（标着「←写文章正文的地方」的那个，或者落地页上
  那个让你说想写什么的框、弹窗里的题目和正文框）。**say 只用来跟印记说话**
  （标着「←跟印记说话的地方」的那个）。往作文框里跟印记聊天、或者往对话框里
  写作文，都是错的。
- 写正文的时候把**整段**写完整（三到五句），不要只写一句就交。
  🚨 **一段控制在 150 字以内。** 你的回答整个要装进一个 JSON 里，写太长会被
  截断，然后你会看到自己上一段停在半个字上 —— 那是你自己写太长了，不是网站的
  毛病，别反复去补它。
- 有些按钮要先写点字才会亮（比如「开始写作」）。按不动的时候先想想
  是不是该先往某个框里写点东西。
- 屏幕上有板的时候，先把板上的卡片一张张 place 完，**再去点那颗「标好了」**
  （板上只有它能把你标的结果交上去；卡片全摆完之前它是灰的）。
  🚨 别忘了这一步 —— 摆完不交，等于没标。
- 它显然在生成东西（在转、在打字）就 wait。
- 卡住了就 stuck，不要硬编一个动作。你卡住这件事本身就是我们要的信息。`;
}

export async function think(args: {
  student: WriteStudent;
  screen: WriteAffordances;
  recent: string[];
  note?: string;
}): Promise<WriteBeat> {
  const user = [
    `你今天要做的事：在这个网站上把这篇文章写出来。`,
    ``,
    args.recent.length
      ? `你刚才做过的几步（最近的在最后）：\n${args.recent.map((r) => "  - " + r).join("\n")}`
      : "",
    args.note ? `\n注意：${args.note}` : "",
    ``,
    renderWriteScreen(args.screen),
  ]
    .filter(Boolean)
    .join("\n");

  const body = {
    model: BRAIN_MODEL,
    enable_thinking: false,
    temperature: 0.8,
    // 🚨 一段作文 + 一段自述装在同一个 JSON 里，默认额度不够：实测她连着四步
    // 在补同一段被截掉的话（「停在'还'字那里」），而截断发生在**学生这个模型**
    // 的输出上，不在产品里。额度给够，再让她自己把段落写短一点。
    max_tokens: 2000,
    messages: [
      { role: "system", content: systemFor(args.student) },
      { role: "user", content: user },
    ],
    response_format: { type: "json_object" as const },
  };

  let lastErr = "";
  // 🚨 **耐心要配得上这条 walk 有多贵。**
  // 原来是三次、退避 1.5s/3s —— 加起来只等 4.5 秒。2026-09-11 线上那次，
  // 英文那个学生走到第 24 步（六百多字、二十多分钟）时 DashScope 抖了一下，
  // 连着三次 `fetch failed`，整条 walk 就这么没了，那二十分钟的模型钱一起没了。
  // 网络抖一下不是一条结论，不该把一整次观察作废。
  // 六次、指数退避封顶 20 秒 ≈ 一分钟的耐心，对一条要跑半小时的 walk 来说很便宜。
  for (let attempt = 0; attempt < BRAIN_ATTEMPTS; attempt++) {
    try {
      const r = await fetch(BASE, {
        method: "POST",
        headers: { "Content-Type": "application/json", Authorization: `Bearer ${apiKey()}` },
        body: JSON.stringify(body),
      });
      if (!r.ok) {
        // 🚨 报错里绝不带 key。
        lastErr = `HTTP ${r.status} ${(await r.text()).slice(0, 300)}`;
        await sleep(backoffMs(attempt));
        continue;
      }
      const j = (await r.json()) as { choices?: { message?: { content?: string } }[] };
      const beat = JSON.parse(firstObject(j.choices?.[0]?.message?.content ?? "")) as WriteBeat;
      if (!beat.action || typeof beat.action.kind !== "string") {
        // 🚨 这一支原来是**立刻**重来，没有任何间隔。第七轮走查里英文那条
        // 就死在这儿：模型连着六次回了没有 action 的 JSON，六次几乎同时发出，
        // 拿到的当然是同一个结果，一整条 walk 就没了。
        // 隔一下再要，采样才会变 —— 和网络出错那一支用同一条退避。
        lastErr = `学生没给出动作`;
        await sleep(backoffMs(attempt));
        continue;
      }
      beat.clarity = clamp(beat.clarity);
      beat.taught = clamp(beat.taught);
      beat.read ??= "";
      beat.snag ??= "";
      return beat;
    } catch (e) {
      lastErr = e instanceof Error ? e.message : String(e);
      await sleep(backoffMs(attempt));
    }
  }
  throw new Error(`扮演学生的模型连着 ${BRAIN_ATTEMPTS} 次没给出动作：${lastErr}`);
}

/** 学生这一边重试几次。见上面那段为什么不是三次。 */
const BRAIN_ATTEMPTS = 6;

/** 1.5s、3s、6s、12s、20s、20s —— 封顶，免得最后两次各等一分钟。 */
function backoffMs(attempt: number): number {
  return Math.min(20_000, 1500 * 2 ** attempt);
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

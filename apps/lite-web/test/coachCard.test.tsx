import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { useState } from "react";
import { render, screen, cleanup, fireEvent } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { CoachCard, type CoachCardSpec, type CoachCardAnswer } from "@lite/readings/CoachCard";

/**
 * CoachCard — 印记 hands her the step instead of describing it.
 *
 * The product ruling this whole file exists to hold:
 *
 *   > the questions are ladders, we use these to support students to dive to
 *   > think deep to connect his own interests. we do use them as gate
 *   > sometimes to avoid students' 胡乱阅读, but we do not serve as an exam,
 *   > 不要变成标准化考试, we think each child's thoughts are precious.
 *
 * So the load-bearing assertion here is a NEGATIVE one: nothing on this card
 * may read as right or wrong. There is no ✓, no ✗, no score, no 「正确」 —
 * the card carries no answer key because the server never sends one (there
 * isn't one to send). Her tap is a choice, not an attempt.
 */

const OPTIONS = [
  { blockId: "b1", quote: "城市地表以沥青和混凝土为主，白天吸热、夜里放热。" },
  { blockId: "b2", quote: "同一时期，郊区的夜间气温平均低了三摄氏度。" },
  { blockId: "b2", quote: "作者由此判断，热岛效应主要由建材决定。" },
];

const CHOOSE_SPAN: CoachCardSpec = {
  type: "choose_span",
  prompt: "哪一句你读着最不服气？",
  options: OPTIONS,
};

const PICK_IN_ARTICLE: CoachCardSpec = {
  type: "pick_in_article",
  prompt: "去文章里点出你觉得最站不住的那一句。",
};

const SHORT_TEXT: CoachCardSpec = {
  type: "short_text",
  prompt: "用你自己的话说说，作者漏掉了什么？",
};

/** Every shape a card can be on screen, for the sweeps that must hold on all
 *  of them at once. */
const EVERY_SHAPE: {
  name: string;
  card: CoachCardSpec;
  answered?: CoachCardAnswer;
  stale?: boolean;
  busy?: boolean;
}[] = [
  { name: "choose_span", card: CHOOSE_SPAN },
  { name: "pick_in_article", card: PICK_IN_ARTICLE },
  { name: "short_text", card: SHORT_TEXT },
  { name: "这一轮还在飞", card: CHOOSE_SPAN, busy: true },
  {
    name: "pick_in_article 已作答",
    card: PICK_IN_ARTICLE,
    answered: {
      type: "pick_in_article",
      prompt: PICK_IN_ARTICLE.prompt,
      choice: OPTIONS[0]!.quote,
      blockId: OPTIONS[0]!.blockId,
    },
  },
  {
    name: "choose_span 已作答",
    card: CHOOSE_SPAN,
    answered: {
      type: "choose_span",
      prompt: CHOOSE_SPAN.prompt,
      choice: OPTIONS[2]!.quote,
      blockId: OPTIONS[2]!.blockId,
    },
  },
  {
    name: "short_text 已作答",
    card: SHORT_TEXT,
    answered: { type: "short_text", prompt: SHORT_TEXT.prompt, choice: "他没算住在那儿的人。" },
  },
  { name: "折叠起来的旧卡片", card: CHOOSE_SPAN, stale: true },
];

/**
 * 🚨 一处对错都不许有——**包括属性**。
 *
 * 评审往卡片里注入过 `aria-label="答对了"` / `title="正确答案"` / `data-state="✓"`，
 * 旧扫描（只看 `textContent` + 一条只有英文的正则）12 个测试全绿。读屏用户会听到
 * 「答对了」，鼠标悬停会看到「正确答案」。所以这里扫的是 **textContent ＋
 * innerHTML**：属性、ARIA、class、data-* 一起进网。
 *
 * ⚠️ 词表里不能有裸的 `分`：它会在 分析 / 部分 / 分钟 上误伤。中文里该禁的是
 * 得分 / 分数 / 打分 / 满分 这些真的在记账的词。
 */
const FORBIDDEN = [
  "✓",
  "✔",
  "✗",
  "✘",
  "×",
  "√",
  "☑",
  "❌",
  "⭕",
  "正确",
  "错误",
  "答对",
  "答错",
  "做对",
  "做错",
  "对了",
  "错了",
  "得分",
  "分数",
  "打分",
  "满分",
  "score",
  "correct",
  "wrong",
  "right-answer",
];

/** markup 里偷偷说对错的另一半：英文 class / data / aria 值。 */
const FORBIDDEN_RE = /correct|incorrect|wrong|score|right-answer|graded|passed|failed/i;

/** 一次扫两面：文字 **和** markup（属性只活在后者里）。 */
function sweep(container: HTMLElement) {
  const surface = `${container.textContent ?? ""} ${container.innerHTML}`;
  for (const forbidden of FORBIDDEN) {
    expect(surface.includes(forbidden), `卡片上出现了「${forbidden}」`).toBe(false);
  }
  expect(FORBIDDEN_RE.test(surface), `卡片的 markup 里出现了判对错的词：${container.innerHTML}`).toBe(false);
}

afterEach(cleanup);

describe("CoachCard", () => {
  it("choose_span 把文章里的每一句都摆出来，点哪一句就回传哪一句", () => {
    const onAnswer = vi.fn();
    render(<CoachCard card={CHOOSE_SPAN} onAnswer={onAnswer} />);

    expect(screen.getByText(CHOOSE_SPAN.prompt)).toBeTruthy();
    for (const o of OPTIONS) {
      expect(screen.getByRole("button", { name: o.quote })).toBeTruthy();
    }

    fireEvent.click(screen.getByRole("button", { name: OPTIONS[1]!.quote }));

    // 逐字回传：服务端拿 choice 去原文里做字面子串核对，改一个标点，
    // 「她指了」这件事就没了（reading_coach.go 的 quoteIsArticleText）。
    expect(onAnswer).toHaveBeenCalledTimes(1);
    expect(onAnswer.mock.calls[0]![0]).toEqual({
      type: "choose_span",
      prompt: CHOOSE_SPAN.prompt,
      choice: OPTIONS[1]!.quote,
      blockId: OPTIONS[1]!.blockId,
    });
  });

  it("pick_in_article 不给选项 —— 那一句要她自己去文章里找", () => {
    render(<CoachCard card={PICK_IN_ARTICLE} onAnswer={vi.fn()} />);

    expect(screen.getByText(PICK_IN_ARTICLE.prompt)).toBeTruthy();
    expect(screen.queryAllByRole("button")).toHaveLength(0);
  });

  it("short_text 收下她自己的话", () => {
    const onAnswer = vi.fn();
    render(<CoachCard card={SHORT_TEXT} onAnswer={onAnswer} />);

    expect(screen.getByText(SHORT_TEXT.prompt)).toBeTruthy();
    const box = screen.getByRole("textbox");
    fireEvent.change(box, { target: { value: "  他只算了成本，没算住在那儿的人。  " } });
    fireEvent.click(screen.getByRole("button", { name: /提交回答/ }));

    expect(onAnswer).toHaveBeenCalledTimes(1);
    expect(onAnswer.mock.calls[0]![0]).toEqual({
      type: "short_text",
      prompt: SHORT_TEXT.prompt,
      choice: "他只算了成本，没算住在那儿的人。",
    });
  });

  it("空白的一句话不算作答，按钮点不动", () => {
    const onAnswer = vi.fn();
    render(<CoachCard card={SHORT_TEXT} onAnswer={onAnswer} />);

    fireEvent.change(screen.getByRole("textbox"), { target: { value: "   " } });
    fireEvent.click(screen.getByRole("button", { name: /提交回答/ }));

    expect(onAnswer).not.toHaveBeenCalled();
  });

  // 🚨 这条 2026-09-17 **反过来了**。
  //
  // 原来答完之后只留她选的那一句，理由是「几个选项并排摆着、中间标出来一个，
  // 读起来就是答案对照表」。产品负责人用真文章走完一遍之后逐字报的是另一头：
  // 「阅读卡片选择以后无法看到其他选项（贴句子的卡片也是），无法回退。」
  //
  // 而且它有代价：印记 下一轮讲的往往就是几个选项之间的差别，选项一收走，
  // 它只好在对话里把三个选项重抄一遍（真实截图）。
  //
  // 「不是对照表」这件事仍然要守，只是判据换成了更准的那个：**没有任何表示
  // 对错的记号**。下面同时断言这两条。
  it("答过的卡片留着全部选项，她选的那一句标出来，但已经点不动了", () => {
    const onAnswer = vi.fn();
    const { container } = render(
      <CoachCard
        card={CHOOSE_SPAN}
        onAnswer={onAnswer}
        answered={{
          type: "choose_span",
          prompt: CHOOSE_SPAN.prompt,
          choice: OPTIONS[2]!.quote,
          blockId: OPTIONS[2]!.blockId,
        }}
      />,
    );

    // 三句都还在屏幕上 —— 她回头看得见自己是在什么里面选的。
    for (const o of OPTIONS) {
      expect(screen.getByText(new RegExp(o.quote.slice(0, 10)))).toBeTruthy();
    }
    // 她选的那一句被标出来，用的是「你选的」，不是「对」。
    expect(screen.getByText("你选的")).toBeTruthy();
    // 🚨 铁律②：卡片上永远不出现对错。没被选的那两条不带任何记号。
    const text = container.textContent ?? "";
    for (const mark of ["✓", "✗", "×", "正确", "错误", "答对", "答错", "标准答案"]) {
      expect(text.includes(mark)).toBe(false);
    }
    // 整张卡片不再是可操作的东西
    expect(screen.queryAllByRole("button")).toHaveLength(0);
    expect(screen.queryAllByRole("textbox")).toHaveLength(0);
    // 🚨 只点最外层那一下是空转的：任何「不渲染按钮」的实现都能过。整棵子树
    // 每个节点都点一遍，才真的问出了「这张卡片上还有没有能触发作答的东西」。
    const everyNode = [container.firstElementChild!, ...container.querySelectorAll("*")];
    expect(everyNode.length).toBeGreaterThan(3);
    for (const node of everyNode) {
      fireEvent.click(node);
      fireEvent.keyDown(node, { key: "Enter" });
    }
    expect(onAnswer).not.toHaveBeenCalled();
  });

  it("答过的 short_text 也留着她写的那句", () => {
    render(
      <CoachCard
        card={SHORT_TEXT}
        onAnswer={vi.fn()}
        answered={{ type: "short_text", prompt: SHORT_TEXT.prompt, choice: "他没算住在那儿的人。" }}
      />,
    );

    expect(screen.getByText(/他没算住在那儿的人。/)).toBeTruthy();
    expect(screen.queryAllByRole("button")).toHaveLength(0);
  });

  it("turn 还在飞的时候点不出第二次作答", () => {
    const onAnswer = vi.fn();
    render(<CoachCard card={CHOOSE_SPAN} onAnswer={onAnswer} busy />);

    fireEvent.click(screen.getByRole("button", { name: OPTIONS[0]!.quote }));
    expect(onAnswer).not.toHaveBeenCalled();
  });

  // 🚨 铁律②/⑤：卡片上永远不出现对错。
  it.each(EVERY_SHAPE)("$name 上没有任何对/错的痕迹（文字 + 属性）", ({ card, answered, stale, busy }) => {
    const { container } = render(
      <CoachCard card={card} onAnswer={vi.fn()} answered={answered} stale={stale} busy={busy} />,
    );
    sweep(container);
  });

  /**
   * 🚨 卡片不许说屏幕的方位。
   *
   * 真实走查里第一张卡写死了「在左边文章里点出那一句」——而文章在桌面端排在
   * 右边、手机上排在下面（`02-first-reply-with-card.png` / `p2-phone-first-card.png`）。
   * 学生看到的**第一张卡片**把她指去了空白的那一边：她找不到 → 打字说「我读完
   * 了」→ hunt 判定正确地拒绝推进 → 印记 连着训她两次。
   *
   * 布局会随视口变，所以正确的修法不是换一个方位词，而是**一个都不提**。
   *
   * ⚠️ 词表里不能有裸的「上」「下」：「点一下」「读一下」全是常用词，会误伤。
   * 禁的是真的在说方位的那几个。同理只扫**文字和可读属性**，不扫 innerHTML：
   * markup 里合法地躺着 `text-left`、`border-left-color` 这类 CSS。
   */
  const DIRECTIONS = ["左", "右", "上边", "上面", "上方", "下边", "下面", "下方", "底下", "顶部", "旁边"];
  const DIRECTION_RE = /\b(left|right|above|below|beside|top|bottom)\b/i;

  /** 屏幕上读得到的一切字：文字 + 读屏/悬停能拿到的属性值。 */
  function readable(container: HTMLElement): string {
    const parts = [container.textContent ?? ""];
    for (const el of [container, ...container.querySelectorAll("*")]) {
      for (const attr of ["aria-label", "title", "placeholder", "alt"]) {
        const v = el.getAttribute(attr);
        if (v) parts.push(v);
      }
    }
    return parts.join(" ");
  }

  it.each(EVERY_SHAPE)("$name 的文案里一个方位词都没有", ({ card, answered, stale, busy }) => {
    const { container } = render(
      <CoachCard card={card} onAnswer={vi.fn()} answered={answered} stale={stale} busy={busy} />,
    );
    const surface = readable(container);
    for (const word of DIRECTIONS) {
      expect(surface.includes(word), `卡片上出现了方位词「${word}」——布局一变它就指错了`).toBe(false);
    }
    expect(DIRECTION_RE.test(surface), `卡片上出现了方位词：${surface}`).toBe(false);
  });

  /**
   * 产品负责人对着截图说的：*"ai's chat box and task card is not very clear"*。
   * 一张卡片和一个聊天气泡在同一列里几乎一样重，她分不出「这是在跟我说话」和
   * 「这是要我动手的东西」。DOM 上先有一个稳定的钩子，样式才有地方挂。
   */
  it.each(EVERY_SHAPE)("$name 在 DOM 上认得出自己是一张卡片", ({ card, answered, stale, busy }) => {
    const { container } = render(
      <CoachCard card={card} onAnswer={vi.fn()} answered={answered} stale={stale} busy={busy} />,
    );
    expect(container.querySelector("[data-coach-card]")).toBeTruthy();
  });

  /**
   * jsdom 不套样式表，所以任何组件测试都够不到 `::after { content: "✓" }`。
   * 这一条直接读源码——而 `index.css` 正是未来某个善意的「奖励小花样」会落地
   * 的地方。
   */
  it("index.css 的伪元素里没有任何对错记号（组件测试永远够不到这一层）", () => {
    const css = readFileSync(resolve(process.cwd(), "src/index.css"), "utf8");
    const contents = [...css.matchAll(/content\s*:\s*([^;}]+)/g)].map((m) => m[1]!.trim());
    for (const value of contents) {
      for (const forbidden of FORBIDDEN) {
        expect(
          value.includes(forbidden),
          `index.css 的 content: ${value} 里出现了「${forbidden}」——伪元素也算屏幕上的字`,
        ).toBe(false);
      }
      expect(FORBIDDEN_RE.test(value), `index.css 的 content: ${value} 在判对错`).toBe(false);
    }
    // 词表以后被加长时这一条会提醒：确实扫到了东西，不是空转。
    expect(contents.length).toBeGreaterThan(0);
  });

  /**
   * 铁律③ 一次只问一个：新卡片出现之后，旧的那张**不置灰**（一排死掉的灰色 UI
   * 读起来就是「这是你没做完的所有事」——计分板从后门溜进来），而是折叠成一行
   * 问题，点一下还能回去答。
   */
  describe("旧卡片折叠起来，标成「已替换」", () => {
    it("折叠的卡片只剩问题本身——选项不在屏幕上", () => {
      render(<CoachCard card={CHOOSE_SPAN} onAnswer={vi.fn()} stale />);

      expect(screen.getByText(new RegExp(CHOOSE_SPAN.prompt))).toBeTruthy();
      for (const o of OPTIONS) {
        expect(screen.queryByRole("button", { name: o.quote })).toBeNull();
      }
    });

    /**
     * 🚨 2026-09-20 推翻了这条原来的断言（「点一下重新展开，而且照样能答」）。
     * 产品负责人报的第 1 条：「对于多张卡片没有进行卡片管理……旧卡标记已替换
     * 并停止接收答案」。屏幕上同时有几张能答的卡，她答哪一张、印记 接哪一张，
     * 两边对不上。题目留着能看，答案只收当前那一张。
     */
    it("点一下折叠的卡片，题目回来了，但它不再收答案", () => {
      const onAnswer = vi.fn();
      render(<CoachCard card={CHOOSE_SPAN} onAnswer={onAnswer} stale />);

      fireEvent.click(screen.getByRole("button", { name: new RegExp(CHOOSE_SPAN.prompt) }));

      expect(screen.getByText(CHOOSE_SPAN.prompt)).toBeTruthy();
      expect(screen.getByText("已替换")).toBeTruthy();
      // 选项一个都不在：一排点不动的按钮读起来是「坏了」。
      for (const o of OPTIONS) {
        expect(screen.queryByRole("button", { name: o.quote })).toBeNull();
      }
      expect(onAnswer).not.toHaveBeenCalled();
    });

    it("已经答过的卡片不会被折叠——她说过的话不该缩回去", () => {
      render(
        <CoachCard
          card={SHORT_TEXT}
          onAnswer={vi.fn()}
          stale
          answered={{ type: "short_text", prompt: SHORT_TEXT.prompt, choice: "他没算住在那儿的人。" }}
        />,
      );
      expect(screen.getByText(/他没算住在那儿的人。/)).toBeTruthy();
    });
  });

  /**
   * 键盘/读屏：她按的那个按钮在作答之后被卸载了。什么都不接手的话，
   * `document.activeElement` 会掉回 `<body>`——答完一题回到文档顶部，而且没有
   * 任何提示告诉她刚才发生了什么。
   */
  describe("作答之后焦点有人接", () => {
    it("点完一个选项，焦点落在她的答案上，不是 body", () => {
      function Harness() {
        const [answered, setAnswered] = useState<CoachCardAnswer | null>(null);
        return <CoachCard card={CHOOSE_SPAN} onAnswer={setAnswered} answered={answered} />;
      }
      render(<Harness />);

      fireEvent.click(screen.getByRole("button", { name: OPTIONS[1]!.quote }));

      const her = screen.getByRole("status");
      expect(her.textContent).toContain(OPTIONS[1]!.quote);
      expect(document.activeElement).toBe(her);
      expect(document.activeElement?.tagName).not.toBe("BODY");
    });

    it("刷新回来的已作答卡片不抢焦点——她刚进房间，不该被拽到某张旧卡上", () => {
      render(
        <CoachCard
          card={SHORT_TEXT}
          onAnswer={vi.fn()}
          answered={{ type: "short_text", prompt: SHORT_TEXT.prompt, choice: "他没算住在那儿的人。" }}
        />,
      );
      expect(document.activeElement).toBe(document.body);
    });
  });

  /** 无障碍：问题和选项要被连在一起念出来，textarea 不能只有 placeholder 当名字。 */
  describe("读屏读得出这是一组东西", () => {
    it("卡片是一个 group，名字就是那个问题", () => {
      render(<CoachCard card={CHOOSE_SPAN} onAnswer={vi.fn()} />);
      expect(screen.getByRole("group", { name: CHOOSE_SPAN.prompt })).toBeTruthy();
    });

    it("short_text 的输入框自己有名字（placeholder 不算）", () => {
      render(<CoachCard card={SHORT_TEXT} onAnswer={vi.fn()} />);
      expect(screen.getByRole("textbox", { name: SHORT_TEXT.prompt })).toBeTruthy();
    });
  });
});

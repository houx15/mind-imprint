import { render, screen, cleanup, fireEvent } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import {
  CoachCard,
  composeBoardAnswer,
  parseBoardAnswer,
  boardItems,
  type CoachCardSpec,
} from "@lite/readings/CoachCard";

/**
 * 答完之后这张卡片还剩下什么 —— 产品负责人 2026-09-17 的第一条：
 *
 *   > 阅读卡片选择以后无法看到其他选项（贴句子的卡片也是），无法回退
 *
 * 三件事在这里钉住：
 *
 *  1. choose_span 答完之后，**全部选项还在**，她选的那一句标出来。
 *  2. 板摆完之后，**同一块板还在**（同样的格子、同样的位置），只是不能再动。
 *  3. 无论哪一种，屏幕上都不许出现任何表示对错的记号（铁律②）—— 这是把选项
 *     留下来之后唯一还需要守的那条，也是当初收走选项时真正想守的那条。
 *
 * 外加回车发送（第八条：「动手部分，按回车键无法发送」）。
 */

afterEach(cleanup);

const OPTIONS = [
  { blockId: "b1", quote: "城市地表以沥青和混凝土为主，白天吸热、夜里放热。", where: "第1段" },
  { blockId: "b4", quote: "同一时期，郊区的夜间气温平均低了三摄氏度。", where: "第4段" },
  { blockId: "b7", quote: "作者由此判断，热岛效应主要由建材决定。", where: "第7段" },
];

const CHOOSE_SPAN: CoachCardSpec = {
  type: "choose_span",
  prompt: "作者是怎么让你相信建材说了算的？",
  options: OPTIONS,
};

const LABEL_ROLES: CoachCardSpec = {
  type: "label_roles",
  prompt: "分析下列句子，判断它们各自属于哪一类论证成分。",
  options: OPTIONS,
  labels: ["主张", "证据", "限制", "背景", "对比"],
};

const WORD_BANK: CoachCardSpec = {
  type: "word_bank",
  prompt: "这几个词，你现在各自是什么状态？",
  words: [
    { blockId: "b1", term: "scrambling" },
    { blockId: "b1", term: "mitigate" },
    { blockId: "b4", term: "ambient" },
  ],
};

const NO_MARKS = ["✓", "✗", "×", "√", "正确", "错误", "答对", "答错", "标准答案", "得分"];

describe("答完之后的 choose_span", () => {
  it("三个选项一个不少，她选的那一句标着「你选的」", () => {
    render(
      <CoachCard
        card={CHOOSE_SPAN}
        onAnswer={vi.fn()}
        answered={{
          type: "choose_span",
          prompt: CHOOSE_SPAN.prompt,
          choice: OPTIONS[1]!.quote,
          blockId: OPTIONS[1]!.blockId,
        }}
      />,
    );
    for (const o of OPTIONS) {
      expect(screen.getByText(new RegExp(o.quote.slice(0, 8)))).toBeTruthy();
    }
    expect(screen.getByText("你选的")).toBeTruthy();
  });

  it("每个选项都带着它的段号 —— 一句摘出来的话，不说从哪儿来她判断不了", () => {
    render(
      <CoachCard
        card={CHOOSE_SPAN}
        onAnswer={vi.fn()}
        answered={{
          type: "choose_span",
          prompt: CHOOSE_SPAN.prompt,
          choice: OPTIONS[1]!.quote,
          blockId: OPTIONS[1]!.blockId,
        }}
      />,
    );
    for (const o of OPTIONS) {
      expect(screen.getByText(o.where!)).toBeTruthy();
    }
  });

  it("没答的时候也带段号", () => {
    render(<CoachCard card={CHOOSE_SPAN} onAnswer={vi.fn()} />);
    for (const o of OPTIONS) {
      expect(screen.getByText(o.where!)).toBeTruthy();
    }
  });

  // 🚨 这是把选项留下来之后唯一还要守的那条。当初收走选项的理由是「并排摆着
  // 读起来像答案对照表」—— 真正让它像对照表的是**记号**，不是并排。
  it("屏幕上没有任何表示对错的记号", () => {
    const { container } = render(
      <CoachCard
        card={CHOOSE_SPAN}
        onAnswer={vi.fn()}
        answered={{
          type: "choose_span",
          prompt: CHOOSE_SPAN.prompt,
          choice: OPTIONS[1]!.quote,
          blockId: OPTIONS[1]!.blockId,
        }}
      />,
    );
    const text = container.textContent ?? "";
    for (const mark of NO_MARKS) {
      expect(text.includes(mark)).toBe(false);
    }
  });

  // 兜底卡、或者正文换过之后重新分段 —— 她那一句已经不在选项里了。这时候摆一
  // 组和她无关的选项比什么都不摆更糟。
  it("她选的那一句不在选项里，就只显示她那一句", () => {
    render(
      <CoachCard
        card={CHOOSE_SPAN}
        onAnswer={vi.fn()}
        answered={{
          type: "choose_span",
          prompt: CHOOSE_SPAN.prompt,
          choice: "一句现在已经不在这篇文章里的话。",
        }}
      />,
    );
    expect(screen.getByText("一句现在已经不在这篇文章里的话。")).toBeTruthy();
    expect(screen.queryByText(new RegExp(OPTIONS[0]!.quote.slice(0, 8)))).toBeNull();
  });
});

describe("摆完之后的板", () => {
  it("标注板：同一块板还在，每一句还在她放的那一格里", () => {
    const items = boardItems(LABEL_ROLES);
    const placement = { o0: "主张", o1: "证据", o2: "限制" };
    const choice = composeBoardAnswer(LABEL_ROLES, placement, items);
    render(
      <CoachCard
        card={LABEL_ROLES}
        onAnswer={vi.fn()}
        answered={{ type: "label_roles", prompt: LABEL_ROLES.prompt, choice }}
      />,
    );
    // 五个格子都在（空的也在：板的样子没变）。
    for (const bin of LABEL_ROLES.labels!) {
      expect(screen.getByText(bin)).toBeTruthy();
    }
    for (const o of OPTIONS) {
      expect(screen.getByText(new RegExp(o.quote.slice(0, 8)))).toBeTruthy();
    }
    expect(screen.getByText("你摆的")).toBeTruthy();
    // 不能再动：一张卡片都不是按钮。
    expect(screen.queryAllByRole("button")).toHaveLength(0);
  });

  it("生词板：三个词各自还在她放的那一格里", () => {
    const items = boardItems(WORD_BANK);
    const placement = { w0: "不认识", w1: "不确定", w2: "认识" };
    const choice = composeBoardAnswer(WORD_BANK, placement, items);
    render(
      <CoachCard
        card={WORD_BANK}
        onAnswer={vi.fn()}
        answered={{ type: "word_bank", prompt: WORD_BANK.prompt, choice }}
      />,
    );
    for (const bin of ["认识", "不确定", "不认识"]) {
      expect(screen.getByText(bin)).toBeTruthy();
    }
    for (const w of WORD_BANK.words!) {
      expect(screen.getByText(w.term)).toBeTruthy();
    }
  });

  // parseBoardAnswer 读的是 composeBoardAnswer 写的那段字，而那段字就是存进
  // atom_message 的那一份。两个方向必须闭合，否则刷新回来看到的是一块空板。
  it("compose → parse 闭合", () => {
    for (const card of [LABEL_ROLES, WORD_BANK]) {
      const items = boardItems(card);
      const placement: Record<string, string> = {};
      const bins = card.type === "label_roles" ? card.labels! : ["认识", "不确定", "不认识"];
      items.forEach((it, i) => {
        placement[it.id] = bins[i % bins.length]!;
      });
      const back = parseBoardAnswer(card, composeBoardAnswer(card, placement, items));
      expect(back).toEqual(placement);
    }
  });

  // 一行都认不出来（格式变过、老数据）→ 退回显示她那段作答，而不是摆一块空板。
  // 空板读起来是「你什么都没摆」，那是假的。
  it("读不懂那段作答就原样显示它，不摆一块空板", () => {
    render(
      <CoachCard
        card={LABEL_ROLES}
        onAnswer={vi.fn()}
        answered={{ type: "label_roles", prompt: LABEL_ROLES.prompt, choice: "我按自己的想法摆好了" }}
      />,
    );
    expect(screen.getByText(/我按自己的想法摆好了/)).toBeTruthy();
  });

  it("摆完的板上也没有任何表示对错的记号", () => {
    const items = boardItems(LABEL_ROLES);
    const choice = composeBoardAnswer(LABEL_ROLES, { o0: "证据", o1: "主张", o2: "限制" }, items);
    const { container } = render(
      <CoachCard
        card={LABEL_ROLES}
        onAnswer={vi.fn()}
        answered={{ type: "label_roles", prompt: LABEL_ROLES.prompt, choice }}
      />,
    );
    const text = container.textContent ?? "";
    for (const mark of NO_MARKS) {
      expect(text.includes(mark)).toBe(false);
    }
  });
});

describe("动手部分的输入框", () => {
  const SHORT_TEXT: CoachCardSpec = {
    type: "short_text",
    prompt: "用你自己的话说说，作者漏掉了什么？",
  };

  // 产品负责人 2026-09-17：「动手部分，按回车键无法发送，需要点击发送按钮才能
  // 发送。」这个框和底下的对话输入框长得一样、挨在一起，一个认回车一个不认。
  it("回车发送", () => {
    const onAnswer = vi.fn();
    render(<CoachCard card={SHORT_TEXT} onAnswer={onAnswer} />);
    const box = screen.getByRole("textbox") as HTMLTextAreaElement;
    fireEvent.change(box, { target: { value: "他没算住在那儿的人。" } });
    fireEvent.keyDown(box, { key: "Enter" });
    expect(onAnswer).toHaveBeenCalledWith({
      type: "short_text",
      prompt: SHORT_TEXT.prompt,
      choice: "他没算住在那儿的人。",
    });
  });

  it("Shift+回车不发送（她要换行）", () => {
    const onAnswer = vi.fn();
    render(<CoachCard card={SHORT_TEXT} onAnswer={onAnswer} />);
    const box = screen.getByRole("textbox") as HTMLTextAreaElement;
    fireEvent.change(box, { target: { value: "他没算住在那儿的人。" } });
    fireEvent.keyDown(box, { key: "Enter", shiftKey: true });
    expect(onAnswer).not.toHaveBeenCalled();
  });

  // 🚨 中文输入法用回车上屏候选词。不挡的话，她打「礼貌」按回车选词，
  // 选的那一下就把半句话发出去了。
  it("输入法正在选词的时候不发送", () => {
    const onAnswer = vi.fn();
    render(<CoachCard card={SHORT_TEXT} onAnswer={onAnswer} />);
    const box = screen.getByRole("textbox") as HTMLTextAreaElement;
    fireEvent.change(box, { target: { value: "他没算住在那儿的人" } });
    fireEvent.keyDown(box, { key: "Enter", isComposing: true });
    expect(onAnswer).not.toHaveBeenCalled();
  });

  it("空白不发送", () => {
    const onAnswer = vi.fn();
    render(<CoachCard card={SHORT_TEXT} onAnswer={onAnswer} />);
    const box = screen.getByRole("textbox") as HTMLTextAreaElement;
    fireEvent.change(box, { target: { value: "   " } });
    fireEvent.keyDown(box, { key: "Enter" });
    expect(onAnswer).not.toHaveBeenCalled();
  });
});

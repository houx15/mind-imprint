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
const EVERY_SHAPE: { name: string; card: CoachCardSpec; answered?: CoachCardAnswer }[] = [
  { name: "choose_span", card: CHOOSE_SPAN },
  { name: "pick_in_article", card: PICK_IN_ARTICLE },
  { name: "short_text", card: SHORT_TEXT },
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
];

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
    fireEvent.click(screen.getByRole("button", { name: /说说看|发给印记|交给印记/ }));

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
    fireEvent.click(screen.getByRole("button", { name: /说说看|发给印记|交给印记/ }));

    expect(onAnswer).not.toHaveBeenCalled();
  });

  it("答过的卡片还留着她的选择，但已经点不动了", () => {
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

    // 她的话还在对话里
    expect(screen.getByText(new RegExp(OPTIONS[2]!.quote.slice(0, 10)))).toBeTruthy();
    // 但整张卡片不再是可操作的东西
    expect(screen.queryAllByRole("button")).toHaveLength(0);
    expect(screen.queryAllByRole("textbox")).toHaveLength(0);
    fireEvent.click(container.firstElementChild!);
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
  it.each(EVERY_SHAPE)("$name 上没有任何对/错的痕迹", ({ card, answered }) => {
    const { container } = render(<CoachCard card={card} onAnswer={vi.fn()} answered={answered} />);

    const text = container.textContent ?? "";
    for (const forbidden of ["✓", "✔", "✗", "✘", "×", "√", "正确", "错误", "答对", "答错", "分", "得分", "score", "correct", "wrong"]) {
      expect(text.includes(forbidden), `卡片上出现了「${forbidden}」`).toBe(false);
    }
    // 也不能靠 markup 偷偷说对错（aria / data / class 里的 correct|wrong|score）
    expect(/correct|wrong|score|right-answer/i.test(container.innerHTML)).toBe(false);
  });
});

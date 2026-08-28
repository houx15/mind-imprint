import { render, screen, cleanup, fireEvent, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { GuideBox } from "@lite/writings/GuideBox";
import type { WritingBlockGuide } from "@lite/api/writingRoom";

/**
 * GuideBox — pinned against the product verdict that reshaped it (Task 9):
 * *"our snippets is not real guidance, it is even not good as pro version"*
 * and *"a large paragraph of small texts is not easy to read."* These tests
 * pin the fix: four visually distinct parts, questions at reading size, and
 * a borrowed example that is unmistakably borrowed.
 */

afterEach(cleanup);

const GUIDE: WritingBlockGuide = {
  job: "这一段要让读者相信「便宜」这个说法不成立。",
  methods: [
    {
      name: "正反",
      formalName: "对比论证",
      definition: "一正一反两个例子放在一起。",
      examples: [{ topic: "两个菜市场", text: "东街留了装卸区…" }],
      patterns: [],
    },
  ],
  questions: ["你见过哪条街上的树长不开？", "这跟成本有什么关系？"],
};

describe("GuideBox", () => {
  it("renders all four parts, and questions at reading size", () => {
    render(<GuideBox guide={GUIDE} onDismiss={() => {}} onDeepen={() => {}} />);

    expect(screen.getByText(/这一段要做的事/)).toBeTruthy();
    expect(screen.getByText(/常见的几种写法/)).toBeTruthy();
    expect(screen.getByText("正反")).toBeTruthy();
    expect(screen.getByText(/一正一反两个例子/)).toBeTruthy();
    expect(screen.getByText(/想一想/)).toBeTruthy();
    expect(screen.getByRole("button", { name: /看几个例子/ })).toBeTruthy();
    expect(screen.getByRole("button", { name: /深入一层/ })).toBeTruthy();

    // The old box put everything at mk-body 14px, which is what made it unreadable.
    expect(screen.getByText("你见过哪条街上的树长不开？").className).toContain("text-mk-body-lg");
    expect(screen.getByText("这跟成本有什么关系？").className).toContain("text-mk-body-lg");
  });

  it("keeps 收起 wired to onDismiss", () => {
    const onDismiss = vi.fn();
    render(<GuideBox guide={GUIDE} onDismiss={onDismiss} onDeepen={() => {}} />);
    fireEvent.click(screen.getByRole("button", { name: "收起" }));
    expect(onDismiss).toHaveBeenCalledOnce();
  });

  it("wires 深入一层 to onDeepen", () => {
    const onDeepen = vi.fn();
    render(<GuideBox guide={GUIDE} onDismiss={() => {}} onDeepen={onDeepen} />);
    fireEvent.click(screen.getByRole("button", { name: /深入一层/ }));
    expect(onDeepen).toHaveBeenCalledOnce();
  });

  it("shows the borrowed example only after she asks, with its topic named", () => {
    render(<GuideBox guide={GUIDE} onDismiss={() => {}} onDeepen={() => {}} />);

    // Not painted open by default — a borrowed example next to a live
    // question list would read as a suggestion unless she asked for it.
    expect(screen.queryByText(/东街留了装卸区/)).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: /看几个例子/ }));

    expect(screen.getByText(/东街留了装卸区/)).toBeTruthy();
    // The topic MUST be named beside the example — that is the whole
    // guarantee: an example about 两个菜市场 can never be mistaken for a
    // suggestion about her own piece.
    expect(screen.getByText(/两个菜市场/)).toBeTruthy();
  });

  it("does not render a method picker — no radio inputs, no selectable role for a method", () => {
    render(<GuideBox guide={GUIDE} onDismiss={() => {}} onDeepen={() => {}} />);
    expect(screen.queryAllByRole("radio")).toHaveLength(0);
    expect(screen.queryAllByRole("checkbox")).toHaveLength(0);
  });

  it("shows a method's sentence patterns, which is all an English method carries", () => {
    // The English half of vocab's library (en_concession / en_qualify /
    // en_evidence) ships FRAMES rather than worked examples. Gating 例子 on
    // `examples` alone left an English block with a button that never
    // appeared and teaching that never reached her.
    const english: WritingBlockGuide = {
      job: "Concede the strongest objection before answering it.",
      methods: [
        {
          name: "Conceding, then turning",
          formalName: "Concession",
          definition: "Grant what is true, then say what it does not settle.",
          examples: [],
          patterns: [{ label: "Admit then limit", frame: "While it is true that ___, this does not mean ___." }],
        },
      ],
      questions: ["What is the strongest thing someone could say against you?"],
    };
    render(<GuideBox guide={english} onDismiss={() => {}} onDeepen={() => {}} />);

    fireEvent.click(screen.getByRole("button", { name: /看几个例子/ }));
    expect(screen.getByText(/While it is true that ___/)).toBeTruthy();
    // The blanks are the 铁律① line: the frame says what SHAPE the sentence
    // takes, and the part it will not write is exactly the part she fills in.
    expect(screen.getByText(/横线上的内容要你自己填/)).toBeTruthy();
  });

  it("omits parts that have nothing to show, rather than rendering an empty section", () => {
    const sparse: WritingBlockGuide = { job: "", methods: [], questions: ["只有一个问题？"] };
    render(<GuideBox guide={sparse} onDismiss={() => {}} onDeepen={() => {}} />);
    expect(screen.queryByText(/这一段要做的事/)).toBeNull();
    expect(screen.queryByText(/常见的几种写法/)).toBeNull();
    expect(screen.queryByRole("button", { name: /看几个例子/ })).toBeNull();
    expect(screen.getByText("只有一个问题？")).toBeTruthy();
  });

  /**
   * The method card (F3, 2026-08-28; widened same day). Every method's name
   * is tappable and opens a card explaining "what is this, really" —
   * definition + borrowed example, always. `formalName` only gates ONE
   * sentence inside the card (the curriculum term), not the affordance
   * itself: `name` ("正反") stays what she reads by default, and the 语文课
   * term ("对比论证") only surfaces as an extra sentence if she taps for it.
   */
  describe("the method card", () => {
    it("opens on tapping the method's name and reveals the formal term", () => {
      render(<GuideBox guide={GUIDE} onDismiss={() => {}} onDeepen={() => {}} />);

      // Not shown before she asks.
      expect(screen.queryByText(/对比论证/)).toBeNull();

      fireEvent.click(screen.getByRole("button", { name: /正反/ }));

      const term = screen.getByText(/对比论证/);
      expect(term).toBeTruthy();
      // The explanation rides along in the same paragraph, at reading size —
      // this is the room's legibility standard, not an 11px chip.
      expect(term.closest("p")?.className).toContain("text-mk-body-lg");
    });

    it("is dismissible — re-tap, 收起 button, and Escape", () => {
      render(<GuideBox guide={GUIDE} onDismiss={() => {}} onDeepen={() => {}} />);
      const nameButton = screen.getByRole("button", { name: /正反/ });

      fireEvent.click(nameButton);
      expect(screen.getByRole("dialog")).toBeTruthy();
      fireEvent.click(nameButton);
      expect(screen.queryByRole("dialog")).toBeNull();

      fireEvent.click(nameButton);
      expect(screen.getByRole("dialog")).toBeTruthy();
      fireEvent.click(screen.getByRole("button", { name: "知道了" }));
      expect(screen.queryByRole("dialog")).toBeNull();

      fireEvent.click(nameButton);
      expect(screen.getByRole("dialog")).toBeTruthy();
      fireEvent.keyDown(window, { key: "Escape" });
      expect(screen.queryByRole("dialog")).toBeNull();
    });

    it("shows the borrowed example already in the library, inside the card", () => {
      render(<GuideBox guide={GUIDE} onDismiss={() => {}} onDeepen={() => {}} />);
      fireEvent.click(screen.getByRole("button", { name: /正反/ }));
      expect(screen.getByText(/东街留了装卸区/)).toBeTruthy();
      expect(screen.getByText(/两个菜市场/)).toBeTruthy();
    });

    it("opens the card even when formalName equals name, but omits the 语文课 line", () => {
      // 开门见山: the one pair in vocab's library where the plain name IS the
      // curriculum term. There is nothing left to NAME, but "what is this
      // method" is still worth explaining — the card opens, the definition
      // and example still show, only the term sentence is absent.
      const known: WritingBlockGuide = {
        job: "",
        methods: [
          {
            name: "开门见山",
            formalName: "开门见山",
            definition: "第一句就把结论说出来，后面全部用来支撑它。",
            examples: [{ topic: "校车安全", text: "校车该不该装安全带，其实早有答案：该装。" }],
            patterns: [],
          },
        ],
        questions: [],
      };
      render(<GuideBox guide={known} onDismiss={() => {}} onDeepen={() => {}} />);

      fireEvent.click(screen.getByRole("button", { name: /开门见山/ }));

      const dialog = screen.getByRole("dialog");
      // The definition also appears in the collapsed list row above (it's
      // always shown there too), so these assertions are scoped to the card
      // itself rather than the whole document.
      expect(within(dialog).getByText(/第一句就把结论说出来/)).toBeTruthy();
      expect(within(dialog).getByText(/校车该不该装安全带/)).toBeTruthy();
      // The one thing that must NOT appear: a "在语文课上，这个叫" sentence
      // naming a term identical to what she already read.
      expect(within(dialog).queryByText(/在语文课上/)).toBeNull();
    });

    it("opens the card even when formalName is empty, but omits the 语文课 line", () => {
      // 最后提个建议 has no curriculum term at all — and is exactly the kind
      // of vague-sounding name a student would want explained. Gating the
      // whole card on formalName would have hidden the explanation from
      // precisely the method that needed it most.
      const noTerm: WritingBlockGuide = {
        job: "",
        methods: [
          {
            name: "最后提个建议",
            formalName: "",
            definition: "结尾给读者一个具体能做的事，而不是空喊一句口号。",
            examples: [{ topic: "旧书摊", text: "如果你也常去，不妨周末去看看还剩多少本。" }],
            patterns: [],
          },
        ],
        questions: [],
      };
      render(<GuideBox guide={noTerm} onDismiss={() => {}} onDeepen={() => {}} />);

      fireEvent.click(screen.getByRole("button", { name: /最后提个建议/ }));

      const dialog = screen.getByRole("dialog");
      expect(within(dialog).getByText(/结尾给读者一个具体能做的事/)).toBeTruthy();
      expect(within(dialog).getByText(/不妨周末去看看还剩多少本/)).toBeTruthy();
      expect(within(dialog).queryByText(/在语文课上/)).toBeNull();
    });

    it("is an explainer, not a picker — no selectable control anywhere once open", () => {
      render(<GuideBox guide={GUIDE} onDismiss={() => {}} onDeepen={() => {}} />);
      fireEvent.click(screen.getByRole("button", { name: /正反/ }));
      expect(screen.getByRole("dialog")).toBeTruthy();
      expect(screen.queryAllByRole("radio")).toHaveLength(0);
      expect(screen.queryAllByRole("checkbox")).toHaveLength(0);
    });
  });
});

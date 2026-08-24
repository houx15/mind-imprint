import type { ReadingOutcome } from "@/studio/reading/readingLoop";

/**
 * The guided-tour demo project's 阅读成果 seed (P8).
 *
 * One already-CONFIRMED reading finding, so the demo ReadingRoom's 阅读成果
 * tab shows a real, complete result — the student's own selected sentence +
 * the finding she reached + the full lens review — instead of the empty
 * "完成并保存一次透镜练习后……" placeholder. Threaded into `useReadingLoop`
 * via `initialOutcomes` (demo-only); its span also stays highlighted in the
 * article (see the `spans` memo in ReadingRoom.tsx).
 *
 * It sits on block b8 (China-top-emitter), which carries NO seeded AI anchor
 * (0086 anchored b3/b4/b6) — so it never collides with the three highlighted
 * sentences the tour clicks to reveal the in-line lens card.
 *
 * 铁律① (AI never writes the student's essay): this is the student's own
 * analytical reading finding about a SOURCE — not a sentence of her essay.
 * The lens review only judges how well her selected sentence supports her
 * judgment; it never drafts body text.
 */
export const DEMO_READING_OUTCOMES: ReadingOutcome[] = [
  {
    id: "demo-outcome-1",
    cardId: "craap",
    cardName: "CRAAP 来源体检",
    blockId: "b8",
    start: 94,
    end: 277,
    quote:
      "China is the world’s top annual emitter of carbon dioxide, a position it has occupied since the mid-2000s, and its yearly emissions still make up close to a third of the global total.",
    finding:
      "这条「中国碳排放全球第一、约占全球三分之一」是强数据，它提醒我：论文证明的「变绿」不能单独用来证明「更可持续」——趋势变好和问题解决要分开看。",
    judgment:
      "可以放心引用这条排放数字，但要把它和「变绿」放在一起，作为对总论点的限定，而不是让任何一条数字盖过另一条。",
    support:
      "这是长期稳定的宏观统计（自 2000 年代中期至今），来源口径清楚（年度 CO₂ 排放占全球比重），和变绿数据不冲突——一个讲土地利用，一个讲能源结构。",
    caveat:
      "排放「总量」第一不等于「人均」或「历史累计」第一，正式写作时要说清用的是哪个口径，否则容易被反驳。",
    eval: {
      verdict: "strong",
      verdictLabel: "选句站得住",
      verdictReason:
        "你选的是一条能countervailing（对冲）主叙事的关键统计，并且用它来给判断加限定、而不是过度外推——这正是溯源体检该做的。",
      checks: [
        {
          key: "authority",
          label: "权威性",
          status: "pass",
          evidence: "长期稳定的国家级排放统计",
          explanation: "这是被反复独立证实的宏观数据，不是单一二手转述。",
        },
        {
          key: "accuracy",
          label: "口径准确",
          status: "partial",
          evidence: "「总量」占三分之一",
          explanation: "说的是年度排放总量占比，没区分人均与历史累计——引用时要标明口径。",
        },
        {
          key: "relevance",
          label: "相关性",
          status: "pass",
          evidence: "直接对冲「变绿=可持续」",
          explanation: "这条正好回应了总问题里最容易被忽略的反面，放在论证里很有用。",
        },
      ],
      finding:
        "「中国碳排放全球第一、约占三分之一」是强数据，用来给「变绿」判断加限定，而不是推翻它。",
      judgment: "引用这条排放数字来限定总论点，与变绿数据并置。",
      support: "长期稳定的宏观统计，口径清楚，与变绿数据不冲突。",
      caveat: "总量口径 ≠ 人均/历史累计，需说清。",
      nextStep: "把这条排放数字和 b4 的「变绿成因」一起用，构成「趋势在变好、但问题没解决」的让步段。",
      spanIds: ["demo-outcome-1"],
    },
  },
];

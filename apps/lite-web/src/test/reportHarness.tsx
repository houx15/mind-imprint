// THROWAWAY harness — renders ReportView with a realistic fixture so the
// current report can be looked at. Not part of the app; delete before merge.
import { createRoot } from "react-dom/client";
import "../index.css";
import "../learning/student-surfaces.css";
import { ReportView } from "../reports/ReportView";
import type { LiteReport } from "../api/reports";

const reading: LiteReport = {
  version: 1,
  kind: "reading",
  title: "中国的太阳能扩张正在改写全球排放曲线",
  studentName: "Phoebe",
  finishedAt: "2026-09-14T09:12:00Z",
  stats: [
    { key: "focusMinutes", label: "阅读时长", value: 34, unit: "分钟" },
    { key: "wordsRead", label: "读了", value: 2180, unit: "字" },
    { key: "chatTurns", label: "AI 对话轮数", value: 12, unit: "" },
    { key: "highlights", label: "划线", value: 7, unit: "处" },
    { key: "notes", label: "笔记", value: 4, unit: "条" },
    { key: "lenses", label: "用了透镜", value: 2, unit: "个" },
    { key: "stepsDone", label: "阅读任务完成数", value: 5, unit: "" },
  ],
  moments: [
    {
      quote: "如果装机量第一，但人均排放还在涨，那说明扩张的速度还没追上需求的速度。",
      where: "我在对话里说的",
    },
    {
      quote: "我本来以为“全球第一”就等于“做得最好”，读完才发现这是两件事。",
      where: "我的收获",
    },
  ],
  keep: {
    label: "我的收获",
    text: "判断一个国家在气候问题上做得怎么样，不能只看它建了多少太阳能板，还要看排放总量和人均排放的走向。这三个数字讲的是三件事。",
    source: "coach",
  },
  gains: ["学会了把“装机量”和“发电量”分开看", "知道了要去查数据的原始出处"],
  lensNotes: [
    {
      lens: "溯源透镜",
      quote:
        "According to the International Energy Agency, China added more solar capacity in 2025 than the rest of the world combined.",
      finding: "这句话的出处是 IEA 的年度报告，不是记者自己的推算。",
    },
  ],
  notes: [
    {
      quote: "Coal still generated more than half of the country's electricity last year.",
      note: "这里和前面说的“全球第一”有冲突，要放在一起看。",
    },
  ],
  piece: "",
  prosePending: false,
};

createRoot(document.getElementById("root")!).render(
  <div style={{ background: "var(--mk-paper)", minHeight: "100vh" }}>
    <ReportView report={reading} />
  </div>,
);

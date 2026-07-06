// SLICE-1 MOCK FIXTURE — throwaway. Replaced by real backend course data in Slice 4.
// Do not treat as a source of truth; it exists only to exercise the Courses grid UI.

export type CourseToneKind = "not_started" | "in_progress" | "done";

export interface MockCourse {
  id: string;
  branch: string; // pillar label shown as the figure chip, e.g. "批判性思维"
  title: string;
  blurb: string;
  meta: string; // "3 个任务 · 4 个工具"
  time: string; // "约 40 分钟"
  progressPct: number | null; // null → hide the progress bar
  tone: string; // "未开始" | "进行中" | "已学完"
  toneKind: CourseToneKind;
  ctaLabel: string; // "开始学习" | "继续" | "回顾"
}

export const MOCK_COURSES: MockCourse[] = [
  {
    id: "c-info-trust",
    branch: "批判性思维",
    title: "一条网络信息，该不该信",
    blurb:
      "从一句「卫星图显示中国让地球变绿」出发，跟着印记学会横向溯源、辨识来源、拆穿断言——把「随手一信」变成「查过再信」。",
    meta: "3 个任务 · 4 个工具",
    time: "约 40 分钟",
    progressPct: null,
    tone: "未开始",
    toneKind: "not_started",
    ctaLabel: "开始学习",
  },
];

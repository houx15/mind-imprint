import type { CourseReport } from "@mind-imprint/contracts";

/** A frozen, illustrative course report shown during onboarding — NOT a real attempt.
 *  Content is substantive on purpose (see spec §5.2 quality bar).
 *  cardIds are real ids from packages/contracts/cards/ (verified against the
 *  registry): fact-opinion-value.json (事实/观点/价值判断卡) and toulmin.json
 *  (论证构建卡·图尔敏) — chosen because they match this course's own
 *  completed-step titles below (区分事实判断与价值判断 / 用反例检验你的界定). */
export const exampleCourseReport: CourseReport = {
  title: "追问“可持续”：一堂关于概念澄清的思辨课",
  goal: "学会在下判断前，先把关键概念拆开、界定清楚，避免用模糊的大词代替真正的论证。",
  teaching_thread:
    "这门课带你走过一次完整的概念澄清：先识别一句话里被当作理所当然的大词，" +
    "再追问它到底指什么、由谁来衡量、在什么范围内成立，最后用一个反例检验你的界定是否站得住。",
  completedStepTitles: [
    "识别被含糊使用的关键概念",
    "为概念给出可操作的界定",
    "区分事实判断与价值判断",
    "用反例检验你的界定",
    "把澄清后的概念放回原来的问题",
  ],
  cardIds: ["fact-opinion-value", "toulmin"],
  secondsSpent: 1140,
  quiz: { total: 5, correct: 4 },
};

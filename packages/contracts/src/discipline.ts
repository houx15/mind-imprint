import { z } from "zod";
import disciplinesJson from "../disciplines/disciplines.json";

/**
 * discipline —— 学科表的契约。自上而下的那一张图。
 *
 * 学生的兴趣关键词是自下而上长的（她读过、写过、做过什么）；这里是另一半：
 * 人类已经分好的学科。两者之间的路由在服务端（`apps/api/internal/interest`）。
 *
 * ## 学科为骨，课程为投影
 *
 * `syllabus` 是一个**可以为空**的投影层：IB / A-Level / AP / IGCSE 的考纲编号挂
 * 在学科上，而不是反过来。这样考试局改一次大纲改的是一个数组，学生的关键词
 * 不受影响——因为它当初连的是 `"statistical-inference"`，从来不是 `"MAA-4"`。
 * 读 A-Level 的和读 IB 的学生共用同一张图，只是投影不同。
 *
 * 空的 `syllabus` 合法，它表示「这门学科高中不教」——那是真话，而且对一个讲
 * 探索的产品来说是有吸引力的真话。
 *
 * ## 真相源
 *
 * `packages/contracts/disciplines/disciplines.json` 是唯一可编辑的那一份。
 * Go 端在 `apps/api/internal/disciplines/` 放了一份逐字节副本（`go:embed` 的
 * 模式出不了包目录），由 `TestEmbeddedCopyMatchesSourceOfTruth` 守着不漂移。
 * 改这份 JSON 之后必须把它 `cp` 到 Go 那一侧，否则那条测试会红。
 */

/** 树的七根主枝，也是 `Discipline.field` 的取值。 */
export const FIELD_IDS = [
  "formal",
  "science",
  "making",
  "society",
  "humanities",
  "arts",
  "self",
] as const;

export const FieldIdSchema = z.enum(FIELD_IDS);
export type FieldId = z.infer<typeof FieldIdSchema>;

export const FIELD_LABELS: Record<FieldId, string> = {
  formal: "数学与形式",
  science: "科学与自然",
  making: "技术与创造",
  society: "社会与世界",
  humanities: "人文与写作",
  arts: "艺术与表达",
  self: "自我与成长",
};

/** 考试体系。`University` 是兜底的那一档：高中不教的学科也有大学方向。 */
export const BoardSchema = z.enum(["IB", "ALevel", "AP", "IGCSE", "University"]);
export type Board = z.infer<typeof BoardSchema>;

export const SyllabusRefSchema = z.object({
  board: BoardSchema,
  code: z.string().min(1),
  label: z.string().min(1),
  /** HL/SL · AS/A2。大学方向没有这一栏。 */
  level: z.string().optional(),
});
export type SyllabusRef = z.infer<typeof SyllabusRefSchema>;

export const DisciplineSchema = z.object({
  id: z.string().min(1),
  field: FieldIdSchema,
  zh: z.string().min(1),
  en: z.string().min(1),
  /** 它研究什么——写成一句问题，不是定义。Go 端有测试强制它以问号结尾。 */
  asks: z.string().min(1),
  /** 它的核心方法——学生能拿去用的那几个词。 */
  method: z.string().min(1),
  /** 一个典型问题——具体到能想象出画面。 */
  exemplar: z.string().min(1),
  /** 路由 T1 档（免费那一档）的命中面。学科自己的中英文名不必重复写进来。 */
  aliases: z.array(z.string().min(1)).min(4),
  syllabus: z.array(SyllabusRefSchema),
});
export type Discipline = z.infer<typeof DisciplineSchema>;

/** 全部 42 门，构建期就已校验过形状。 */
export const DISCIPLINES: Discipline[] = z
  .array(DisciplineSchema)
  .parse(disciplinesJson);

const BY_ID = new Map(DISCIPLINES.map((d) => [d.id, d]));

export function disciplineById(id: string): Discipline | undefined {
  return BY_ID.get(id);
}

export function disciplinesByField(field: FieldId): Discipline[] {
  return DISCIPLINES.filter((d) => d.field === field);
}

/**
 * 一门学科在她自己的课程体系里的落点，排在最前面。
 *
 * 学科卡先给她看自己那一行，其余折到「其他课程体系」下面——同一张图，
 * 不同投影。她的体系没有覆盖这门学科时，返回的第一项就是别人的，
 * 界面据此说「你的课程体系里没有这一门」，而不是假装有。
 */
export function syllabusForBoard(d: Discipline, board: Board): {
  mine: SyllabusRef[];
  others: SyllabusRef[];
} {
  return {
    mine: d.syllabus.filter((s) => s.board === board),
    others: d.syllabus.filter((s) => s.board !== board),
  };
}

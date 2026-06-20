export const RUBRIC_TAGS = [
  "D1_来源意识",
  "D2_交叉验证",
  "D5_论证结构",
  "D7_对立观点处理",
] as const;

export type RubricTag = (typeof RUBRIC_TAGS)[number];

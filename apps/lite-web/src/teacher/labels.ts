// teacher/labels.ts — the single copy of 类型/来源 option tables shown across
// the teacher assignment UI. Pinned on the Go side by
// `apps/api/internal/liteworkspace/labels_test.go`
// (`TestLabelsMatchTheWebTables`), which reads this file as text — a label
// changed here without the Go table changing to match fails that test.
//
// Deliberately import-free: `AssignmentForm.tsx` and `AssignmentAIMode.tsx`
// already form an import cycle (AssignmentForm renders AssignmentAIMode,
// AssignmentAIMode imports KindField/SettingsFields back from
// AssignmentForm), so this file must not pull in anything from either side
// of that cycle — it only needs the two plain string-union types.

import type { AssignmentKind } from "../api/assignments";
import type { ReadingSource } from "./assignmentLogic";

export const KIND_OPTIONS: { value: AssignmentKind; label: string }[] = [
  { value: "reading", label: "阅读" },
  { value: "writing", label: "写作" },
  { value: "project", label: "项目" },
];

export const SOURCE_OPTIONS: { value: ReadingSource; label: string }[] = [
  { value: "library", label: "分级阅读库" },
  { value: "url", label: "链接" },
  { value: "text", label: "正文" },
  { value: "file", label: "上传文件" },
  { value: "personalized", label: "个性化" },
];

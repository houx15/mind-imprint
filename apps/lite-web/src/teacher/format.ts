// teacher/format.ts — pure formatting helpers for the lite teacher end.
// No UI here: these turn raw roster/item numbers into the labels the
// teacher-facing screens show. States use 已/待/中 per the project's copy
// rules, not folksy prose.

export function formatMinutes(n: number): string {
  if (n < 0) return "—";
  if (n < 60) return `${n} 分钟`;
  const h = Math.floor(n / 60);
  const m = n % 60;
  return m === 0 ? `${h} 小时` : `${h} 小时 ${m} 分钟`;
}

const STATUS: Record<string, Record<string, string>> = {
  reading: { active: "进行中", finished: "已完成" },
  writing: { active: "进行中", finished: "已完成" },
  project: { talking: "立项中", running: "进行中", review: "回顾中", keeping: "已完成", archived: "已归档" },
};

export function itemStatusLabel(kind: string, status: string): string {
  return STATUS[kind]?.[status] ?? status;
}

export function kindLabel(kind: string): string {
  return ({ reading: "阅读", writing: "写作", project: "项目" } as Record<string, string>)[kind] ?? kind;
}

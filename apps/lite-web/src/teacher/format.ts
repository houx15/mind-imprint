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

/** writing.lang is a code (`zh`/`en`); the teacher reads a language name. */
export function langLabel(lang: string | null | undefined): string {
  if (!lang) return "—";
  return ({ zh: "中文", en: "英文" } as Record<string, string>)[lang] ?? lang;
}

/**
 * The URL a student pasted for her reading, if it is safe to render as a
 * link on a teacher screen. React does not block `javascript:` hrefs, and the
 * URL is student-controlled, so only an absolute http/https URL comes back;
 * anything else (other schemes, relative paths, unparsable text) is `null`
 * and the caller renders the text plainly.
 */
export function safeHttpUrl(url: string | null | undefined): string | null {
  if (!url) return null;
  try {
    const { protocol } = new URL(url);
    return protocol === "http:" || protocol === "https:" ? url : null;
  } catch {
    return null;
  }
}

export function kindLabel(kind: string): string {
  return ({ reading: "阅读", writing: "写作", project: "项目" } as Record<string, string>)[kind] ?? kind;
}

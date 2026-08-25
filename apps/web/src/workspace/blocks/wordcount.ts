// wordcount.ts — the editor's live "字" counter, mirroring the Go backend's
// canonical word count (apps/api/internal/agent/wordcount.go · agent.CountWords)
// so the live counter agrees with the 整稿体检 review header ("… 字 · 字数偏离区间")
// and the word-budget band. CJK-aware: each CJK char counts as one, and each
// run of non-space, non-CJK characters (a Latin/number word) counts as one.
//
// The old `text.replace(/\s+/g, "").length` counted CHARACTERS: fine for
// Chinese (~one char per word) but wildly wrong for English/双语 — a ~165-word
// English draft read "986 字", telling a writer aiming at an "约 800 词" target
// they were over when they were ~20% there. See the 2026-08-25 projects e2e
// bug findings (BUG-02).

function isCJK(cp: number): boolean {
  return (
    (cp >= 0x4e00 && cp <= 0x9fff) || // CJK Unified Ideographs
    (cp >= 0x3400 && cp <= 0x4dbf) || // CJK Extension A
    (cp >= 0x20000 && cp <= 0x2a6df) || // CJK Extension B
    (cp >= 0x3040 && cp <= 0x30ff) || // hiragana + katakana
    (cp >= 0xff00 && cp <= 0xffef) // full-width forms
  );
}

export function countWords(s: string): number {
  let count = 0;
  let inRun = false;
  for (const ch of s) {
    const cp = ch.codePointAt(0);
    if (cp !== undefined && isCJK(cp)) {
      count++;
      inRun = false;
    } else if (/\s/.test(ch)) {
      inRun = false;
    } else if (!inRun) {
      count++;
      inRun = true;
    }
  }
  return count;
}

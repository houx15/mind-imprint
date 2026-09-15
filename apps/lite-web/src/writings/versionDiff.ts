import { splitParagraphs } from "../reports/paragraphs";

/**
 * Version diff for the finished-writing page (与当前版本对比).
 *
 * Two levels: a longest-common-subsequence over paragraphs, then, inside each
 * replaced paragraph, a longest-common-subsequence over characters. A run of
 * deleted paragraphs followed by added ones is paired in order; unpaired
 * paragraphs stay whole additions or deletions.
 */

export type DiffPart = { kind: "same" | "add" | "del"; text: string };

export type ParagraphDiff = { kind: "same" | "add" | "del"; text: string } | { kind: "change"; parts: DiffPart[] };

type Op<T> = { op: "same" | "add" | "del"; value: T };

/** Above this many table cells the character diff is skipped: 500 × 500. */
const MAX_CELLS = 250_000;

function lcsOps<T>(a: readonly T[], b: readonly T[]): Op<T>[] {
  const n = a.length;
  const m = b.length;
  const width = m + 1;
  const dp = new Uint32Array((n + 1) * width);
  const at = (i: number, j: number): number => dp[i * width + j] ?? 0;
  for (let i = n - 1; i >= 0; i--) {
    for (let j = m - 1; j >= 0; j--) {
      dp[i * width + j] = a[i] === b[j] ? at(i + 1, j + 1) + 1 : Math.max(at(i + 1, j), at(i, j + 1));
    }
  }
  const out: Op<T>[] = [];
  let i = 0;
  let j = 0;
  while (i < n && j < m) {
    if (a[i] === b[j]) {
      out.push({ op: "same", value: a[i] as T });
      i++;
      j++;
    } else if (at(i + 1, j) >= at(i, j + 1)) {
      // Deletions first, so a replaced run reads as del… then add….
      out.push({ op: "del", value: a[i] as T });
      i++;
    } else {
      out.push({ op: "add", value: b[j] as T });
      j++;
    }
  }
  while (i < n) out.push({ op: "del", value: a[i++] as T });
  while (j < m) out.push({ op: "add", value: b[j++] as T });
  return out;
}

/** Character-level marks inside one paragraph, adjacent parts merged. */
export function diffChars(older: string, newer: string): DiffPart[] {
  const a = Array.from(older);
  const b = Array.from(newer);
  if (a.length * b.length > MAX_CELLS) {
    const parts: DiffPart[] = [];
    if (older) parts.push({ kind: "del", text: older });
    if (newer) parts.push({ kind: "add", text: newer });
    return parts;
  }
  const parts: DiffPart[] = [];
  for (const { op, value } of lcsOps(a, b)) {
    const last = parts[parts.length - 1];
    if (last && last.kind === op) last.text += value;
    else parts.push({ kind: op, text: value });
  }
  return parts;
}

export function diffVersions(older: string, newer: string): ParagraphDiff[] {
  const ops = lcsOps(splitParagraphs(older), splitParagraphs(newer));
  const out: ParagraphDiff[] = [];
  let dels: string[] = [];
  let adds: string[] = [];
  const flush = () => {
    const paired = Math.min(dels.length, adds.length);
    for (let k = 0; k < paired; k++) out.push({ kind: "change", parts: diffChars(dels[k] ?? "", adds[k] ?? "") });
    for (const text of dels.slice(paired)) out.push({ kind: "del", text });
    for (const text of adds.slice(paired)) out.push({ kind: "add", text });
    dels = [];
    adds = [];
  };
  for (const { op, value } of ops) {
    if (op === "same") {
      flush();
      out.push({ kind: "same", text: value });
    } else if (op === "del") {
      dels.push(value);
    } else {
      adds.push(value);
    }
  }
  flush();
  return out;
}

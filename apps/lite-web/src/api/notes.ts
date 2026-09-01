import { apiFetch } from "./client";

// api/notes.ts —— 便签板。形状读自 apps/api/internal/api/pbl_board.go。

const base = (id: string) => `/api/v1/pbl/projects/${id}`;

export type NoteKind = "observation" | "quote" | "assumption" | "question" | "idea";

export interface Note {
  id: string;
  kind: NoteKind;
  body: string;
  /** 谁写的。印记写的便签她留下了，和她自己写的，不是一回事。 */
  author: "student" | "yinji";
  /** 她改过印记写的这张。纠正是最强的过程信号之一。 */
  edited: boolean;
  cluster: string;
  x: number;
  y: number;
  createdAt: string;
}

export interface NoteKindMeta {
  kind: NoteKind;
  label: string;
  hue: string;
}

/** 界面上的说法。四种是她带回来的材料，办法是「想办法」时用的。 */
const OBSERVATION: NoteKindMeta = { kind: "observation", label: "观察结论", hue: "#3B82F6" };

export const NOTE_KINDS: NoteKindMeta[] = [
  OBSERVATION,
  { kind: "quote", label: "别人说的", hue: "#8B5CF6" },
  { kind: "assumption", label: "我猜的", hue: "#F59E0B" },
  { kind: "question", label: "想问的", hue: "#EF4444" },
  { kind: "idea", label: "办法", hue: "#10B981" },
];

export function noteKindMeta(kind: NoteKind): NoteKindMeta {
  return NOTE_KINDS.find((k) => k.kind === kind) ?? OBSERVATION;
}

export function listNotes(projectId: string): Promise<Note[]> {
  return apiFetch<Note[]>(`${base(projectId)}/notes`);
}

export function createNotes(
  projectId: string,
  notes: { kind: NoteKind; body: string; author?: "student" | "yinji"; cluster?: string }[],
): Promise<Note[]> {
  return apiFetch<Note[]>(`${base(projectId)}/notes`, {
    method: "POST",
    body: JSON.stringify({ notes }),
  });
}

export function updateNote(
  projectId: string,
  noteId: string,
  patch: { body?: string; kind?: NoteKind; cluster?: string },
): Promise<Note> {
  return apiFetch<Note>(`${base(projectId)}/notes/${noteId}`, {
    method: "PATCH",
    body: JSON.stringify(patch),
  });
}

export function archiveNote(projectId: string, noteId: string): Promise<Note> {
  return apiFetch<Note>(`${base(projectId)}/notes/${noteId}`, { method: "DELETE" });
}

/**
 * 按堆分组，未归类的排在最前。
 *
 * 把哪些放一起，就是从一堆零散东西里看出线索的那一步——所以分组是这块板上
 * 唯一真正要她动脑的操作，未归类的那一列要一直显眼地在最前面。
 */
export function groupByCluster(notes: Note[]): { cluster: string; notes: Note[] }[] {
  const groups = new Map<string, Note[]>();
  for (const n of notes) {
    const key = n.cluster.trim();
    const list = groups.get(key);
    if (list) list.push(n);
    else groups.set(key, [n]);
  }
  const named = [...groups.entries()]
    .filter(([c]) => c !== "")
    .sort((a, b) => a[0].localeCompare(b[0], "zh"))
    .map(([cluster, notes]) => ({ cluster, notes }));
  return [{ cluster: "", notes: groups.get("") ?? [] }, ...named];
}

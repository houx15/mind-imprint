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

/**
 * 界面上的说法。四种是她带回来的材料，办法是「想办法」时用的。
 *
 * 🚨 标签必须和 kind 说的是同一件事。2026-09-02 线上实测：`observation` 挂着
 * 「观察结论」、`quote` 挂着「实际观察」——两个最要紧的类型是错位的。她把数出
 * 来的人数存成了 quote，把同桌的原话存成了 observation，回灌给印记的那句话于是
 * 变成「她记下的别人的原话：走廊上我数了 23 个人」。而且整套标签里根本没有
 * 「原话」这一档，她想记下同学真说了什么就无处可放。
 */
const OBSERVATION: NoteKindMeta = { kind: "observation", label: "实际观察", hue: "#3B82F6" };

export const NOTE_KINDS: NoteKindMeta[] = [
  OBSERVATION,
  { kind: "quote", label: "别人的原话", hue: "#8B5CF6" },
  { kind: "assumption", label: "我的推论", hue: "#F59E0B" },
  { kind: "question", label: "我的问题", hue: "#EF4444" },
  { kind: "idea", label: "解决方案", hue: "#10B981" },
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

/**
 * 把选中的几张归成一堆。
 *
 * 一个端点，不是循环调 PATCH：归堆是一次决定（"这几张是一回事"），不是三次
 * 各自独立的修改。传空字符串就是把它们从堆里拿出来。
 */
export function clusterNotes(
  projectId: string,
  ids: string[],
  cluster: string,
): Promise<Note[]> {
  return apiFetch<Note[]>(`${base(projectId)}/notes/cluster`, {
    method: "POST",
    body: JSON.stringify({ ids, cluster }),
  });
}

/** 挪到板上的某个位置。 */
export function moveNote(projectId: string, noteId: string, x: number, y: number): Promise<Note> {
  return apiFetch<Note>(`${base(projectId)}/notes/${noteId}`, {
    method: "PATCH",
    body: JSON.stringify({ x, y }),
  });
}

export function archiveNote(projectId: string, noteId: string): Promise<Note> {
  return apiFetch<Note>(`${base(projectId)}/notes/${noteId}`, { method: "DELETE" });
}

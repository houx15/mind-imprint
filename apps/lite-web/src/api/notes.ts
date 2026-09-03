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
  /** 她带回来的那张照片在 OSS 里的 key，空 = 没有照片。 */
  imageKey: string;
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
const OBSERVATION: NoteKindMeta = { kind: "observation", label: "实际观察", hue: "var(--mk-mist)" };

export const NOTE_KINDS: NoteKindMeta[] = [
  OBSERVATION,
  { kind: "quote", label: "别人的原话", hue: "var(--mk-taro)" },
  { kind: "assumption", label: "我的推论", hue: "var(--mk-peach)" },
  { kind: "question", label: "我的问题", hue: "var(--mk-berry)" },
  { kind: "idea", label: "解决方案", hue: "var(--mk-matcha)" },
];

/**
 * 每一类便签写的时候该往哪儿使劲。
 *
 * 🚨 产品负责人 2026-09-03：「not just typing texts, but different hints」。
 * 原来四类共用同一句占位符「具体一点：谁、在哪、做了什么」——那句话对「别人的
 * 原话」是错的（原话要照抄，不是概括），对「我的问题」也是错的。同一句提示贴在
 * 四个不同的动作上，等于没有提示。
 *
 * docs/2026-09-01-pbl-detail.md 要的是「a simple observation method」。方法就
 * 落在这儿：每一类各自说清楚怎么写才算写对了。
 */
export const NOTE_KIND_HINTS: Record<NoteKind, { placeholder: string; how: string }> = {
  observation: {
    placeholder: "中午 12:30，三班门口有 6 个人在等",
    how: "请只记录亲眼所见，包含时间、地点、数量，不写结论。",
  },
  quote: {
    placeholder: "他说：这个我肯定不会用",
    how: "请照抄对方原话，不做改动。概括过的内容不算原话。",
  },
  assumption: {
    placeholder: "我猜大家其实是嫌远",
    how: "由所见事实推导得出。标为推论，表示它尚未被证实。",
  },
  question: {
    placeholder: "为什么只有中午排队，晚上不排？",
    how: "观察后仍无法回答的问题。",
  },
  idea: {
    placeholder: "在门口贴一张排队人数的牌子",
    how: "你提出的做法。",
  },
};

export function noteKindMeta(kind: NoteKind): NoteKindMeta {
  return NOTE_KINDS.find((k) => k.kind === kind) ?? OBSERVATION;
}

export function listNotes(projectId: string): Promise<Note[]> {
  return apiFetch<Note[]>(`${base(projectId)}/notes`);
}

export function createNotes(
  projectId: string,
  notes: {
    kind: NoteKind;
    body: string;
    author?: "student" | "yinji";
    cluster?: string;
    imageKey?: string;
  }[],
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

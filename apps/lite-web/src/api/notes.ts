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
  /** 这条便签放进了结构里的哪一块。null = 还在板上。 */
  treeNodeId: string | null;
  /**
   * 她把这张纸摆进了问题陈述的哪一格：谁 / 需要什么 / 为什么。
   * 空 = 还在「我们看到的证据」那一堆里，没被判过。
   *
   * 🚨 和 cluster 是两回事。cluster 是板上的归堆（"这几张是一回事"），这一个是
   * 问题陈述里的角色（"这条是在说谁"）。共用一列会让她在问题识别里摆一下，
   * 就把自己在板上归的堆悄悄擦掉。见 migration 0129。
   */
  reframeSlot: string;
  x: number;
  y: number;
  /** 她挑出来先试的那条办法（只有 kind='idea' 谈得上），和为什么先试它。 */
  picked: boolean;
  pickWhy: string;
  /** 她自己拖过这张纸吗。没拖过的，位置是代码排的，不是她的判断。 */
  dragged: boolean;
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

/**
 * 挪到板上的某个位置。
 *
 * 🚨 dragged 说的是「这一次是她用手拖的」。代码给新便签排座位（boardSpot）、
 * 切换坐标视图时换算单位，走的都是这条 PATCH，但那两次不是她的判断——传 true
 * 会让印记把代码排的位置说成她摆的。默认 false，只有拖动那一处传 true。
 */
export function moveNote(
  projectId: string,
  noteId: string,
  x: number,
  y: number,
  dragged = false,
): Promise<Note> {
  return apiFetch<Note>(`${base(projectId)}/notes/${noteId}`, {
    method: "PATCH",
    body: JSON.stringify({ x, y, dragged }),
  });
}

/**
 * 她挑出来先试的那一条办法，和为什么先试它。
 *
 * 🚨 必须落库，不能只放进 onFinish 的 payload——印记从来看不到那个 payload，
 * 回灌是回头读表的。线上就因此说错过：她挑的是第三条，印记说的是第一条。
 */
export function pickIdea(projectId: string, noteId: string, why: string): Promise<Note> {
  return apiFetch<Note>(`${base(projectId)}/notes/${noteId}/pick`, {
    method: "POST",
    body: JSON.stringify({ why }),
  });
}

export function archiveNote(projectId: string, noteId: string): Promise<Note> {
  return apiFetch<Note>(`${base(projectId)}/notes/${noteId}`, { method: "DELETE" });
}

/**
 * 把一条便签放进结构里的某一块，或者拿回来（nodeId 给 null）。
 *
 * 🚨 「这个分法盖全了吗」以前是个没法回答的问题。她把材料一条一条放进去之后，
 * **放不进去的那几条就是没盖到的地方**——那几条是她亲手收集的，比任何自评都硬。
 */
export function placeNote(
  projectId: string,
  noteId: string,
  nodeId: string | null,
): Promise<Note> {
  return apiFetch<Note>(`${base(projectId)}/notes/${noteId}/place`, {
    method: "POST",
    body: JSON.stringify({ nodeId }),
  });
}

/**
 * 摆进问题陈述的某一格，或者拿回证据堆（slot 给 ""）。
 *
 * 🚨 单独一个端点，不走通用的 PATCH。那一条会连着写 cluster，而摆格子不该碰
 * 她在便签板上归的堆——那是关于同一张纸的另一句判断。见 migration 0129。
 */
export function setNoteReframeSlot(
  projectId: string,
  noteId: string,
  slot: string,
): Promise<Note> {
  return apiFetch<Note>(`${base(projectId)}/notes/${noteId}/reframe-slot`, {
    method: "POST",
    body: JSON.stringify({ slot }),
  });
}

/**
 * 两张便签之间的关系。
 *
 * 🚨 板上的意义不在单张纸上，在两张纸之间。四种关系里**矛盾**最要紧：两条都是
 * 她亲眼看到的却互相打架——真正的问题几乎都是从那儿长出来的。
 */
export type NoteRelation = "causes" | "contradicts" | "same" | "supports";

export const NOTE_RELATIONS: { relation: NoteRelation; label: string; hue: string }[] = [
  { relation: "causes", label: "导致", hue: "var(--mk-mist)" },
  { relation: "contradicts", label: "互相矛盾", hue: "var(--mk-berry)" },
  { relation: "same", label: "同一件事", hue: "var(--mk-matcha)" },
  { relation: "supports", label: "撑着它", hue: "var(--mk-peach)" },
];

export interface NoteLink {
  id: string;
  fromId: string;
  toId: string;
  relation: NoteRelation;
}

export function listNoteLinks(projectId: string): Promise<NoteLink[]> {
  return apiFetch<NoteLink[]>(`${base(projectId)}/note-links`);
}

export function linkNotes(
  projectId: string,
  fromId: string,
  toId: string,
  relation: NoteRelation,
): Promise<NoteLink> {
  return apiFetch<NoteLink>(`${base(projectId)}/note-links`, {
    method: "POST",
    body: JSON.stringify({ fromId, toId, relation }),
  });
}

export function unlinkNotes(projectId: string, linkId: string): Promise<void> {
  return apiFetch<void>(`${base(projectId)}/note-links/${linkId}`, { method: "DELETE" });
}

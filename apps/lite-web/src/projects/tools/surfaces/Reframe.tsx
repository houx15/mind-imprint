import { Says, errorMarkdown } from "../../Says";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { HelpCircle, Heart, Lightbulb, Search, User } from "lucide-react";
import { Icon } from "@/ui";
import {
  confirmReframe,
  createReframe,
  currentReframe,
  draftReframe,
  listReframes,
  reframeSentence,
  seedHmw,
  updateReframe,
  type Reframe as ReframeRow,
} from "../../../api/reframe";
import {
  createNotes,
  listNotes,
  noteKindMeta,
  setNoteReframeSlot,
  updateNote,
  type Note,
} from "../../../api/notes";
import { Stage } from "../board/Stage";
import { DropField } from "../board/DropField";
import { Sticky } from "../board/Sticky";
import { DragGhost } from "../board/DragGhost";
import { useZoneDrag } from "../board/useZoneDrag";
import { SketchArrow } from "../board/sketch";
import { tone, type ToneName } from "../../../shared/tone";
import type { ToolSurfaceProps } from "../registry";
import { apiErrorText } from "../../../api/errorText";

/**
 * Reframe —— 把问题说清楚。一块要用手摆的板。
 *
 * 🚨 产品负责人 2026-09-03，附了一张画好的图：「you said you have finished
 * gamification, but in my view, no. look at these to understand what is
 * interaction」。图上是一块四格的板——她观察到的现象摊在「我们看到的证据」里，
 * 用手拖进「谁 / 需要什么 / 为什么」，底下那句 How might we 跟着填出来。
 *
 * 上一版是**一次一个文本框**，四句问完。那一版的注释写着「四个格子如果一起
 * 摊开，她会把它当成一张表格填完」——那个担心是对的，但答案不是把表格拆成四屏，
 * 是**不要做成表格**。摆位置和填空是两件事：填空要求她当场造一句话；摆位置让她
 * 拿着自己已经写下的观察，去回答"这条到底是在说谁"。后者才是问题识别这门功课。
 *
 * 放在哪一格存在便签自己的 `reframe_slot` 上（migration 0132），**不复用
 * cluster**：cluster 是她在便签板上归的堆（"这几张是一回事"），摆格子是另一句
 * 判断（"这条是在说谁"）。共用一列意味着她在这儿摆一下，就把自己在板上归的堆
 * 悄悄擦掉——悄悄擦掉她的判断，是这个产品最不该做的事。
 *
 * reframe 行的 who / needs / why 由格子里的便签拼出来，每次落一张就写回去，
 * 所以回灌那一头一个字都没动：印记看到的仍旧是那三句话。
 */

type SlotKey = "who" | "needs" | "why";

interface Slot {
  key: SlotKey;
  /**
   * 存进 note.reframeSlot 的值。用中文原词，和界面上写的一模一样——这一列会被
   * 念给印记听（「她把这条判成了『需要什么』」），一个英文枚举到那儿还得翻一次，
   * 而每一次翻译都是一次可以漂移的机会。后端那份白名单在 pbl_board.go。
   */
  slot: string;
  title: string;
  hint: string;
  tone: ToneName;
  icon: typeof User;
  /** 空着的时候那一格自己说要什么。四格四句，不共用一句。 */
  empty: string;
}

const SLOTS: Slot[] = [
  {
    key: "who",
    slot: "谁",
    title: "谁",
    hint: "受影响或涉及的人",
    tone: "mist",
    icon: User,
    empty: "把说到「人」的那几条拖过来",
  },
  {
    key: "needs",
    slot: "需要什么",
    title: "需要什么",
    hint: "他们真正需要的东西",
    tone: "peach",
    icon: Heart,
    empty: "他要的那个东西。先不说你打算怎么给",
  },
  {
    key: "why",
    slot: "为什么",
    title: "为什么",
    hint: "背后的原因与影响",
    tone: "taro",
    icon: HelpCircle,
    empty: "没有会怎样？答得出这个，问题才站得住",
  },
];

/** 证据格：还没被判过的都在这儿。它不是第四个答案，是那一堆原材料。 */
const EVIDENCE = {
  slot: "",
  title: "我们看到的证据",
  hint: "具体观察到的现象",
  tone: "matcha" as ToneName,
  icon: Search,
};

/** 摆进格子的便签 → 存进 reframe 行的那一句。 */
export function joinBodies(notes: Note[]): string {
  return notes.map((n) => n.body.trim()).filter(Boolean).join("、");
}

export function Reframe({ projectId, tool, onFinish, onClose }: ToolSurfaceProps) {
  const [all, setAll] = useState<ReframeRow[]>([]);
  const [row, setRow] = useState<ReframeRow | null>(null);
  const [notes, setNotes] = useState<Note[]>([]);
  const [hmw, setHmw] = useState("");
  const [adding, setAdding] = useState<string | null>(null);
  const [draft, setDraft] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [ready, setReady] = useState(false);
  const [busy, setBusy] = useState(false);

  const previous = currentReframe(all);

  const boot = useCallback(async () => {
    const [list, loadedNotes] = await Promise.all([listReframes(projectId), listNotes(projectId)]);
    setAll(list);
    // 🚨 解决方案不进这块板。这里判的是「问题是什么」，把办法摊在同一张桌上，
    // 她会顺手用一个办法去回答"需要什么"——那正是这一步要拦住的事。
    setNotes(loadedNotes.filter((n) => n.kind !== "idea"));

    const existing = draftReframe(list);
    const live = currentReframe(list);
    const draftRow = existing ?? (await createReframe(projectId, live ? { supersedes: live.id } : {}));
    setRow(draftRow);
    setHmw(draftRow.hmw);
    setReady(true);
  }, [projectId]);

  useEffect(() => {
    void boot();
  }, [boot]);

  /** 每一格里现在有哪几张。 */
  const placed = useMemo(() => {
    const by = new Map<string, Note[]>();
    for (const s of SLOTS) by.set(s.slot, []);
    by.set(EVIDENCE.slot, []);
    for (const n of notes) {
      const at = n.reframeSlot.trim();
      const key = SLOTS.some((s) => s.slot === at) ? at : EVIDENCE.slot;
      by.get(key)!.push(n);
    }
    return by;
  }, [notes]);

  const inSlot = (s: Slot) => placed.get(s.slot) ?? [];
  const evidence = placed.get(EVIDENCE.slot) ?? [];

  /**
   * 把三格拼回 reframe 行。
   *
   * 🚨 每一次落纸都写回去，不等她点"完成"。闭环那一条（产品负责人 2026-09-03
   * 第⑤条）断在这种地方：界面上摆得好好的，印记那边什么都没有。
   */
  const syncRef = useRef<ReframeRow | null>(null);
  syncRef.current = row;
  const sync = useCallback(
    async (next: Note[]) => {
      const r = syncRef.current;
      if (!r) return;
      const pick = (slot: string) => joinBodies(next.filter((n) => n.reframeSlot.trim() === slot));
      try {
        const got = await updateReframe(projectId, r.id, {
          who: pick("谁"),
          needs: pick("需要什么"),
          why: pick("为什么"),
        });
        setRow(got);
      } catch (err) {
        setError(apiErrorText(err));
      }
    },
    [projectId],
  );

  const drag = useZoneDrag({
    onDrop: (id, zone) => {
      // 落在空处 = 放回原处。不当成"清空这张纸的归属"——那会让一次手滑
      // 变成一次撤销不了的判断。
      if (zone === null) return;
      const target = zone === EVIDENCE.title ? "" : zone;
      const note = notes.find((n) => n.id === id);
      if (!note || note.reframeSlot.trim() === target) return;
      const next = notes.map((n) => (n.id === id ? { ...n, reframeSlot: target } : n));
      setNotes(next);
      void sync(next);
      setNoteReframeSlot(projectId, id, target).catch((err) => setError(apiErrorText(err)));
    },
  });

  /** 直接在某一格里写一张。拖是主路，写是退路——她手上不一定正好有那张纸。 */
  async function addInto(slot: string) {
    const body = draft.trim();
    if (!body) return;
    setDraft("");
    setAdding(null);
    try {
      const [made] = await createNotes(projectId, [
        { kind: slot === "谁" ? "observation" : "assumption", body },
      ]);
      if (!made) return;
      // 🚨 建完再摆一次格子。createNotes 那个端点写的是 cluster，不是
      // reframe_slot——少了这一步，她刚写的那条会当场掉回证据堆里。
      const placedNote = await setNoteReframeSlot(projectId, made.id, slot);
      const next = [...notes, placedNote];
      setNotes(next);
      void sync(next);
    } catch (err) {
      setError(apiErrorText(err));
    }
  }

  /**
   * 「这是一个方案」。
   *
   * 上一版把这件事做在输入框旁边，是这件工具最要紧的一道拦截，留着：需求那一格
   * 里最常出现的不是需求，是她想到的第一个做法。一旦写进去，后面所有事都绕着
   * 这个还没被验证的做法转。判成方案不罚——原样变成一条「解决方案」便签，
   * 头脑风暴那边接着用，一个字都不丢。
   */
  async function parkAsIdea(id: string) {
    const next = notes.filter((n) => n.id !== id);
    setNotes(next);
    void sync(next);
    try {
      // 变成一条「解决方案」便签，同时从这块板上收回——两句判断分两个端点，
      // 因为它们本来就是两件事。
      await updateNote(projectId, id, { kind: "idea" });
      await setNoteReframeSlot(projectId, id, "");
    } catch (err) {
      setError(apiErrorText(err));
    }
  }

  const filledSlots = SLOTS.filter((s) => inSlot(s).length > 0).length;
  const sentence = hmw.trim();
  const todo =
    filledSlots < SLOTS.length
      ? `${SLOTS.filter((s) => inSlot(s).length === 0).map((s) => `「${s.title}」`).join("")}还是空的`
      : !sentence
        ? "补全下面那句问题"
        : "";

  async function finish() {
    if (!row) return;
    setBusy(true);
    try {
      // 她自己改过的那一版才算数。原样带走，不拼接、不做正则修补。
      const saved = await updateReframe(projectId, row.id, { hmw: sentence });
      const got = await confirmReframe(projectId, saved.id);
      onFinish({ reframeId: got.id }, got.hmw);
    } catch (err) {
      setError(apiErrorText(err));
      setBusy(false);
    }
  }

  const dragged = drag.drag ? notes.find((n) => n.id === drag.drag!.id) : null;

  function card(n: Note, slot?: Slot) {
    const meta = noteKindMeta(n.kind);
    // 🚨 纸的颜色跟**便签自己的类别**走，不跟它现在落在哪一格走。
    //
    // 第一版是按格子上色，截图一看就露馅：证据堆里三张纸一模一样，而「实际观察」
    // 和「我的推论」的区别恰恰是这块板要教的事——她把一条推论当证据拖进「为什么」
    // 的那一刻，颜色应该在提醒她。而且按格上色的话，同一张纸拖过去会变色，
    // 看起来像换了一张。
    const t = { solid: meta.hue, bg: `color-mix(in srgb, ${meta.hue} 16%, var(--mk-surface))`, fg: "var(--mk-ink)" };
    return (
      <Sticky
        key={n.id}
        tone={t}
        label={meta.label}
        byYinji={n.author === "yinji"}
        dragging={drag.drag?.id === n.id}
        onPointerDown={(e) => drag.start(n.id, e)}
        right={
          slot?.key === "needs" ? (
            <button
              type="button"
              onPointerDown={(e) => e.stopPropagation()}
              onClick={() => void parkAsIdea(n.id)}
              title="这是一个方案，先放回想法里"
              className="shrink-0 rounded-mk-full px-1.5 py-0.5 text-mk-small"
              style={{ background: "var(--mk-surface)", color: "var(--mk-secondary)" }}
            >
              这是方案
            </button>
          ) : undefined
        }
      >
        {n.body}
      </Sticky>
    );
  }

  return (
    <Stage
      // 🚨 用 tool.label（「问题识别」），不用图上写的「重新定义问题」。邀请卡、
      // 材料清单、这块板的标题必须是同一个名字——同一件东西两个名字，是
      // AGENTS.md 里那条「不手写第二份」防的正是这种漂移。
      // 图上那个名字更说得清方法，改不改是产品决定，已经问了产品负责人。
      title={tool.label}
      task="把你观察到的现象，放到合适的位置，帮我们看清问题。"
      why={tool.reason}
      step={{ now: filledSlots + (sentence ? 1 : 0), total: SLOTS.length + 1 }}
      todo={todo}
      insight="问题说清楚了，接下来才谈得上想办法。"
      finishLabel="形成 How might we"
      onFinish={() => void finish()}
      onClose={onClose}
      busy={busy}
    >
      {error && (
        <div className="mb-2 text-mk-small" style={{ color: "var(--mk-danger)" }}>
          <Says content={errorMarkdown(error)} />
        </div>
      )}

      {previous && (
        // 她原来以为问题是什么。看得见这一句，才知道自己刚改动了什么。
        <div className="mb-3 rounded-mk-md px-3 py-2" style={{ background: "var(--mk-surface)" }}>
          <p className="text-mk-small text-mk-muted">你之前说的是</p>
          <p className="mt-0.5 text-mk-small text-mk-secondary">{reframeSentence(previous)}</p>
        </div>
      )}

      {!ready && <p className="text-mk-small text-mk-muted">打开中…</p>}

      {ready && row && (
        <>
          {/* auto-rows-fr：四格等高。不等高的时候「谁」那一行矮一截，看起来像
              两块内容 + 两块空白，而不是一块四格的板。 */}
          <div className="grid gap-3 md:grid-cols-2 md:auto-rows-fr">
            {SLOTS.map((s) => (
              <DropField
                key={s.key}
                zoneRef={drag.zoneRef(s.slot)}
                testId={`reframe-zone-${s.key}`}
                icon={s.icon}
                title={s.title}
                hint={s.hint}
                tone={tone(s.tone)}
                active={drag.drag?.over === s.slot}
                count={inSlot(s).length}
                isEmpty={inSlot(s).length === 0 && adding !== s.slot}
                empty={s.empty}
                // 🚨 比默认矮一点。四格按默认高度撑开之后，1280×720 上「整合
                // 起来」那句话——这块板唯一的产出——被挤到折叠线以下了。
                minHeight={132}
                onAdd={() => {
                  setAdding(s.slot);
                  setDraft("");
                }}
              >
                {adding === s.slot && (
                  <input
                    autoFocus
                    value={draft}
                    onChange={(e) => setDraft(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === "Enter") void addInto(s.slot);
                      if (e.key === "Escape") setAdding(null);
                    }}
                    onBlur={() => void addInto(s.slot)}
                    placeholder="写一条，回车放进这一格"
                    className="rounded-mk-md border border-mk-input-border bg-mk-surface px-2.5 py-1.5 text-mk-body text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent-200"
                  />
                )}
                {inSlot(s).map((n) => card(n, s))}
              </DropField>
            ))}

            <DropField
              zoneRef={drag.zoneRef(EVIDENCE.title)}
              testId="reframe-zone-evidence"
              icon={EVIDENCE.icon}
              title={EVIDENCE.title}
              hint={EVIDENCE.hint}
              tone={tone(EVIDENCE.tone)}
              active={drag.drag?.over === EVIDENCE.title}
              count={evidence.length}
              isEmpty={evidence.length === 0}
              empty="还没有材料。先去便签板上把看到的写下来，或者点右上角的 ＋ 直接写一条。"
            >
              {evidence.map((n) => card(n))}
            </DropField>
          </div>

          {/* 整合起来：三格拼出来的那一句。四张图里每一块板底下都有这样一句——
              板上摆的东西必须收成一件她能带走的东西，否则那只是摆纸。 */}
          <div className="mt-4 flex items-start gap-2">
            <div className="hidden shrink-0 pt-6 text-center sm:block">
              <span className="text-mk-small" style={{ color: "var(--mk-accent-500)" }}>
                整合起来
              </span>
              <SketchArrow />
            </div>

            <div
              className="min-w-0 flex-1 rounded-mk-lg p-3"
              style={{ background: "var(--mk-surface)", boxShadow: "inset 0 0 0 2px var(--mk-accent-200)" }}
            >
              <textarea
                value={hmw}
                onChange={(e) => setHmw(e.target.value)}
                rows={2}
                placeholder={seedHmw(row.who, row.needs)}
                className="w-full resize-none bg-transparent text-mk-body-lg text-mk-ink outline-none placeholder:text-mk-faint"
              />
              <div className="mt-1 flex flex-wrap items-center justify-between gap-2">
                <p className="flex items-center gap-1.5 text-mk-small text-mk-faint">
                  <Icon icon={Lightbulb} size={13} />
                  {row.why ? `因为${row.why}` : "把上面的内容放好后，试着补全这句话"}
                </p>
                {!sentence && (row.who || row.needs) && (
                  // 起个头。填进去的每一个字都是她自己写在便签上的话——
                  // 模板只负责摆成一个问句，她拿到之后随便改。见 seedHmw。
                  <button
                    type="button"
                    onClick={() => setHmw(seedHmw(row.who, row.needs))}
                    className="rounded-mk-full px-3 py-1 text-mk-small"
                    style={{ background: "var(--mk-accent-50)", color: "var(--mk-accent-500)" }}
                  >
                    用上面的内容起个头
                  </button>
                )}
              </div>
            </div>
          </div>
        </>
      )}

      <DragGhost drag={drag.drag}>
        {dragged && (
          <Sticky
            tone={{
              solid: noteKindMeta(dragged.kind).hue,
              bg: `color-mix(in srgb, ${noteKindMeta(dragged.kind).hue} 16%, var(--mk-surface))`,
              fg: "var(--mk-ink)",
            }}
            label={noteKindMeta(dragged.kind).label}
          >
            {dragged.body}
          </Sticky>
        )}
      </DragGhost>
    </Stage>
  );
}

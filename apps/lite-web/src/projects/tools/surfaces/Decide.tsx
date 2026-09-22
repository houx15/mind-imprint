import { Says, errorMarkdown } from "../../Says";
import { useCallback, useEffect, useRef, useState } from "react";
import { Check, GripVertical } from "lucide-react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { Icon } from "@/ui";
import { apiErrorText } from "../../../api/errorText";
import { ApiError } from "../../../api/client";
import {
  addOption,
  getDecisionDraft,
  saveDecisionDraft,
  type DecisionDraft,
  type DecisionDraftState,
  decisionTodo,
  rankOptions,
  joinWhyNot,
  listDecisions,
  openDecisionOf,
  optionTone,
  optionTag,
  settleDecision,
  type Decision,
} from "../../../api/decide";
import { Stage } from "../board/Stage";
import { DragGhost } from "../board/DragGhost";
import { useZoneDrag } from "../board/useZoneDrag";
import type { ToolSurfaceProps } from "../registry";
import { beforeNavigate } from "../../../routing";
import { GrowingTextarea } from "../../../shared/GrowingTextarea";
import { textChanges, remarkTextChanges, type TextRange } from "../../../shared/textChanges";

/**
 * Decide —— 理性决策。
 *
 * 产品负责人 2026-09-02：
 *   「it happens when AI proposes several thing to decide... present several
 *    options as cards, with title, description, and students can select one to
 *    confirm. but when confirm, they need to answer two small questions:
 *    why this, and why not others.」
 *
 * 所以这一屏只做三件事：把印记提的几条路摆成卡片、让她选一张、问两个小问题。
 *
 * 🚨 第二个问题（为什么不选别的）才是这件工具真正教的东西。选中一个不难——
 * 顺眼就点了；说得出为什么放掉另外两条，才说明她真的把它们放在一起比过。
 *
 * 上一版是我自己设计的：她加选项、定标准、给每个选项填「赢在哪疼在哪」。那既是
 * 让她替印记把活干了，也把一个当场的判断拖成了一张表格。已废弃。
 */
export function Decide({ projectId, tool, onFinish, onClose }: ToolSurfaceProps) {
  const [decision, setDecision] = useState<Decision | null>(null);
  const [ready, setReady] = useState(false);
  const [choice, setChoice] = useState("");
  const [why, setWhy] = useState("");
  // 🚨 放掉的每一条分开答。一个大框只会得到「其他的都不太合适」，而
  // "为什么放掉另外那两条" 才是这件工具真正教的东西。
  const [dropped, setDropped] = useState<Record<string, string>>({});
  // 🚨 什么情况会让她改主意。`pbl_decision.flip` 这一列 0111 就加了，0113 把
  // 界面撤掉之后一直空着——而它是复盘阶段唯一能回头对照的东西：当初写下的那个
  // 条件，后来到底发生了没有。一个决定因此从一次表态变成一个可以被推翻的假设。
  //
  // 不设成门槛：一个决定不写翻盘条件也仍然是个决定，硬拦只会逼出一句应付的话。
  const [flip, setFlip] = useState("");
  // 她自己要加的那条路。
  const [adding, setAdding] = useState(false);
  const [mineLabel, setMineLabel] = useState("");
  const [mineWhy, setMineWhy] = useState("");

  /**
   * 把一条路拖到另一条前面。
   *
   * 🚨 上一版是 ↑↓ 两个小箭头，注释里我写着「三五张卡就在眼前，点一下最直接」。
   * 产品负责人 2026-09-03 推翻了这个判断（「not just typing texts, but ...
   * draggable」，附四张图，其中一张写着「把最主要的障碍拖到最左」）。她是对的：
   * ↑↓ 是在**操作一个列表**，拖是在**把几条路摆在一起比**，而"摆在一起比过了"
   * 正是这件工具要留下的证据。
   */
  async function reorder(fromId: string, toId: string) {
    if (!decision || fromId === toId) return;
    const order = ordered.map((o) => o.id);
    const from = order.indexOf(fromId);
    const to = order.indexOf(toId);
    if (from < 0 || to < 0) return;
    order.splice(to, 0, ...order.splice(from, 1));
    try {
      setDecision(await rankOptions(projectId, decision.id, order));
    } catch (err) {
      setError(apiErrorText(err));
    }
  }
  const [error, setError] = useState<string | null>(null);
  const [draftLoaded, setDraftLoaded] = useState(false);
  const [leaving, setLeaving] = useState(false);
  const [saving, setSaving] = useState(false);
  const [draftError, setDraftError] = useState<string | null>(null);
  const [conflict, setConflict] = useState(false);
  const [remoteDraft, setRemoteDraft] = useState<DecisionDraftState | null>(null);
  const [savedSnapshot, setSavedSnapshot] = useState("");
  const [reviewedVersion, setReviewedVersion] = useState(0);
  const revision = useRef(0);
  const saved = useRef("");
  const pending = useRef<Promise<void> | null>(null);
  const closed = useRef(false);
  const loadEpoch = useRef(0);
  const snapshot = JSON.stringify({
    contentVersion: reviewedVersion,
    choiceId: decision?.options.find((o) => o.label === choice)?.id ?? "",
    why, dropped, flip, adding, mineLabel, mineWhy,
  } satisfies DecisionDraft);
  const latest = useRef(snapshot);
  latest.current = snapshot;

  const flushDraft = useCallback((): Promise<void> => {
    if (pending.current) return pending.current;
    if (!draftLoaded || !decision || closed.current) return Promise.resolve();
    const save = async () => {
      setSaving(true);
      try {
        while (saved.current !== latest.current && !closed.current) {
          const value = latest.current;
          const result = await saveDecisionDraft(projectId, decision.id, JSON.parse(value), revision.current);
          revision.current = result.revision;
          saved.current = value;
          setSavedSnapshot(value);
        }
        setDraftError(null);
        setConflict(false);
      } catch (err) {
        setConflict(err instanceof ApiError && err.status === 409);
        setDraftError(apiErrorText(err));
        throw err;
      } finally {
        setSaving(false);
        pending.current = null;
      }
    };
    // A microtask ensures pending is assigned before a no-op save finishes.
    pending.current = Promise.resolve().then(save);
    return pending.current;
  }, [draftLoaded, decision?.id, projectId]);

  useEffect(() => {
    if (!draftLoaded || snapshot === saved.current) return;
    const timer = window.setTimeout(() => { void flushDraft().catch(() => {}); }, 300);
    return () => window.clearTimeout(timer);
  }, [snapshot, draftLoaded, flushDraft]);

  useEffect(() => beforeNavigate(async () => {
    setLeaving(true);
    try { await flushDraft(); }
    finally { setLeaving(false); }
  }), [flushDraft]);

  useEffect(() => {
    if (!draftLoaded || snapshot === savedSnapshot) return;
    const warn = (event: BeforeUnloadEvent) => { event.preventDefault(); };
    window.addEventListener("beforeunload", warn);
    return () => window.removeEventListener("beforeunload", warn);
  }, [draftLoaded, snapshot, savedSnapshot]);

  async function close() {
    if (leaving) return;
    setLeaving(true);
    try { await flushDraft(); onClose(); } catch { setLeaving(false); }
  }

  async function inspectSavedDraft() {
    if (!decision) return;
    try { setRemoteDraft(await getDecisionDraft(projectId, decision.id)); }
    catch (err) { setDraftError(apiErrorText(err)); }
  }

  async function keepCurrentDraft() {
    if (!remoteDraft || remoteDraft.settled) return;
    revision.current = remoteDraft.revision;
    try { await flushDraft(); setRemoteDraft(null); }
    catch { setRemoteDraft(null); }
  }

  function useSavedDraft() {
    if (!remoteDraft || !decision || remoteDraft.settled || pending.current) return;
    const d = remoteDraft.draft;
    const restored = {
      contentVersion: d.contentVersion ?? 0,
      choiceId: decision.options.some((o) => o.id === d.choiceId) ? d.choiceId! : "",
      why: d.why ?? "", dropped: d.dropped ?? {}, flip: d.flip ?? "",
      adding: d.adding ?? false, mineLabel: d.mineLabel ?? "", mineWhy: d.mineWhy ?? "",
    };
    revision.current = remoteDraft.revision;
    saved.current = JSON.stringify(restored);
    setSavedSnapshot(saved.current);
    setReviewedVersion(restored.contentVersion);
    setChoice(decision.options.find((o) => o.id === restored.choiceId)?.label ?? "");
    setWhy(restored.why); setDropped(restored.dropped); setFlip(restored.flip);
    setAdding(restored.adding); setMineLabel(restored.mineLabel); setMineWhy(restored.mineWhy);
    setDraftError(null); setConflict(false); setRemoteDraft(null);
  }

  const boot = useCallback(async () => {
    const epoch = ++loadEpoch.current;
    setReady(false);
    setError(null);
    try {
      const current = openDecisionOf(await listDecisions(projectId));
      if (current) {
        const state = await getDecisionDraft(projectId, current.id);
        if (epoch !== loadEpoch.current) return;
        if (state.settled) throw new Error("这个决定已确认，请重新打开");
        const d = state.draft;
        const restored = {
          contentVersion: d.contentVersion ?? 0,
          choiceId: current.options.some((o) => o.id === d.choiceId) ? d.choiceId! : "",
          why: d.why ?? "", dropped: d.dropped ?? {}, flip: d.flip ?? "",
          adding: d.adding ?? false, mineLabel: d.mineLabel ?? "", mineWhy: d.mineWhy ?? "",
        };
        revision.current = state.revision;
        saved.current = JSON.stringify(restored);
        setSavedSnapshot(saved.current);
        setReviewedVersion(restored.contentVersion);
        setChoice(current.options.find((o) => o.id === restored.choiceId)?.label ?? "");
        setWhy(restored.why); setDropped(restored.dropped); setFlip(restored.flip);
        setAdding(restored.adding); setMineLabel(restored.mineLabel); setMineWhy(restored.mineWhy);
        setDraftLoaded(true);
      }
      if (epoch !== loadEpoch.current) return;
      setDecision(current);
    } catch (err) {
      if (epoch === loadEpoch.current) setError(apiErrorText(err));
    } finally {
      if (epoch === loadEpoch.current) setReady(true);
    }
  }, [projectId]);

  useEffect(() => {
    void boot();
    return () => { loadEpoch.current++; };
  }, [boot]);

  const whyNot = decision
    ? joinWhyNot(
        decision.options
          .filter((o) => o.label !== choice)
          .map((o) => ({ label: o.label, why: dropped[o.id] ?? "" })),
      )
    : "";

  async function finish() {
    if (!decision || leaving || reviewedVersion !== decision.contentVersion) return;
    setLeaving(true);
    try {
      await flushDraft();
      const got = await settleDecision(projectId, decision.id, {
        choice: choice.trim(),
        why: why.trim(),
        whyNot: whyNot.trim(),
        flip: flip.trim(),
        contentVersion: reviewedVersion,
      });
      closed.current = true;
      onFinish({ decisionId: got.id, choice: got.choice }, got.why);
    } catch (err) {
      setLeaving(false);
      setError(apiErrorText(err));
    }
  }

  async function addMine() {
    if (!decision || !mineLabel.trim()) return;
    try {
      setDecision(await addOption(projectId, decision.id, {
        label: mineLabel.trim(),
        description: mineWhy.trim(),
      }));
      setAdding(false);
      setMineLabel("");
      setMineWhy("");
    } catch (err) {
      setError(apiErrorText(err));
    }
  }

  const todo = decision && reviewedVersion !== decision.contentVersion ? "请核对修订内容与原理由" : decisionTodo(decision, { choice, why, whyNot });
  // 排过就按名次显示，没排过就按印记给的顺序。
  const ordered = decision
    ? [...decision.options].sort((a, b) => {
        if (a.studentRank && b.studentRank) return a.studentRank - b.studentRank;
        if (a.studentRank) return -1;
        if (b.studentRank) return 1;
        return a.ordinal - b.ordinal;
      })
    : [];
  const chosenIndex = ordered.findIndex((o) => o.label === choice);
  const chosenTone = optionTone(chosenIndex < 0 ? 0 : chosenIndex);
  const answeredDrops = decision
    ? decision.options.filter((o) => o.label !== choice && (dropped[o.id] ?? "").trim() !== "")
        .length
    : 0;

  /**
   * 刚刚真的拖过一次。
   *
   * 🚨 选中走 onClick，不走 useZoneDrag 的 onTap。onTap 那条路在 e2e 里点不亮
   * （拖动排序是好的，紧接着的一次点击选不中任何东西），而 onClick 是这块界面
   * 一直用的、验过的那条。真拖过之后浏览器不会在原来那张卡上再发一次 click
   * （mouseup 落在别的元素上），这个 ref 只兜住"在同一张卡上小幅挪了一下"
   * 那一种：那是一次排序，不该顺手把它选中。
   */
  const justDragged = useRef(false);

  // 拖着一张卡去插到另一张前面。选定之后不再排：那时候要她做的是解释，不是继续比。
  const drag = useZoneDrag({
    onDrop: (id, zone) => {
      justDragged.current = true;
      window.setTimeout(() => (justDragged.current = false), 0);
      if (!zone || choice) return;
      void reorder(id, zone.replace("opt:", ""));
    },
  });
  const draggedOption = drag.drag ? ordered.find((o) => o.id === drag.drag!.id) : null;

  return (
    <Stage
      title={tool.label}
      task="针对每个选项的原因及可能后果进行深入思考，再做出决定"
      why={tool.reason}
      todo={todo}
      insight={choice ? `暂选：${choice}` : undefined}
      finishLabel="确认选择"
      busy={leaving}
      onFinish={() => void finish()}
      onClose={() => void close()}
    >
      {error && (
        <div className="mb-2 text-mk-small" style={{ color: "var(--mk-danger)" }}>
          <Says content={errorMarkdown(error)} />
          {!decision && <button className="ml-2 underline" onClick={() => void boot()}>重新加载</button>}
        </div>
      )}

      {ready && !decision && !error && (
        <p className="text-mk-small text-mk-muted">
          暂时没有需要决策的内容。印记提出几个方案时，会在这里让你选。
        </p>
      )}

      {decision && (
        <div ref={(el) => { if (el) el.inert = leaving; }}>
          <div className="mb-2 text-mk-small text-mk-muted" role="status">
            {draftError ? <Says content={errorMarkdown(`保存失败：${draftError}`)} /> : saving || snapshot !== savedSnapshot ? "草稿保存中" : "草稿已保存"}
            {draftError && (conflict
              ? <button className="ml-2 underline" onClick={() => void inspectSavedDraft()}>查看已保存版本</button>
              : <button className="ml-2 underline" onClick={() => void flushDraft().catch(() => {})}>重试保存</button>)}
          </div>
          {remoteDraft && (
            <section className="mb-3 rounded-mk-md border border-mk-border p-3 text-mk-small">
              <h3 className="font-semibold">已保存版本</h3>
              {remoteDraft.settled ? <>
                <p>这个决定已在另一页面确认，当前草稿无法覆盖。请保留需要的文字后关闭当前草稿。</p>
                <button className="mt-2 underline" onClick={() => { closed.current = true; onClose(); }}>关闭未保存草稿</button>
              </> : <>
                <p>另一页面已保存新内容。请对照下面的版本，再决定是否用当前输入替换。</p>
                <p>暂选方案：{decision.options.find((o) => o.id === remoteDraft.draft.choiceId)?.label ?? "未选择"}</p>
                <p className="whitespace-pre-wrap">选择原因：{remoteDraft.draft.why || "未填写"}</p>
                {Object.entries(remoteDraft.draft.dropped ?? {}).map(([id, reason]) => <p key={id} className="whitespace-pre-wrap">未选原因（{decision.options.find((o) => o.id === id)?.label ?? "原方案"}）：{reason}</p>)}
                <p className="whitespace-pre-wrap">改主意的条件：{remoteDraft.draft.flip || "未填写"}</p>
                <p className="whitespace-pre-wrap">自定义方案：{remoteDraft.draft.mineLabel || "未填写"} {remoteDraft.draft.mineWhy}</p>
                <button className="mt-2 underline" onClick={() => void keepCurrentDraft()}>保留当前输入并保存</button>
                <button className="ml-3 mt-2 underline" onClick={useSavedDraft}>使用已保存版本</button>
              </>}
            </section>
          )}
          <p className="text-mk-body text-mk-ink">{decision.subject}</p>
          {(decision.revisionHistory?.length ?? 0) > 0 && <section className="my-3 rounded-mk-md border border-mk-border p-3 text-mk-small">
            <h3 className="font-semibold">修订记录 · 第 {decision.contentVersion + 1} 版</h3>
            <p>候选方案已修改。请比较修改前后的内容，并核对原理由是否仍然适用。</p>
            {decision.revisionHistory.map((previous, index) => {
              const next = decision.revisionHistory[index + 1] ?? decision;
              return <details key={previous.version} className="my-2" open={index === decision.revisionHistory.length - 1}>
                <summary className="cursor-pointer">第 {previous.version + 1} → {previous.version + 2} 版：{previous.reason}</summary>
                {previous.subject !== next.subject && <p className="my-2">主题：{previous.subject} → {next.subject}</p>}
                {previous.options.map((old) => {
                  const updated = next.options.find((o) => o.id === old.id);
                  if (!updated || (old.label === updated.label && old.description === updated.description)) return null;
                  const changes = textChanges(old.description, updated.description);
                  return <div key={old.id} className="my-5 grid gap-4 border-t-2 border-mk-border pt-5 md:grid-cols-2">
                    <section className="min-w-0 overflow-hidden rounded-mk-md border-2 border-rose-200 bg-white">
                      <header className="border-b border-rose-200 bg-rose-50 px-4 py-3"><p className="mb-1 font-semibold text-rose-800">修改前 · 删除处已标记</p><h4 className="font-semibold">{old.label}</h4></header>
                      <div className="px-4 pb-4"><DecisionText ranges={changes.before} removed>{old.description}</DecisionText></div>
                    </section>
                    <section className="min-w-0 overflow-hidden rounded-mk-md border-2 border-emerald-300 bg-white">
                      <header className="border-b border-emerald-200 bg-emerald-50 px-4 py-3"><p className="mb-1 font-semibold text-emerald-800">修改后 · 新增处已高亮</p><h4 className="font-semibold">{updated.label}</h4></header>
                      <div className="px-4 pb-4"><DecisionText ranges={changes.after}>{updated.description}</DecisionText></div>
                    </section>
                  </div>;
                })}
              </details>;
            })}
            {reviewedVersion !== decision.contentVersion && <button className="mt-2 underline" onClick={() => setReviewedVersion(decision.contentVersion)}>已核对修订内容与原理由</button>}
          </section>}

          {/* 🚨 印记给的这几条不是全集。「在别人摆好的选项里挑一个」和「决定」
              是两回事——后者包含「这些都不对，我要的是另一样」。 */}
          {!choice &&
            (adding ? (
              <div className="mt-2 rounded-mk-md border border-dashed border-mk-border px-3 py-2.5">
                <input
                  autoFocus
                  value={mineLabel}
                  onChange={(e) => setMineLabel(e.target.value)}
                  placeholder="方案名称"
                  className="w-full rounded-mk-md border border-mk-input-border bg-mk-surface px-2.5 py-1.5 text-mk-small text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent-200"
                />
                <GrowingTextarea
                  value={mineWhy}
                  onChange={(e) => setMineWhy(e.target.value)}
                  placeholder="方案说明"
                  className="mt-1.5 w-full rounded-mk-md border border-mk-input-border bg-mk-surface px-2.5 py-1.5 text-mk-small text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent-200"
                />
                <div className="mt-2 flex gap-2">
                  <button
                    type="button"
                    onClick={() => void addMine()}
                    disabled={!mineLabel.trim()}
                    className="rounded-mk-full px-3 py-1 text-mk-small disabled:opacity-40"
                    style={{ background: "var(--mk-accent-500)", color: "var(--mk-surface)" }}
                  >
                    添加方案
                  </button>
                  <button
                    type="button"
                    onClick={() => setAdding(false)}
                    className="text-mk-small text-mk-secondary"
                  >
                    取消
                  </button>
                </div>
              </div>
            ) : (
              <button
                type="button"
                onClick={() => setAdding(true)}
                className="mt-2 w-full rounded-mk-md border border-dashed border-mk-border py-2 text-mk-small text-mk-secondary"
              >
                都不合适，我有别的方案
              </button>
            ))}

          <p className="mt-1 text-mk-small text-mk-muted">
            {choice ? "请说明未选择其他方案的原因。" : "请拖动卡片排出顺序，然后选择一个方案。"}
          </p>

          {/* 一条从左到右的轴。没有它，横着排的三张卡只是三张卡；有了它，
              左右的位置才是一句话。图上那句是「越往左越关键」。 */}
          {!choice && ordered.length > 1 && (
            <div className="mt-3 flex items-center gap-2 text-mk-small text-mk-faint">
              <span style={{ color: "var(--mk-accent-500)" }}>从左到右排列方案优先级</span>
              <span
                className="h-px flex-1"
                style={{
                  background:
                    "linear-gradient(to right, var(--mk-accent-200), color-mix(in srgb, var(--mk-border) 80%, transparent))",
                }}
              />
            </div>
          )}

          <div className={`mt-2 grid gap-2 ${ordered.length === 3 ? "md:grid-cols-3" : "md:grid-cols-2"}`}>
            {ordered.map((o, i) => {
              const on = choice === o.label;
              const t = optionTone(i);
              // 选中一张之后，别的暗下去——她要看见的是"我挑了这条，放掉了那些"。
              const faded = choice !== "" && !on;
              return (
                <div
                  key={o.id}
                  role="button"
                  tabIndex={0}
                  aria-pressed={on}
                  ref={drag.zoneRef(`opt:${o.id}`)}
                  // 🚨 拖和选是同一个手势的两半（useZoneDrag 的 onDrop / onTap）。
                  // 分成"拖把手 + 点正文"在触屏上两个都不好按，而这块板上她做得
                  // 最多的两件事就是这两件。
                  onPointerDown={(e) => !choice && drag.start(o.id, e)}
                  onClick={() => {
                    if (justDragged.current) return;
                    setChoice(on ? "" : o.label);
                  }}
                  onKeyDown={(e) => {
                    if (e.key !== "Enter" && e.key !== " ") return;
                    e.preventDefault();
                    if (!e.repeat) setChoice(on ? "" : o.label);
                  }}
                  className="block h-full w-full rounded-mk-md border px-3 py-2.5 text-left transition-opacity"
                  // 🚨 整张卡染上这条路自己的淡底，不挂左侧色条。
                  style={{
                    borderColor: on ? t.solid : drag.drag?.over === `opt:${o.id}` ? t.solid : "transparent",
                    background: t.bg,
                    cursor: choice ? "pointer" : "grab",
                    touchAction: "none",
                    opacity: faded ? 0.5 : drag.drag?.id === o.id ? 0.35 : 1,
                    boxShadow: on ? `0 0 0 2px color-mix(in srgb, ${t.solid} 32%, transparent)` : undefined,
                  }}
                >
                  <div className="flex items-start gap-2">
                    <span
                      className="mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center rounded-mk-full text-[11px] font-semibold"
                      style={{ background: t.solid, color: "var(--mk-surface)" }}
                    >
                      {optionTag(i)}
                    </span>
                    <div className="min-w-0 flex-1">
                      <span className="block text-mk-small font-semibold" style={{ color: t.fg }}>
                        {o.label}
                        {/* 她自己加的那条要认得出来——那是她的判断，不是印记的提议。 */}
                        {o.author === "student" && (
                          <span className="ml-1.5 text-mk-small font-normal text-mk-muted">
                            你加的
                          </span>
                        )}
                      </span>
                      {o.description && (
                        <DecisionText>{o.description}</DecisionText>
                      )}
                    </div>
                    {/* 抓手。一直在，不靠 hover 才出现——触屏上没有 hover，
                        而她第一次看见这三张卡时最需要知道的就是"这个能拿起来"。 */}
                    {!choice && ordered.length > 1 && (
                      <Icon
                        icon={GripVertical}
                        size={14}
                        className="mt-0.5 shrink-0 opacity-40"
                        style={{ color: t.solid }}
                      />
                    )}
                    {on && (
                      <span className="mt-0.5 shrink-0" style={{ color: t.solid }}>
                        <Icon icon={Check} size={15} />
                      </span>
                    )}
                  </div>
                </div>
              );
            })}
          </div>

          {/* 两个小问题。选完才出现——先比较，再解释。 */}
          {choice && (
            <div className="mt-5 space-y-4 border-t border-mk-border pt-4">
              <div>
                <div className="flex items-center gap-2">
                  <span
                    className="h-3.5 w-1 rounded-mk-full"
                    style={{ background: chosenTone.solid }}
                  />
                  <label className="text-mk-body font-semibold text-mk-ink">选择原因</label>
                </div>
                <GrowingTextarea
                  value={why}
                  onChange={(e) => setWhy(e.target.value)}
                  rows={2}
                  placeholder="请说明它解决了什么，或它比其他方案好在哪里"
                  className="mt-1.5 w-full resize-none rounded-mk-md border border-mk-input-border bg-mk-surface px-2.5 py-2 text-mk-small text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent-200"
                />
              </div>

              <div>
                <div className="flex items-center gap-2">
                  <span
                    className="h-3.5 w-1 rounded-mk-full"
                    style={{ background: "var(--mk-warning)" }}
                  />
                  <label className="text-mk-body font-semibold text-mk-ink">改主意的条件</label>
                  <span className="text-mk-small text-mk-faint">选填</span>
                </div>
                <p className="mt-0.5 text-mk-small text-mk-muted">
                  出现什么情况，你会回来改这个决定。复盘时会拿它对照。
                </p>
                <input
                  value={flip}
                  onChange={(e) => setFlip(e.target.value)}
                  placeholder="例如：接龙发出去两天还不到 5 个人报名"
                  className="mt-1.5 w-full rounded-mk-md border border-mk-input-border bg-mk-surface px-2.5 py-1.5 text-mk-small text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent-200"
                />
              </div>

              {/* 🚨 放掉的每一条各答一次，而不是一个大框。
                  一个大框得到的是「其他的都不太合适」；一张一张摆在面前，她才会
                  真的想起每一条当初为什么看起来可行。 */}
              <div>
                <div className="flex items-center gap-2">
                  <span className="h-3.5 w-1 rounded-mk-full" style={{ background: "var(--mk-border)" }} />
                  <label className="text-mk-body font-semibold text-mk-ink">
                    未选方案（{answeredDrops}/{decision.options.length - 1}）
                  </label>
                </div>
                <p className="mt-0.5 text-mk-small text-mk-muted">
                  请逐项说明与「{choice}」相比，未选择该方案的原因。
                </p>
                <div className="mt-2 space-y-2">
                  {ordered.map((o, i) =>
                    o.label === choice ? null : (
                      <div
                        key={o.id}
                        className="rounded-mk-md border px-3 py-2"
                        style={{ borderColor: "transparent", background: optionTone(i).bg }}
                      >
                        <div className="flex items-center gap-2">
                          <span
                            className="flex h-5 w-5 shrink-0 items-center justify-center rounded-mk-full text-[11px] font-semibold"
                            style={{ background: optionTone(i).solid, color: "var(--mk-surface)" }}
                          >
                            {optionTag(i)}
                          </span>
                          <span className="text-mk-small" style={{ color: optionTone(i).fg }}>
                            {o.label}
                          </span>
                        </div>
                        <GrowingTextarea
                          value={dropped[o.id] ?? ""}
                          onChange={(e) =>
                            setDropped((prev) => ({ ...prev, [o.id]: e.target.value }))
                          }
                          placeholder="放弃原因"
                          className="mt-1.5 w-full rounded-mk-md border border-mk-input-border bg-mk-surface px-2.5 py-1.5 text-mk-small text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent-200"
                        />
                      </div>
                    ),
                  )}
                </div>
              </div>
            </div>
          )}
        </div>
      )}
      <DragGhost drag={drag.drag}>
        {draggedOption && (
          <div
            className="rounded-mk-md px-3 py-2.5 text-mk-small font-semibold"
            style={{
              background: optionTone(ordered.indexOf(draggedOption)).bg,
              color: optionTone(ordered.indexOf(draggedOption)).fg,
            }}
          >
            {draggedOption.label}
          </div>
        )}
      </DragGhost>
    </Stage>
  );
}

function DecisionText({ children, ranges = [], removed = false }: { children: string; ranges?: TextRange[]; removed?: boolean }) {
  return (<div className="mt-2 break-words text-mk-small leading-relaxed text-mk-secondary [&_p]:my-2 [&_ul]:my-2 [&_ul]:list-disc [&_ul]:pl-5 [&_ol]:my-2 [&_ol]:list-decimal [&_ol]:pl-5 [&_li]:my-1 [&_strong]:font-semibold [&_h1]:font-semibold [&_h2]:font-semibold [&_h3]:font-semibold [&_pre]:overflow-x-auto [&_table]:block [&_table]:overflow-x-auto [&_td]:border [&_td]:p-1 [&_th]:border [&_th]:p-1">
                          <ReactMarkdown
                            remarkPlugins={[remarkGfm, [remarkTextChanges, { source: children, ranges, removed }]]}
                            skipHtml
                            // 卡片整体负责选择，正文不嵌套链接或其他交互控件。
                            components={{
                              a: ({ children }) => <span>{children}</span>,
                              img: ({ alt }) => <span>{alt}</span>,
                              input: ({ checked }) => <span>{checked ? "☑" : "☐"}</span>,
                              del: ({ children }) => <del className="bg-rose-100 text-rose-900 decoration-rose-600 decoration-2">{children}</del>,
                              mark: ({ children }) => <mark className="rounded-sm bg-emerald-100 px-0.5 font-medium text-emerald-950">{children}</mark>,
                            }}
                          >
                            {children}
                          </ReactMarkdown>
                        </div>);
}

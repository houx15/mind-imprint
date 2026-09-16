import { Says, errorMarkdown } from "../../Says";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { MessageSquareQuote } from "lucide-react";
import { Icon } from "@/ui";
import {
  isDocument,
  listArtifacts,
  pendingArtifacts,
  refreshSiteReview,
  settleArtifact,
  type Artifact,
} from "../../../api/artifacts";
import {
  answerDimension,
  answerMark,
  askAbout,
  getReview,
  type ReviewMark,
  type ReviewPlan,
  splitIntoParts,
  partProgress,
  SPOT_QUESTION,
  spotProblem,
} from "../../../api/review";
import { ToolFrame } from "../ToolFrame";
import { ReviewMarkdown } from "./ReviewMarkdown";
import { PaperPreview } from "../../PaperPreview";
import { FoldoutPreview } from "../../FoldoutPreview";
import { ArtifactChanges } from "../../ArtifactChanges";
import { useWidePane } from "../wide";
import { DONE, TODO, tone } from "../../../shared/tone";
import { Progress } from "../../../shared/Progress";
import type { ToolSurfaceProps } from "../registry";
import { apiErrorText } from "../../../api/errorText";

/**
 * Review —— 审一遍印记交出来的东西。
 *
 * 产品负责人 2026-09-01：能审的前提是**看得懂**。所以原文照原样铺开，划出来的
 * 句子就高亮在它本来的位置上——一句话离开上下文就没法判断对不对，而判断对不对
 * 正是审阅这件事本身。
 *
 * 她选中任何一段都能就地问一句，那一问会开一条独立的会话线。这是「click on any
 * word to have a new chat line」，也是这块界面上最该顺手的动作：选中，点一下，
 * 完了。
 *
 * 印记自己写的"我猜了什么 / 哪里还不对"钉在最上面。她要审的是一份**承认了自己
 * 弱点**的东西，不是一份看起来什么都对的东西。
 */
export function Review({ projectId, tool, onFinish, onOpenSession, onClose }: ToolSurfaceProps) {
  const [artifacts, setArtifacts] = useState<Artifact[]>([]);
  const [pickedId, setPickedId] = useState<string | null>(null);
  const [plan, setPlan] = useState<ReviewPlan>({ marks: [], dimensions: [] });
  const [openMark, setOpenMark] = useState<string | null>(null);
  const [selection, setSelection] = useState("");
  // 🚨 「找茬」这一轮：印记承认的那几处先盖住，她自己先找。
  //
  // 产品负责人 2026-09-03：「review games … gamification, interaction!」。
  // 审一份东西最省力的走法是从头读到尾然后点通过；而这件工具真正要教的是
  // 铁律①那一条——她判断 AI，不是反过来。所以把顺序倒过来：先让她找，再对答案。
  // 「你找出了印记自己都没提的一处」是这件事最好的时刻，也只有先藏起来才可能发生。
  const [spotting, setSpotting] = useState("");   // 正在写理由的那一段引文
  const [spotWhy, setSpotWhy] = useState("");
  const [revealed, setRevealed] = useState(false);
  const [verdictWhy, setVerdictWhy] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [finishing, setFinishing] = useState(false);
  const finishingRef = useRef(false);
  const writes = useRef(Promise.resolve());
  const failedWrites = useRef(new Map<string, unknown>());

  const pending = pendingArtifacts(artifacts);
  const artifact = pending.find((a) => a.id === pickedId) ?? pending[0] ?? null;

  useEffect(() => {
    void (async () => {
      const all = await listArtifacts(projectId);
      setArtifacts(all);
    })();
  }, [projectId]);

  const loadPlan = useCallback(async () => {
    if (!artifact) return;
    setPlan(await getReview(projectId, artifact.id));
  }, [projectId, artifact]);

  useEffect(() => {
    void loadPlan();
  }, [loadPlan]);

  async function ask() {
    const quote = selection.trim();
    if (!artifact || !quote) return;
    try {
      // 🚨 服务端在这一个请求里连支线一起开好了，并把 id 返回来。
      //
      // 以前这里把 id 扔了，只刷新一下划线列表——于是她选中一句话、点「问问
      // 这一句」，得到的是一张写着服务端默认问题的卡片和一个空输入框。她向
      // AI 提了个问题，产品把这个问题原样退回来让她自己答。那是教她"这里的
      // AI 是假的"最快的办法。
      const { sessionId } = await askAbout(projectId, artifact.id, quote);
      setSelection("");
      window.getSelection()?.removeAllRanges();
      onOpenSession(sessionId);
    } catch (err) {
      setError(apiErrorText(err));
    }
  }

  function enqueueAnswer(id: string, save: () => Promise<void>) {
    // Serialize edits so an older response cannot overwrite a newer answer.
    writes.current = writes.current.then(async () => {
      try {
        await save();
        failedWrites.current.delete(id);
      } catch (err) {
        failedWrites.current.set(id, err);
        setError(apiErrorText(err));
      }
    });
    return writes.current;
  }

  function saveMark(id: string, answer: string) {
    return enqueueAnswer(id, async () => {
      const got = await answerMark(projectId, id, answer);
      setPlan((p) => ({ ...p, marks: p.marks.map((m) => (m.id === got.id ? got : m)) }));
    });
  }

  function saveDimension(id: string, answer: string) {
    return enqueueAnswer(id, async () => {
      const got = await answerDimension(projectId, id, answer);
      setPlan((p) => ({
        ...p,
        dimensions: p.dimensions.map((d) => (d.id === got.id ? got : d)),
      }));
    });
  }

  async function finish(verdict: "kept" | "revise" | "dropped") {
    if (!artifact || finishingRef.current) return;
    finishingRef.current = true;
    setFinishing(true);
    setError(null);
    try {
      // Blur saves the answer first. Preserve the student's explicit verdict:
      // answering a review question does not itself request a revision.
      await writes.current;
      if (failedWrites.current.size) throw failedWrites.current.values().next().value;
      const saved = await getReview(projectId, artifact.id);
      setPlan(saved);
      await settleArtifact(projectId, artifact.id, verdict, verdictWhy.trim());
      // 她写的结论就是她的话，原样带走。
      onFinish({ artifactId: artifact.id, verdict }, verdictWhy.trim());
    } catch (err) {
      setError(apiErrorText(err));
    } finally {
      finishingRef.current = false;
      setFinishing(false);
    }
  }

  async function refreshCurrentSite() {
    if (!artifact || finishingRef.current) return;
    finishingRef.current = true;
    setFinishing(true);
    setError(null);
    try {
      await writes.current;
      if (failedWrites.current.size) throw failedWrites.current.values().next().value;
      const all = await refreshSiteReview(projectId, artifact.id);
      setArtifacts(all);
      setPickedId(pendingArtifacts(all).find((a) => a.kind === "site" && !a.stale)?.id ?? null);
      setPlan({ marks: [], dimensions: [] });
      setVerdictWhy("");
      setSelection("");
      setSpotting("");
      setOpenMark(null);
    } catch (err) { setError(apiErrorText(err)); }
    finally { finishingRef.current = false; setFinishing(false); }
  }

  const paragraphs = useMemo(
    () => (artifact?.payload.body ?? "").split(/\n{2,}/).filter((p) => p.trim()),
    [artifact],
  );


  /**
   * 她有没有留下意见。
   *
   * 回答可能是确认无问题，也可能提出修改；是否修改由学生明确选择。
   */
  const hasComments =
    plan.marks.some((m) => m.answer.trim()) ||
    plan.dimensions.some((d) => d.answer.trim());

  /**
   * 🚨 有东西可审的时候，什么都不缺。
   *
   * 原来这里是 `reviewTodo(hasComments, verdictWhy)`，它在她既没留意见、也没写
   * 重做方向的时候返回「结论」——而 ToolFrame 的 todo 非空就会把完成按钮禁掉。
   * 于是「审核通过」这条路被锁死了：她读完觉得没问题、想直接通过，按钮是灰的；
   * 要解锁，得先去「重做的方向」里编一个理由——那个框问的恰恰是打回重做的理由。
   * 界面上同时写着「可以直接通过」，而那正是她唯一做不到的事。
   *
   * 通过本身就是她的判断，不需要再打一行字来证明；打回重做要给方向，那道门槛
   * 在下面那个按钮上（disabled={!verdictWhy.trim()}），不该再拦一次整个工具。
   */
  const todo = artifact?.stale ? "主页已更新，请刷新审核" : artifact ? "" : "暂时没有需要审核的内容";
  const { wide } = useWidePane();
  // 划出来的句子在正文里的编号，1 开始。正文里的角标和下面那条问题靠它对上。
  const visibleMarks = useMemo(() => revealed ? plan.marks : plan.marks.filter((m) => m.mine), [revealed, plan.marks]);
  const aiMarkCount = plan.marks.filter((m) => !m.mine).length;
  const parts = useMemo(() => splitIntoParts(paragraphs, visibleMarks), [paragraphs, visibleMarks]);

  // 她自己找出来的那几处（question 是那句固定标签）。
  const mySpots = useMemo(
    () => plan.marks.filter((m) => m.mine && m.question === SPOT_QUESTION),
    [plan.marks],
  );

  // 揭晓与否是"这个人看到哪儿了"，不是项目数据——存本地就够，也不该同步给别人。
  const revealKey = artifact ? `pbl:revealed:${artifact.id}` : "";
  useEffect(() => {
    if (!revealKey) return;
    try {
      setRevealed(window.localStorage.getItem(revealKey) === "1");
    } catch {
      // 无痕窗口读不到就当没揭晓过，不该因此崩掉整屏。
      setRevealed(false);
    }
  }, [revealKey]);

  function reveal() {
    setRevealed(true);
    try {
      window.localStorage.setItem(revealKey, "1");
    } catch {
      /* 记不住就只这一次有效 */
    }
  }

  async function spot() {
    if (!artifact) return;
    const quote = spotting.trim();
    const why = spotWhy.trim();
    if (!quote || !why) return;
    try {
      await spotProblem(projectId, artifact.id, quote, why);
      setSpotting("");
      setSpotWhy("");
      setSelection("");
      await loadPlan();
    } catch (err) {
      setError(apiErrorText(err));
    }
  }
  // 战绩：划出来的句子 + 该看的几个方面，一起算。两样都是"她做过的判断"。
  const scored = useMemo(() => {
    const all = [...plan.marks, ...plan.dimensions];
    return { done: all.filter((x) => x.answer.trim() !== "").length, total: all.length };
  }, [plan.marks, plan.dimensions]);

  function renderParagraphs(list: string[]) {
    return <ReviewMarkdown text={list.join("\n\n")} marks={visibleMarks}
      onOpen={(id) => setOpenMark(openMark === id ? null : id)} />;
  }

  const markNo = useMemo(
    () => new Map(plan.marks.map((m, i) => [m.id, i + 1] as const)),
    [plan.marks],
  );

  return (
    <ToolFrame
      title={tool.label}
      task="参考审核框架，进行深度审核"
      why={tool.reason}
      todo={todo}
      finishLabel="审核通过"
      onFinish={() => void finish("kept")}
      onClose={onClose}
      busy={finishing}
    >
      <fieldset disabled={finishing} className="min-w-0">
      {error && (
        <div className="mb-2 text-mk-small" style={{ color: "var(--mk-danger)" }}>
          <Says content={errorMarkdown(error)} />
        </div>
      )}

      {artifact?.stale && <div className="mb-3 rounded-mk-md border border-mk-border p-3">
        <p className="text-mk-small text-mk-ink">主页已更新，下方是旧版审核。请刷新后检查当前页面，旧意见将保留在原版本中。</p>
        <button type="button" onClick={() => void refreshCurrentSite()} className="mt-2 rounded-mk-full border border-mk-border px-3 py-1.5 text-mk-small">刷新审核</button>
      </div>}

      {!artifact && (
        <p className="text-mk-small text-mk-muted">暂时没有需要审核的内容</p>
      )}

      {artifact && (
        <>
          {pending.length > 1 && (
            <div className="mb-3 flex flex-wrap gap-1">
              {pending.map((a) => (
                <button
                  key={a.id}
                  type="button"
                  onClick={() => setPickedId(a.id)}
                  aria-pressed={a.id === artifact.id}
                  className="rounded-mk-full px-2.5 py-1 text-mk-small"
                  style={
                    a.id === artifact.id
                      ? { background: "var(--mk-accent-500)", color: "#fff" }
                      : { color: "var(--mk-secondary)", border: "1px solid var(--mk-border)" }
                  }
                >
                  {a.title || "未命名"} · {new Date(a.createdAt).toLocaleString()}{a.stale ? "（已过期）" : ""}
                </button>
              ))}
            </div>
          )}

          <ArtifactChanges artifact={artifact} showMetadata={revealed} previousArtifact={artifacts.find(a => a.id === (artifact.payload.baseArtifactId ?? artifact.payload.replacesArtifactId))} />
          <PaperPreview key={`paper-${artifact.id}`} artifact={artifact} />
          <FoldoutPreview key={artifact.id} artifact={artifact} />

          {/* 🚨 审了几处，一眼看得见。
              产品负责人 2026-09-03：「gamification, interaction!」——审一份文档
              本来是件没有反馈的事：读完了也不知道自己审得算不算数。给它一个
              进度和一个终点，她才有理由把四处都答完，而不是划到底点通过。 */}
          {scored.total > 0 && (
            <div
              className="mb-3 flex items-center gap-3 rounded-mk-md px-3 py-2.5"
              style={{
                background:
                  scored.done === scored.total ? DONE.bg : "var(--mk-paper)",
              }}
            >
              <Progress done={scored.done} total={scored.total} size={44} />
              <div className="min-w-0">
                <p className="text-mk-small font-semibold text-mk-ink">
                  {scored.done === scored.total
                    ? "已全部审核"
                    : `已审 ${scored.done} / ${scored.total} 处`}
                </p>
                <p className="text-mk-small text-mk-muted">
                  {scored.done === scored.total
                    ? "请下结论：审核通过，或重新执行任务。"
                    : "每一部分附有一个审核问题，请逐项回答。"}
                </p>
              </div>
            </div>
          )}

          {/* 印记先承认自己的弱点。她要审的是这个，不是一份看起来什么都对的东西。
              🚨 原来这一块是一列灰色小字，每行前面还顶着「猜测内容：」「可能出错：」
              ——最该被看见的两件事，长得和旁边的说明文字一模一样。产品负责人
              2026-09-03：「critical points are highlighted, or put in a colored box」。 */}
          {/* 🚨 找茬：印记承认的那几处先盖住。
              审一份东西最省力的走法是从头读到尾然后点通过。把顺序倒过来——先让
              她找，再对答案——「你找出了印记自己都没提的一处」这个时刻才可能发生，
              而那正是铁律①要的：她判断 AI。 */}
          {!revealed && (aiMarkCount > 0 || artifact.admits.length > 0 || artifact.guessed.length > 0) && (
            <div className="mb-4 rounded-mk-md px-3 py-2.5" style={{ background: tone("mist").bg }}>
              <p className="text-mk-small font-semibold" style={{ color: tone("mist").fg }}>
                自查
              </p>
              <p className="mt-0.5 text-mk-small text-mk-ink">
                印记的标注与自查说明暂未显示。
                请先自行审核：在正文中选中句子，标出你认为有问题的地方。
              </p>
              <div className="mt-2 flex items-center gap-2">
                <span className="text-mk-small text-mk-muted">已标出 {mySpots.length} 处</span>
                <button
                  type="button"
                  onClick={reveal}
                  className="rounded-mk-full px-3 py-1 text-mk-small"
                  style={{ background: tone("mist").solid, color: "var(--mk-surface)" }}
                >
                  显示印记的标注
                </button>
              </div>
            </div>
          )}

          {revealed && mySpots.length > 0 && (
            <div className="mb-3 rounded-mk-md px-3 py-2.5" style={{ background: DONE.bg }}>
              <p className="text-mk-small font-semibold" style={{ color: DONE.fg }}>
                你标出 {mySpots.length} 处 · 印记标注 {aiMarkCount} 处
              </p>
              <p className="mt-0.5 text-mk-small text-mk-ink">
                请对照两份标注。你标出而印记未提及的内容，需要向印记确认。
              </p>
            </div>
          )}

          {revealed && (
          <div className="mb-4 space-y-2">
            <Callout
              t={tone("peach")}
              title="印记的猜测"
              blurb="印记写这一版时依据的假设。假设不成立，结论也不成立。"
              items={artifact.guessed}
            />
            <Callout
              t={tone("berry")}
              title="印记存疑的地方"
              blurb="印记自己没有把握的内容，请优先审核。"
              items={artifact.admits}
            />
          </div>
          )}

          {/* 正文：一部分一部分地进去，而不是摊开一整篇。
              🚨 产品负责人 2026-09-03：「we must go into texts, instead of
              presenting a large text」。一整篇铺在那里，她能做的只有从头划到尾
              ——那是"读过了"，不是"审过了"。每一部分自带「这一部分要看什么」，
              以及落在这一部分里的问题，就地答。 */}
          {artifact.kind === "site" && artifact.payload.url && (
            <a href={artifact.payload.url} target="_blank" rel="noreferrer" className="mb-3 block text-mk-small underline">
              查看当前主页预览
            </a>
          )}
          {isDocument(artifact) ? (
            <div
              onMouseUp={() => setSelection(window.getSelection()?.toString() ?? "")}
              className="space-y-5"
            >
              {parts.map((part, pi) => {
                const at = partProgress(part);
                return (
                  <section key={pi}>
                    {part.name && (
                      <div className="mb-1.5 flex items-center gap-2">
                        <span
                          className="flex h-5 w-5 shrink-0 items-center justify-center rounded-mk-full text-[11px] font-semibold"
                          style={{
                            background: at.total > 0 && at.done === at.total ? DONE.solid : TODO.solid,
                            color: "var(--mk-surface)",
                          }}
                        >
                          {pi + 1}
                        </span>
                        <p className="text-mk-body font-semibold text-mk-ink">{part.name}</p>
                        {at.total > 0 && (
                          <span
                            className="rounded-mk-full px-1.5 text-[11px] font-semibold"
                            style={
                              at.done === at.total
                                ? {
                                    background: DONE.bg,
                                    color: DONE.solid,
                                  }
                                : { background: "var(--mk-paper)", color: "var(--mk-faint)" }
                            }
                          >
                            {at.done === at.total ? "已审核" : `${at.done}/${at.total}`}
                          </span>
                        )}
                      </div>
                    )}
                    {part.note && (
                      <p
                        className="mb-2 rounded-mk-md px-3 py-2 text-mk-small text-mk-ink"
                        style={{
                          background: tone("mist").bg,
                          color: tone("mist").fg,
                        }}
                      >
                        审核要点：{part.note}
                      </p>
                    )}
                    <div className="space-y-2">{renderParagraphs(part.paragraphs)}</div>
                    {part.marks.length > 0 && (
                      <div className="mt-2 space-y-2">
                        {part.marks.map((m) => (
                          <MarkRow
                            key={m.id}
                            no={markNo.get(m.id) ?? 0}
                            mark={m}
                            open={openMark === m.id}
                            onToggle={() => setOpenMark(openMark === m.id ? null : m.id)}
                            onSave={(a) => void saveMark(m.id, a)}
                          />
                        ))}
                      </div>
                    )}
                  </section>
                );
              })}
            </div>
          ) : artifact.kind === "image" && artifact.payload.url ? (
            <img
              src={artifact.payload.url}
              alt={artifact.title}
              className="w-full rounded-mk-md"
            />
          ) : artifact.payload.url ? (
            /* 网站要点开来看。嵌在这块三百多像素宽的面板里，看到的不是它。 */
            <a
              href={artifact.payload.url}
              target="_blank"
              rel="noreferrer"
              className="block rounded-mk-md border border-mk-border px-3 py-2.5"
            >
              <p className="text-mk-small text-mk-ink">打开这个网站看看</p>
              <p className="mt-0.5 truncate text-mk-small text-mk-faint">
                {artifact.payload.url}
              </p>
            </a>
          ) : (
            /* 走到这里说明这件成果既没有正文也没有链接——那它本来就不该被送来
               审。照实说，别让她对着一句「没有内容」猜自己该做什么。 */
            <p className="text-mk-small text-mk-muted">
              这件成果没有可以打开的内容，回到对话里跟印记说一声。
            </p>
          )}

          {/* 她划出来的那一处：写一句哪里不对。 */}
          {spotting && (
            <div className="mt-2 rounded-mk-md px-3 py-2.5" style={{ background: TODO.bg }}>
              <p className="text-mk-small text-mk-muted">「{spotting}」</p>
              <input
                autoFocus
                value={spotWhy}
                onChange={(e) => setSpotWhy(e.target.value)}
                onKeyDown={(e) => e.key === "Enter" && void spot()}
                placeholder="请说明问题"
                className="mt-1.5 w-full rounded-mk-md border border-mk-input-border bg-mk-surface px-2.5 py-1.5 text-mk-small text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent-200"
              />
              <div className="mt-2 flex gap-2">
                <button
                  type="button"
                  onClick={() => void spot()}
                  disabled={!spotWhy.trim()}
                  className="rounded-mk-full px-3 py-1 text-mk-small disabled:opacity-40"
                  style={{ background: TODO.solid, color: "var(--mk-surface)" }}
                >
                  确认标记
                </button>
                <button
                  type="button"
                  onClick={() => {
                    setSpotting("");
                    setSpotWhy("");
                  }}
                  className="text-mk-small text-mk-secondary"
                >
                  取消
                </button>
              </div>
            </div>
          )}

          {/* 选中一段之后能做两件事：自己标一处，或者问印记。 */}
          {selection.trim() && !spotting && (
            <div className="mt-2 flex gap-2">
              <button
                type="button"
                onClick={() => {
                  setSpotting(selection.trim());
                  setSpotWhy("");
                }}
                className="flex flex-1 items-center justify-center gap-1.5 rounded-mk-full py-2 text-mk-small"
                style={{ background: TODO.solid, color: "var(--mk-surface)" }}
              >
                标出问题
              </button>
            </div>
          )}

          {/* 选中就地问 */}
          {selection.trim() && !spotting && (
            <button
              type="button"
              onClick={() => void ask()}
              className="mt-2 flex w-full items-center justify-center gap-1.5 rounded-mk-full border border-mk-border py-2 text-mk-small text-mk-secondary"
            >
              <Icon icon={MessageSquareQuote} size={14} />
              问问这一句
            </button>
          )}

          {/* 划出来的句子。
              🚨 文档已经把每条问题摆在它所属的那一部分里了，这里不能再列一遍
              ——同一个问题出现两次，她答哪一个都不知道。图片和网站没有可以内联
              的正文，问题只能集中列在这里。 */}
          {!isDocument(artifact) && visibleMarks.length > 0 && (
            <div className="mt-5 space-y-2 border-t border-mk-border pt-4">
              <div className="flex items-center gap-2">
                <span className="h-3.5 w-1 rounded-mk-full" style={{ background: TODO.solid }} />
                <p className="text-mk-body font-semibold text-mk-ink">
                  印记的标注（{plan.marks.filter((m) => m.answer.trim()).length}/
                  {plan.marks.length}）
                </p>
              </div>
              {visibleMarks.map((m) => (
                <MarkRow
                  key={m.id}
                  no={markNo.get(m.id) ?? 0}
                  mark={m}
                  open={openMark === m.id}
                  onToggle={() => setOpenMark(openMark === m.id ? null : m.id)}
                  onSave={(a) => void saveMark(m.id, a)}
                />
              ))}
            </div>
          )}

          {/* 该看的几个方面 */}
          {plan.dimensions.length > 0 && (
            <div className="mt-5 space-y-2 border-t border-mk-border pt-4">
              <div className="flex items-center gap-2">
                <span
                  className="h-3.5 w-1 rounded-mk-full"
                  style={{ background: "var(--mk-accent-500)" }}
                />
                <p className="text-mk-body font-semibold text-mk-ink">审核要点</p>
              </div>
              <p className="text-mk-small text-mk-muted">
                请记录检查结果；有问题时说明修改要求，没有问题时说明判断依据。
              </p>
              {plan.dimensions.map((d) => (
                <div key={d.id} className="rounded-mk-md border border-mk-border px-3 py-2">
                  <p className="text-mk-small text-mk-ink">{d.prompt}</p>
                  {d.why && <p className="mt-0.5 text-mk-small text-mk-muted">{d.why}</p>}
                  <AnswerBox value={d.answer} onSave={(a) => void saveDimension(d.id, a)} />
                </div>
              ))}
            </div>
          )}

          {/* 结论。三档的门槛不一样，见 pbl_artifacts.go 里那段注释。 */}
          <div className="mt-4 border-t border-mk-border pt-3">
            {hasComments ? (
              <>
                <p className="text-mk-small text-mk-muted">审核记录已保存。需要修改时请执行修改；确认没有问题时可以审核通过。</p>
                <button type="button" onClick={() => void finish("revise")} className="mt-2 w-full rounded-mk-full border border-mk-border py-1.5 text-mk-small text-mk-secondary">执行修改</button>
              </>
            ) : (
              <>
                <p className="text-mk-small text-mk-muted">
                  你未留下修改意见。可以审核通过，也可以要求重新执行。
                </p>
                <label className="mt-3 block text-mk-small text-mk-secondary">
                  重做的方向
                </label>
                <textarea
                  value={verdictWhy}
                  onChange={(e) => setVerdictWhy(e.target.value)}
                  rows={2}
                  placeholder="请说明重新执行的方向"
                  className="mt-1.5 w-full resize-none rounded-mk-md border border-mk-input-border bg-mk-surface px-2.5 py-2 text-mk-small text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent-200"
                />
                {/* 🚨 重做必须给方向：不说清往哪儿改，印记只能再猜一遍，
                    她会拿到第二份同样不对的东西。 */}
                <button
                  type="button"
                  disabled={!verdictWhy.trim()}
                  onClick={() => void finish("dropped")}
                  className="mt-2 w-full rounded-mk-full border border-mk-border py-1.5 text-mk-small text-mk-secondary disabled:opacity-40"
                >
                  重新执行任务
                </button>
              </>
            )}
          </div>
        </>
      )}
      </fieldset>
    </ToolFrame>
  );
}

/**
 * Callout —— 一件必须被看见的事，放进一个带颜色的盒子。
 *
 * 产品负责人 2026-09-03：「critical points are highlighted, or put in a colored
 * box」。原来印记的猜测和存疑是一列灰色小字，和旁边的说明文字长得一模一样，
 * 于是最该先读的两件事最容易被跳过。
 *
 * 颜色用 color-mix 兑出淡底，不用 Tailwind 的透明度语法——mk 令牌是裸 CSS 变量，
 * 那套语法一个字节的 CSS 都不会生成（见 [[tailwind-mk-token-alpha-trap]]）。
 * 这里的 tone 是写死的十六进制，所以直接兑就行。
 */
function Callout({
  t,
  title,
  blurb,
  items,
}: {
  t: { solid: string; bg: string; fg: string };
  title: string;
  blurb: string;
  items: string[];
}) {
  if (items.length === 0) return null;
  return (
    <div className="rounded-mk-md px-3 py-2.5" style={{ background: t.bg }}>
      <p className="text-mk-small font-semibold" style={{ color: t.fg }}>
        {title}
      </p>
      <p className="mt-0.5 text-mk-small text-mk-muted">{blurb}</p>
      <ul className="mt-1.5 space-y-1">
        {items.map((x, i) => (
          <li key={i} className="flex gap-1.5 text-mk-small text-mk-ink">
            <span style={{ color: t.solid }}>·</span>
            <span>{x}</span>
          </li>
        ))}
      </ul>
    </div>
  );
}

function MarkRow({
  no,
  mark,
  open,
  onToggle,
  onSave,
}: {
  /** 正文里那个角标的号，和这一条对上。 */
  no: number;
  mark: ReviewMark;
  open: boolean;
  onToggle: () => void;
  onSave: (answer: string) => void;
}) {
  const done = mark.answer.trim() !== "";
  return (
    <div
      className="rounded-mk-md border px-3 py-2"
      style={{
        borderColor: done ? DONE.solid : "var(--mk-border)",
        background: done ? DONE.bg : "transparent",
      }}
    >
      <button type="button" onClick={onToggle} className="flex w-full gap-2 text-left">
        <span
          className="mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center rounded-mk-full text-[11px] font-semibold"
          style={{ background: done ? DONE.solid : TODO.solid, color: "var(--mk-surface)" }}
        >
          {no}
        </span>
        <span className="min-w-0 flex-1">
        {/* 🚨 这是哪一部分、这一部分该注意什么。
            设计文档要的是「explanations for each part so that we know what we
            should care about in each part」——这两个字段一直在库里、在 DTO 里，
            前端一次也没渲染过。少了它，她面对的是一串没有出处的问句。 */}
        {mark.part && (
          <p className="text-mk-label uppercase text-mk-faint">{mark.part}</p>
        )}
        {mark.partNote && (
          <p className="text-mk-small text-mk-secondary">{mark.partNote}</p>
        )}
        {mark.quote && (
          <p className="mt-0.5 text-mk-small text-mk-muted">「{mark.quote}」</p>
        )}
        <p className="mt-0.5 text-mk-small font-medium text-mk-ink">{mark.question}</p>
        {mark.mine && <p className="mt-0.5 text-mk-small text-mk-faint">你提的问题</p>}
        </span>
      </button>
      {open && <AnswerBox value={mark.answer} onSave={onSave} />}
      {!open && mark.answer && (
        <p className="mt-1 text-mk-small text-mk-secondary">{mark.answer}</p>
      )}
    </div>
  );
}

function AnswerBox({ value, onSave }: { value: string; onSave: (v: string) => void }) {
  const [text, setText] = useState(value);
  useEffect(() => setText(value), [value]);
  return (
    <textarea
      value={text}
      onChange={(e) => setText(e.target.value)}
      onBlur={() => text.trim() !== value.trim() && onSave(text.trim())}
      rows={2}
      placeholder="你的想法"
      className="mt-1.5 w-full resize-none rounded-mk-sm border border-mk-input-border bg-mk-surface px-2 py-1.5 text-mk-small text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent-200"
    />
  );
}

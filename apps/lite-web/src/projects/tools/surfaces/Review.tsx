import { useCallback, useEffect, useMemo, useState } from "react";
import { MessageSquareQuote } from "lucide-react";
import { Icon } from "@/ui";
import {
  isDocument,
  listArtifacts,
  pendingArtifacts,
  settleArtifact,
  type Artifact,
} from "../../../api/artifacts";
import {
  answerDimension,
  answerMark,
  askAbout,
  getReview,
  splitByMarks,
  type ReviewMark,
  type ReviewPlan,
  splitIntoParts,
  partProgress,
} from "../../../api/review";
import { ToolFrame } from "../ToolFrame";
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
  const [verdictWhy, setVerdictWhy] = useState("");
  const [error, setError] = useState<string | null>(null);

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

  async function saveMark(id: string, answer: string) {
    try {
      const got = await answerMark(projectId, id, answer);
      setPlan((p) => ({ ...p, marks: p.marks.map((m) => (m.id === got.id ? got : m)) }));
    } catch (err) {
      setError(apiErrorText(err));
    }
  }

  async function saveDimension(id: string, answer: string) {
    try {
      const got = await answerDimension(projectId, id, answer);
      setPlan((p) => ({
        ...p,
        dimensions: p.dimensions.map((d) => (d.id === got.id ? got : d)),
      }));
    } catch (err) {
      setError(apiErrorText(err));
    }
  }

  async function finish(verdict: "kept" | "revise" | "dropped") {
    if (!artifact) return;
    try {
      await settleArtifact(projectId, artifact.id, verdict, verdictWhy.trim());
      // 她写的结论就是她的话，原样带走。
      onFinish({ artifactId: artifact.id, verdict }, verdictWhy.trim());
    } catch (err) {
      setError(apiErrorText(err));
    }
  }

  const paragraphs = useMemo(
    () => (artifact?.payload.body ?? "").split(/\n{2,}/).filter((p) => p.trim()),
    [artifact],
  );


  /**
   * 她有没有留下意见。
   *
   * 产品负责人 2026-09-02：留了意见，结论就只有一个——执行修改；没留意见，才谈
   * 得上审核通过或者重新执行。所以这一行决定底下出现哪几个按钮。
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
  const todo = artifact ? "" : "暂时没有需要审核的内容";
  const { wide } = useWidePane();
  // 划出来的句子在正文里的编号，1 开始。正文里的角标和下面那条问题靠它对上。
  const parts = useMemo(() => splitIntoParts(paragraphs, plan.marks), [paragraphs, plan.marks]);
  // 战绩：划出来的句子 + 该看的几个方面，一起算。两样都是"她做过的判断"。
  const scored = useMemo(() => {
    const all = [...plan.marks, ...plan.dimensions];
    return { done: all.filter((x) => x.answer.trim() !== "").length, total: all.length };
  }, [plan.marks, plan.dimensions]);

  /** 一段正文，划出来的地方高亮 + 角标。分段渲染和整篇渲染共用这一段。 */
  function renderParagraphs(list: string[]) {
    return list.map((text, i) => (
      <p
        key={i}
        className={
          wide ? "text-mk-body leading-[1.9] text-mk-ink" : "text-mk-small leading-relaxed text-mk-ink"
        }
      >
        {splitByMarks(text, plan.marks).map((seg, j) =>
          seg.mark ? (
            <mark
              key={j}
              onClick={() => setOpenMark(openMark === seg.mark!.id ? null : seg.mark!.id)}
              className="cursor-pointer rounded-mk-sm px-0.5"
              style={{
                background: seg.mark.answer.trim()
                  ? DONE.bg
                  : TODO.bg,
                color: "var(--mk-ink)",
                boxShadow: seg.mark.answer.trim()
                  ? `inset 0 -2px 0 ${DONE.solid}`
                  : `inset 0 -2px 0 ${TODO.solid}`,
              }}
            >
              {seg.text}
              <sup
                className="ml-0.5 rounded-mk-full px-1 text-[10px] font-semibold"
                style={{
                  background: seg.mark.answer.trim() ? DONE.solid : TODO.solid,
                  color: "var(--mk-surface)",
                }}
              >
                {markNo.get(seg.mark.id) ?? "?"}
              </sup>
            </mark>
          ) : (
            <span key={j}>{seg.text}</span>
          ),
        )}
      </p>
    ));
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
      finishLabel={hasComments ? "执行修改" : "审核通过"}
      onFinish={() => void finish(hasComments ? "revise" : "kept")}
      onClose={onClose}
    >
      {error && (
        <p className="mb-2 text-mk-small" style={{ color: "var(--mk-danger)" }}>
          {error}
        </p>
      )}

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
                  className="rounded-mk-full px-2.5 py-1 text-mk-small"
                  style={
                    a.id === artifact.id
                      ? { background: "var(--mk-accent-500)", color: "#fff" }
                      : { color: "var(--mk-secondary)", border: "1px solid var(--mk-border)" }
                  }
                >
                  {a.title || "未命名"}
                </button>
              ))}
            </div>
          )}

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
                    ? "全部审过了"
                    : `审了 ${scored.done} / ${scored.total} 处`}
                </p>
                <p className="text-mk-small text-mk-muted">
                  {scored.done === scored.total
                    ? "可以下结论了：通过，或者让它重做。"
                    : "印记在每一部分埋了一个问题。答一个，那一处就变绿。"}
                </p>
              </div>
            </div>
          )}

          {/* 印记先承认自己的弱点。她要审的是这个，不是一份看起来什么都对的东西。
              🚨 原来这一块是一列灰色小字，每行前面还顶着「猜测内容：」「可能出错：」
              ——最该被看见的两件事，长得和旁边的说明文字一模一样。产品负责人
              2026-09-03：「critical points are highlighted, or put in a colored box」。 */}
          <div className="mb-4 space-y-2">
            <Callout
              t={tone("peach")}
              title="印记的猜测"
              blurb="这一版是踩着这些假设写的。假设不成立，下面的东西就不成立。"
              items={artifact.guessed}
            />
            <Callout
              t={tone("berry")}
              title="印记觉得可能不对的地方"
              blurb="印记自己也没把握的地方，先从这里看起。"
              items={artifact.admits}
            />
          </div>

          {/* 正文：一部分一部分地进去，而不是摊开一整篇。
              🚨 产品负责人 2026-09-03：「we must go into texts, instead of
              presenting a large text」。一整篇铺在那里，她能做的只有从头划到尾
              ——那是"读过了"，不是"审过了"。每一部分自带「这一部分要看什么」，
              以及落在这一部分里的问题，就地答。 */}
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
                            {at.done === at.total ? "这一部分审过了" : `${at.done}/${at.total}`}
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
                        这一部分要看的：{part.note}
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

          {/* 选中就地问 */}
          {selection.trim() && (
            <button
              type="button"
              onClick={() => void ask()}
              className="mt-2 flex w-full items-center justify-center gap-1.5 rounded-mk-full py-2 text-mk-small text-white"
              style={{ background: "var(--mk-accent-500)" }}
            >
              <Icon icon={MessageSquareQuote} size={14} />
              问问这一句
            </button>
          )}

          {/* 划出来的句子。
              🚨 文档已经把每条问题摆在它所属的那一部分里了，这里不能再列一遍
              ——同一个问题出现两次，她答哪一个都不知道。图片和网站没有可以内联
              的正文，问题只能集中列在这里。 */}
          {!isDocument(artifact) && plan.marks.length > 0 && (
            <div className="mt-5 space-y-2 border-t border-mk-border pt-4">
              <div className="flex items-center gap-2">
                <span className="h-3.5 w-1 rounded-mk-full" style={{ background: TODO.solid }} />
                <p className="text-mk-body font-semibold text-mk-ink">
                  划出来的句子（{plan.marks.filter((m) => m.answer.trim()).length}/
                  {plan.marks.length}）
                </p>
              </div>
              {plan.marks.map((m) => (
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
                审这份东西非看不可的几个方面。答完其中任何一条，结论就是「执行修改」。
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
              <p className="text-mk-small text-mk-muted">
                你已经留下了意见，印记会照着改。底下点「执行修改」。
              </p>
            ) : (
              <>
                <p className="text-mk-small text-mk-muted">
                  你没有留下修改意见。可以直接通过，也可以让它重做。
                </p>
                <label className="mt-3 block text-mk-small text-mk-secondary">
                  重做的方向
                </label>
                <textarea
                  value={verdictWhy}
                  onChange={(e) => setVerdictWhy(e.target.value)}
                  rows={2}
                  placeholder="要重做的话，请说清楚往哪个方向"
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


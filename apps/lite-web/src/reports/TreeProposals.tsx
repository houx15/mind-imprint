import { useCallback, useEffect, useState } from "react";
import { Check, Loader2, X } from "lucide-react";
import {
  decideInterestProposal,
  fetchInterestProposals,
  type InterestProposal,
} from "../api/interest";
import { useAlive } from "../shared/useAlive";

/**
 * TreeProposals —— 报告最后那一节：这一篇读完（写完）之后，可以往你树上加的词。
 *
 * # 为什么这一节存在
 *
 * 在这之前，采集出来的词是**默默种上树的**。产品负责人 2026-09-07：
 *
 *   > we don't ask students to 收进我的树 here. we only invite students to read
 *   > now, or read later. after reading finished, we would get a report, on that
 *   > we can propose several keywords, that students can agree to add to their
 *   > tree
 *
 * 这棵树的整个说法是「这就是你的模型」。一个她没点过头的模型，只是我们对她的
 * 记录 —— 树上那个「你凭什么这么说我」的问题，从这一节开始有了她自己给的答案。
 *
 * # 每一条都带着她自己的那句话
 *
 * 一个候选词只写「电池」，她没有办法判断该不该认。所以每条都摆出 evidence ——
 * 她自己写下的、让这个词被提出来的那一句。**这和树上抽屉里的证据是同一件东西**，
 * 只是提前到了她做决定之前。
 *
 * # 三个状态，别混
 *
 *  - `pending`：采集还没跑完（后台队列，完成时入队 + 每两分钟扫尾）。要说
 *    「处理中」，并且过一会儿自己再问一次 —— 她不该为了看到这一节去刷新页面。
 *  - 采完了，一个候选都没有：**整节不显示**。一篇很薄的阅读采不出词是正常结果，
 *    摆一个「暂无」的空标题是拿版面说一件不值得说的事。
 *  - 有候选：摆出来等她点。
 *
 * # 轮询会停
 *
 * 最多问 `MAX_POLLS` 次。采集也可能一个词都采不出来（那时 `pending` 会翻成
 * false），但如果队列真的堵住了，一个永远在转的圈比一句「这次没有」糟得多。
 */

/** 隔多久问一次采集跑完了没有。 */
const POLL_MS = 4000;
/** 最多问几次。之后就停在「没有候选词」上，不再转圈。 */
const MAX_POLLS = 8;

export function TreeProposals({ atomId }: { atomId: string }) {
  const [rows, setRows] = useState<InterestProposal[] | null>(null);
  const [pending, setPending] = useState(true);
  const [polls, setPolls] = useState(0);
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState("");
  const alive = useAlive();

  const load = useCallback(async () => {
    try {
      const res = await fetchInterestProposals(atomId);
      if (!alive.current) return;
      setRows(res.proposals);
      setPending(res.pending);
    } catch {
      // 报告已经在她眼前了，为一节她没要求过的东西弹一条错误更糟。停在
      // 「没有候选词」上，整节不显示。
      if (alive.current) {
        setRows([]);
        setPending(false);
      }
    }
  }, [atomId, alive]);

  useEffect(() => {
    setRows(null);
    setPending(true);
    setPolls(0);
    void load();
  }, [atomId, load]);

  useEffect(() => {
    if (!pending || polls >= MAX_POLLS) return;
    const t = window.setTimeout(() => {
      setPolls((n) => n + 1);
      void load();
    }, POLL_MS);
    return () => window.clearTimeout(t);
  }, [pending, polls, load]);

  async function decide(p: InterestProposal, accept: boolean) {
    setBusy(p.interestId);
    setError("");
    try {
      await decideInterestProposal(atomId, p.interestId, accept);
      if (!alive.current) return;
      setRows((list) =>
        (list ?? []).map((r) =>
          r.interestId === p.interestId ? { ...r, decided: true, accepted: accept } : r,
        ),
      );
    } catch (e: unknown) {
      // 动词 + 失败，再接后台原话（AGENTS.md 界面文案 §8）。绝不静默地当成功 ——
      // 一个显示成已加入、其实没上树的按钮，是这一节最糟的谎。
      if (alive.current) setError(e instanceof Error ? e.message : String(e));
    } finally {
      if (alive.current) setBusy(null);
    }
  }

  if (rows === null && pending) return <PendingLine />;
  const list = rows ?? [];
  if (list.length === 0) return pending ? <PendingLine /> : null;

  const open = list.filter((p) => !p.decided);
  const taken = list.filter((p) => p.decided && p.accepted);

  return (
    <section className="rounded-mk-lg border border-mk-border bg-mk-surface p-5">
      <h3 className="text-mk-h3 text-mk-ink">可以加进你的兴趣树</h3>
      <p className="mt-1 text-mk-small leading-[1.8] text-mk-muted">
        {open.length > 0
          ? "这几个词来自你在这一篇里写下的话。加进去的会出现在你的树上，并且带着下面这句作为来源。"
          : "这一篇的词你都决定过了。"}
      </p>

      <ul className="mt-4 space-y-2.5">
        {list.map((p) => (
          <li
            key={p.interestId}
            className="rounded-mk-md border border-mk-border p-4"
            style={{ opacity: p.decided && !p.accepted ? 0.55 : 1 }}
          >
            <div className="flex flex-wrap items-start justify-between gap-3">
              <div className="min-w-0">
                <strong className="text-mk-body-lg text-mk-ink">{p.zh}</strong>
                <span className="ml-2 font-mono text-mk-small text-mk-muted">{p.en}</span>
                {p.note ? (
                  <p className="mt-1 text-mk-small leading-[1.75] text-mk-secondary">{p.note}</p>
                ) : null}
              </div>

              {p.decided ? (
                <span className="shrink-0 text-mk-small text-mk-muted">
                  {p.accepted ? "已加入" : "跳过"}
                </span>
              ) : (
                <div className="flex shrink-0 items-center gap-2">
                  <button
                    type="button"
                    disabled={busy !== null}
                    onClick={() => void decide(p, true)}
                    className="inline-flex items-center gap-1.5 rounded-mk-full px-3.5 py-1.5
                               text-mk-small font-semibold text-white transition hover:opacity-90
                               disabled:opacity-45"
                    style={{ background: "var(--mk-accent-500)" }}
                  >
                    {busy === p.interestId ? (
                      <Loader2 size={13} className="animate-spin" />
                    ) : (
                      <Check size={13} strokeWidth={2.2} />
                    )}
                    加入
                  </button>
                  <button
                    type="button"
                    disabled={busy !== null}
                    onClick={() => void decide(p, false)}
                    className="inline-flex items-center gap-1.5 rounded-mk-full border border-mk-border
                               px-3.5 py-1.5 text-mk-small text-mk-secondary transition
                               hover:bg-mk-accent-50 disabled:opacity-45"
                  >
                    <X size={13} strokeWidth={2.2} />
                    不用
                  </button>
                </div>
              )}
            </div>

            {/* 她自己写的那一句。没有它，这就是一句「猜你喜欢」。 */}
            {p.evidence ? (
              <p className="mt-3 border-l-2 border-mk-accent pl-3 text-mk-small italic leading-[1.75] text-mk-secondary">
                「{p.evidence}」
                <span className="mt-1 block not-italic text-[11px] text-mk-muted">你自己写的</span>
              </p>
            ) : null}
          </li>
        ))}
      </ul>

      {taken.length > 0 ? (
        <p className="mt-3 text-mk-small text-mk-muted">
          加进去的词已经在「探索 · 我的兴趣树」上了。
        </p>
      ) : null}

      {error ? (
        <p className="mt-3 rounded-mk-md border border-mk-danger p-3 text-mk-small leading-[1.8] text-mk-danger">
          操作失败：{error}
        </p>
      ) : null}
    </section>
  );
}

function PendingLine() {
  return (
    <p className="text-mk-small text-mk-muted" role="status" aria-live="polite">
      正在从你写下的话里找可以加进树的词
    </p>
  );
}

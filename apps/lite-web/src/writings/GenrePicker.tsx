import { useEffect, useState } from "react";
import { getWritingGenre, putWritingGenre, type WritingGenreState } from "../api/writingRoom";
import { apiErrorText } from "../api/errorText";

/**
 * GenrePicker —— 「这一篇按什么文体在教」，以及她自己改一种。
 *
 * # 为什么有这一块（2026-09-23）
 *
 * 产品负责人：「for writing, maybe we need to let the students select/talk with
 * ai about what genre they are going to write. sometimes they are writing a
 * 记叙文, sometimes 散文, sometimes 议论文, sometimes 书信.」
 *
 * # 🚨 它不是进门那一步的文体单选
 *
 * `writing_setup.go` 的文件头写着「**没有文体单选**——这是产品的明确要求：
 * 学生未必知道「文体」是什么意思，与其让她在一个她读不懂的词上做选择，
 * 不如让她用自己的话再说两句」。那条理由今天照样成立，设定弹窗一个字没动。
 *
 * 这一块解决的是另一件事：印记**本来就在按某一种文体教**（推断出来的），
 * 只是从来没说出口，也没给她一个说「不对，我写的是一封信」的地方。
 * 所以它先陈述、再给出口：
 *
 *   印记按 议论文 在教这一篇 · 换一种
 *
 * 每一种后面跟的是「什么时候选它」而不是定义（服务端给的 blurb）——
 * 「记叙文」三个字她未必读得懂，「写一件真实发生过的事」读得懂。
 * 这是对那条老理由的正面回答：问题从来不是「不该让她选」，
 * 是「不该拿一个她读不懂的词让她选」。
 *
 * # 「是推断的」和「是她定的」要分开说
 *
 * 推断的时候说「印记按 X 在教」；她定过之后说「你定的是 X」。
 * 把推断说成是她的选择，是替她做主之后再赖给她。
 *
 * # 文案（AGENTS.md 界面文案规则）
 *
 * 标签是名词、按钮写「做什么」，不写「就这么定」这类口语起手式；
 * 也不铺垫、不替她减压（规则 7）—— 没有「不用想太多，随便选一个」。
 */
export function GenrePicker({
  writingId,
  /** 图变了就重取一次：板上摆出议论文的骨架之后，推断出来的那一个会变。 */
  outlineVersion,
  /**
   * 服务端判定的文体，每次取到都报上来。
   *
   * 🚨 不是「她改了才报」：图上那个「这一条是什么」的菜单原来自己用
   * `outlineGenreOf(outline)` 推一遍，那是**第二个判定点**。她选了书信、
   * 而图上还摆着议论文的节点时，两边会得出不一样的答案，于是菜单里给的
   * 还是分论点。判定只留服务端那一个（writingGenreOf），这一侧把它传上去。
   */
  onGenre,
}: {
  writingId: string;
  outlineVersion: number;
  onGenre?: (genre: string) => void;
}) {
  const [state, setState] = useState<WritingGenreState | null>(null);
  const [open, setOpen] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    void getWritingGenre(writingId)
      .then((s) => {
        if (cancelled) return;
        setState(s);
        onGenre?.(s.genre);
      })
      .catch(() => {
        // 取不到就整块不显示 —— 少一行字，比摆一个说不出话的控件好。
        if (!cancelled) setState(null);
      });
    return () => {
      cancelled = true;
    };
  }, [writingId, outlineVersion]);

  if (!state) return null;

  const current = state.choices.find((c) => c.id === state.genre);
  const label = current?.label ?? "";
  if (!label) return null;

  async function pick(id: string) {
    setSaving(true);
    setError(null);
    try {
      const next = await putWritingGenre(writingId, id);
      setState(next);
      setOpen(false);
      onGenre?.(next.genre);
    } catch (err) {
      setError(apiErrorText(err));
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="flex flex-col gap-1.5" data-genre-picker>
      <div className="flex flex-wrap items-center gap-2">
        <span className="text-mk-small text-mk-secondary" data-genre-line>
          {state.chosen ? `你定的是${label}` : `印记按${label}在教这一篇`}
        </span>
        <button
          type="button"
          onClick={() => setOpen((v) => !v)}
          aria-expanded={open}
          className="rounded-mk-sm px-1.5 py-0.5 text-mk-small text-mk-accent-700 underline-offset-2 hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
        >
          {open ? "收起" : "换一种"}
        </button>
      </div>

      {open && (
        <div className="flex flex-col gap-1.5">
          {state.choices.map((c) => {
            const on = c.id === state.genre;
            return (
              <button
                key={c.id}
                type="button"
                disabled={saving}
                onClick={() => void pick(c.id)}
                aria-pressed={on}
                className="flex flex-col items-start gap-0.5 rounded-mk-md border px-2.5 py-2 text-left transition-colors duration-[120ms] ease-mk focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
                style={{
                  borderColor: on ? "var(--mk-accent-500)" : "var(--mk-border)",
                  background: on ? "var(--mk-accent-50)" : "var(--mk-surface)",
                }}
              >
                <span className="text-mk-body text-mk-ink">{c.label}</span>
                {/* 「什么时候选它」，不是定义。见文件头。 */}
                <span className="text-mk-small text-mk-muted">{c.blurb}</span>
              </button>
            );
          })}
          {/* 🚨 换文体不动她任何一个字 —— 说出来，否则她不敢点。
              图上那些节点还在，种类由她自己在图上改。 */}
          <p className="text-mk-small text-mk-faint">
            换文体只改印记怎么教，图上已有的内容一条都不会动。
          </p>
        </div>
      )}
      {error && <p className="text-mk-small text-mk-danger">{error}</p>}
    </div>
  );
}

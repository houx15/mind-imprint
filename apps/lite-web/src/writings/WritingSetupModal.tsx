import { useState } from "react";
import { Button } from "@/ui";
import { ApiError } from "../api/client";
import { setWritingSetup } from "../api/writingRoom";
import { isAssignedWriting, type Writing } from "../api/writings";
import { apiErrorText } from "../api/errorText";

/**
 * WritingSetupModal — the first thing she sees on opening a new writing.
 *
 * 两件事：语言，和目标字数。没有 文体 选择器 —— 轻量版的学生可能根本不知道
 * 「文体」是什么，一个她读不懂的下拉框比不问更糟；文体由模型从她自己的句子里
 * 判断（writing_genre.go）。
 *
 * 🚨 **这里不再有那个开放输入框。** 产品负责人 2026-09-21：
 *
 *   「I don't think we should let students type anything in the modal
 *     because it is a little strange. we can skip this 还想说点什么 input.
 *     always directly enter AI-guided journey」
 *
 * 原来第三格是「还想说点什么？」，想让她多说两句好让后面的问题贴着她的事。
 * 但那是**在对话开始之前**要她先写一段话 —— 而她推门进来本来就是要去说话的，
 * 印记的第一句就在门后面等着。把那一步留在弹窗里，等于在她想说之前先要她交作业。
 * 少了它什么都不缺：她说的每一句话仍然会到印记那里，只是从第一轮开始说。
 *
 * 这不是一道门：目标字数可以永远空着（铁律② —— 篇幅从来不是前置条件），
 * 「跳过」也能把整个弹窗打发掉。它唯一一定会做的事是盖上 `setupAt`，
 * 所以只问这一次。
 *
 * 从题库或老师那里来的那一篇，语言和字数都是定好的（2026-09-21 / 2026-09-15）：
 * 只读显示，服务端不管这个弹窗发什么都保留它自己存着的值。
 */

const LANGS: { value: "zh" | "en"; label: string; hint: string }[] = [
  { value: "zh", label: "中文", hint: "用中文写这一篇" },
  { value: "en", label: "English", hint: "Write this one in English" },
];

export function WritingSetupModal({
  writing,
  onDone,
}: {
  writing: Writing;
  onDone: (next: Writing) => void;
}) {
  const assigned = isAssignedWriting(writing);
  const [lang, setLang] = useState<"zh" | "en">(writing.lang === "en" ? "en" : "zh");
  const [words, setWords] = useState(writing.targetWords != null ? String(writing.targetWords) : "");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function submit() {
    if (saving) return;
    // An unparseable or non-positive entry is treated as "she didn't set
    // one", not as an error to scold her with. The field is optional; the
    // only wrong outcome would be blocking her over it.
    const n = Number(words.trim());
    const typed = words.trim() !== "" && Number.isFinite(n) && n > 0 ? Math.round(n) : null;
    const targetWords = assigned ? writing.targetWords : typed;
    setSaving(true);
    setError(null);
    try {
      const next = await setWritingSetup(writing.id, { lang, targetWords, note: "" });
      onDone(next);
    } catch (err) {
      setError(apiErrorText(err));
      setSaving(false);
    }
  }

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-4"
      style={{ background: "color-mix(in srgb, var(--mk-ink) 42%, transparent)" }}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-label="开始之前"
        className="flex w-full max-w-[520px] flex-col gap-5 rounded-mk-lg border border-mk-border bg-mk-surface p-6 shadow-mk-lg"
      >
        <div className="flex flex-col gap-1.5">
          <h2 className="text-mk-h2 text-mk-ink">开始之前</h2>
          {!assigned && <p className="text-mk-small text-mk-muted">语言和目标字数可在开始后修改。</p>}
        </div>

        {assigned ? (
          <div className="flex flex-col gap-2">
            <dl className="grid grid-cols-[auto_1fr] items-baseline gap-x-4 gap-y-1.5">
              <dt className="text-mk-small text-mk-secondary">语言</dt>
              <dd className="text-mk-body font-semibold text-mk-ink">{lang === "en" ? "英文" : "中文"}</dd>
              <dt className="text-mk-small text-mk-secondary">目标字数</dt>
              <dd className="text-mk-body font-semibold text-mk-ink">
                {writing.targetWords != null ? `${writing.targetWords} ${lang === "en" ? "词" : "字"}` : "—"}
              </dd>
            </dl>
            <span className="text-mk-small text-mk-muted">作业要求由老师设定</span>
          </div>
        ) : (
          <>
            <div className="flex flex-col gap-2">
              <span className="text-mk-small text-mk-secondary">写作语言</span>
              <div className="grid grid-cols-2 gap-2">
                {LANGS.map((l) => {
                  const on = lang === l.value;
                  return (
                    <button
                      key={l.value}
                      type="button"
                      aria-pressed={on}
                      onClick={() => setLang(l.value)}
                      className="flex flex-col items-start gap-0.5 rounded-mk-sm border px-3 py-2.5 text-left transition-colors duration-[120ms] ease-mk focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
                      style={
                        on
                          ? { borderColor: "var(--mk-accent-500)", background: "var(--mk-accent-50)" }
                          : { borderColor: "var(--mk-border)", background: "var(--mk-paper)" }
                      }
                    >
                      <span className="text-mk-body font-semibold" style={{ color: on ? "var(--mk-accent-700)" : "var(--mk-ink)" }}>
                        {l.label}
                      </span>
                      <span className="text-mk-small text-mk-muted">{l.hint}</span>
                    </button>
                  );
                })}
              </div>
            </div>

            <div className="flex flex-col gap-2">
              <span className="text-mk-small text-mk-secondary">目标字数（选填）</span>
              <div className="flex items-center gap-2">
                <input
                  type="number"
                  min={1}
                  value={words}
                  onChange={(e) => setWords(e.target.value)}
                  placeholder={lang === "en" ? "e.g. 500" : "比如 800"}
                  aria-label="目标字数"
                  className="w-32 rounded-mk-xs border border-mk-input-border bg-mk-paper px-3 py-2 text-mk-body text-mk-ink outline-none placeholder:text-[#B8ADA2] focus-visible:border-mk-accent focus-visible:ring-2 focus-visible:ring-mk-accent-200"
                />
                <span className="text-mk-small text-mk-muted">{lang === "en" ? "words" : "字"}</span>
              </div>
            </div>
          </>
        )}

        {error && (
          <p role="alert" className="text-mk-small text-mk-danger">
            {error}
          </p>
        )}

        <div className="flex items-center justify-end gap-2">
          <Button variant="ghost" onClick={() => void submit()} disabled={saving}>
            跳过
          </Button>
          <Button onClick={() => void submit()} loading={saving}>
            开始
          </Button>
        </div>
      </div>
    </div>
  );
}

import { useState } from "react";
import { Button } from "@/ui";
import { ApiError } from "../api/client";
import { setWritingSetup } from "../api/writingRoom";
import { isAssignedWriting, type Writing } from "../api/writings";
import { apiErrorText } from "../api/errorText";

/**
 * WritingSetupModal — the first thing she sees on opening a new writing.
 *
 * Three fields, and the third one is the interesting choice. There is no
 * 文体 (genre) selector, even though genre is what decides which skeletons
 * fit: a lite student may simply not know what 文体 means, and a dropdown of
 * words she can't parse is a worse start than no question at all. So the
 * third field is an open box — "还想说点什么都行" — and the model works out
 * the genre from her own sentences. She is never asked to name a category;
 * she is asked to keep talking.
 *
 * What this dialog is NOT: a gate. 目标字数 may be left blank forever (铁律②
 * — length is never a precondition), and 跳过 dismisses the whole thing with
 * the language defaulted. The one thing it always does is stamp `setupAt`, so
 * it asks once and never again.
 *
 * On an assigned writing the language and target are the teacher's (owner,
 * 2026-09-15): they are shown read-only, and the server keeps its stored
 * values whatever this dialog sends. The note box and the stamp work as for
 * her own writing.
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
  const [note, setNote] = useState("");
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
      const next = await setWritingSetup(writing.id, { lang, targetWords, note: note.trim() });
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
          {!assigned && <p className="text-mk-small text-mk-muted">都可以之后再改，现在随便填。</p>}
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
              <span className="text-mk-small text-mk-secondary">这篇用什么语言写？</span>
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
              <span className="text-mk-small text-mk-secondary">大概写多长？（可以不填）</span>
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

        <div className="flex flex-col gap-2">
          <span className="text-mk-small text-mk-secondary">还想说点什么？</span>
          <textarea
            value={note}
            onChange={(e) => setNote(e.target.value)}
            placeholder="比如：想写的角度、需要注意的地方……"
            aria-label="还想说点什么"
            className="min-h-[88px] w-full resize-none rounded-mk-sm border border-mk-input-border bg-mk-paper px-3 py-2 text-mk-body text-mk-ink outline-none placeholder:text-[#B8ADA2] focus-visible:border-mk-accent focus-visible:ring-2 focus-visible:ring-mk-accent-200"
          />
          <span className="text-mk-small text-mk-muted">说得越多，后面的问题越贴着你自己的事。</span>
        </div>

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

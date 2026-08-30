import { useEffect, useRef, useState } from "react";
import { Send, X } from "lucide-react";
import { useEco } from "../store";
import { Bold, Drawer, Sys, cx } from "../ui";

/**
 * 印记 — her AI, as a drawer any surface can call.
 *
 * ## Why a drawer and not a page
 * It is never the destination. She is always doing something else (looking at
 * a planet, at a keyword, at a step) and 印记 arrives ON TOP of that, keeps the
 * context in view behind it, and leaves. A full-page chat would make the
 * conversation the work, which is exactly the wrong shape for this product.
 *
 * ## The honest limit
 * The prototype has no model. Rather than fake fluency, the reply engine
 * (`data/coach.ts`) answers only what it really recognises and otherwise says
 * so — and this panel says so too, permanently, in the header strip. A canned
 * sentence dressed as a real answer is the failure mode the codebase already
 * has a rule about.
 */
export function CoachDrawer() {
  const { state, closeCoach, say } = useEco();
  const [draft, setDraft] = useState("");
  const logRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    logRef.current?.scrollTo({ top: logRef.current.scrollHeight, behavior: "smooth" });
  }, [state.coachLog.length]);

  function send(text: string) {
    const t = text.trim();
    if (!t) return;
    say(t);
    setDraft("");
  }

  const last = state.coachLog[state.coachLog.length - 1];
  const chips = last?.role === "coach" ? (last.choices ?? []) : [];

  return (
    <Drawer open={state.coachOpen} onClose={closeCoach} width={460} label="印记">
      <div className="flex items-center justify-between border-b border-mk-border px-5 py-4">
        <div className="flex items-center gap-3">
          <span
            className="flex h-9 w-9 items-center justify-center rounded-mk-full"
            style={{ background: "linear-gradient(140deg,var(--mk-accent-400),var(--mk-accent-600))" }}
          >
            <span className="text-mk-small font-bold text-white">印</span>
          </span>
          <div>
            <p className="text-mk-h3 text-mk-ink">印记</p>
            <Sys>在 {SURFACE_LABEL[state.coachSurface]} · 一次只问一个</Sys>
          </div>
        </div>
        <button
          type="button"
          onClick={closeCoach}
          className="rounded-mk-full p-2 transition-colors duration-[120ms] hover:bg-mk-accent-50
                     focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
          aria-label="关闭"
        >
          <X size={18} strokeWidth={1.8} />
        </button>
      </div>

      <div
        className="border-b border-mk-border px-5 py-2.5 text-[11px] leading-relaxed text-mk-muted"
        style={{ background: "var(--mk-paper)" }}
      >
        原型说明：这里的回答是脚本，不是真的模型。答不上来时它会直说，不会编一个听起来对的答案。
      </div>

      <div ref={logRef} className="min-h-0 flex-1 space-y-4 overflow-y-auto px-5 py-5">
        {state.coachLog.map((m) => (
          <div
            key={m.id}
            className={cx("flex", m.role === "student" ? "justify-end" : "justify-start")}
          >
            <div
              className={cx(
                "max-w-[86%] rounded-mk-lg px-4 py-3 text-mk-body leading-[1.85]",
                m.role === "student" ? "text-white" : "text-mk-ink",
              )}
              style={
                m.role === "student"
                  ? { background: "var(--mk-accent-600)" }
                  : { background: "var(--mk-paper)", border: "1px solid var(--mk-border)" }
              }
            >
              {m.text.split("\n").map((line, i) => (
                <p key={i} className={i > 0 ? "mt-2" : undefined}>
                  <Bold text={line} className="font-semibold text-mk-accent-700" />
                </p>
              ))}
            </div>
          </div>
        ))}
      </div>

      {chips.length > 0 ? (
        <div className="flex flex-wrap gap-1.5 border-t border-mk-border px-5 pt-3">
          {chips.map((c) => (
            <button
              key={c}
              type="button"
              onClick={() => send(c)}
              className="rounded-mk-full border border-mk-accent-200 bg-mk-accent-50 px-3 py-1.5 text-mk-small
                         text-mk-accent-700 transition-colors duration-[120ms] ease-mk hover:bg-mk-accent-100
                         focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
            >
              {c}
            </button>
          ))}
        </div>
      ) : null}

      <form
        className="flex items-end gap-2 px-5 py-4"
        onSubmit={(e) => {
          e.preventDefault();
          send(draft);
        }}
      >
        <textarea
          rows={2}
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter" && !e.shiftKey) {
              e.preventDefault();
              send(draft);
            }
          }}
          placeholder="说点什么…（Enter 发送）"
          className="min-w-0 flex-1 resize-none rounded-mk-md border border-mk-input-border bg-mk-surface p-3
                     text-mk-body text-mk-ink outline-none transition-colors duration-[120ms] ease-mk
                     placeholder:text-mk-faint focus:border-mk-accent-300 focus:ring-2 focus:ring-mk-accent-100"
        />
        <button
          type="submit"
          disabled={!draft.trim()}
          className="flex h-11 w-11 shrink-0 items-center justify-center rounded-mk-md bg-mk-accent text-white
                     transition-colors duration-[120ms] ease-mk hover:bg-mk-accent-600 disabled:bg-[#F0E9E1]
                     disabled:text-[#B8ADA2] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
          aria-label="发送"
        >
          <Send size={17} strokeWidth={1.9} />
        </button>
      </form>
    </Drawer>
  );
}

const SURFACE_LABEL: Record<string, string> = {
  world: "世界",
  tree: "我的树",
  projects: "项目",
  homepage: "我的主页",
  reading: "阅读",
  writing: "写作",
  step: "项目的这一步",
};

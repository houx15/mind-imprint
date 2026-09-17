import { useEffect, useRef, useState, type ReactNode } from "react";
import { Composer } from "@/studio/ai/Composer";
import { Button, Pebble } from "@/ui";
import { LiteChatMarkdown } from "../../readings/LiteChatMarkdown";
import { TeacherPage } from "../TeacherPage";
import { ChoiceArticleCard } from "./ChoiceArticleCard";
import { panelErrorLine, splitChoices } from "./workspaceLogic";
import type { Choice, Turn } from "./workspaceLogic";

// teacher/workspace/WorkspacePanel.tsx — the shell every teacher workspace
// surface sits inside: conversation on one side, the thing being made on the
// other. This task (D1's Task 5) builds ONLY the shell — it renders `turns`
// and forwards `onSend`/`onChoose`, and knows nothing about homework, the
// home page, or a parent report. `children` IS the canvas; Task 6 (the
// homework card) and the later home-page/parent-report surfaces each pass
// their own in, unchanged.
//
// Patterns borrowed from `../../readings/ReadingCoachPanel.tsx` (message
// bubbles, the thinking-dots row, the error+retry line) — NOT imported: that
// file is the reading room's, and pulling room-specific copy into a shared
// component is a mistake this repo has already made once
// ([[shared-component-module-scope-copy-2026-09-12]]).

export interface WorkspacePanelProps {
  turns: Turn[];
  busy: boolean;
  error: string | null;
  /** 0–4 entries, already clamped by the caller (`clampChoices`). Shown only
   *  under the LAST turn, and only while it is 印记's — a choice row belongs
   *  to the message that offered it, not to whatever she said after. */
  choices: Choice[];
  /** Both return whether the turn was accepted (`WorkspaceThread.run`). */
  onSend: (text: string) => boolean;
  onChoose: (choiceId: string) => boolean;
  /** The composer's text, controlled by the caller (`useWorkspaceThread`
   *  holds it so it survives this panel unmounting, and so a failed sentence
   *  can be written back into it). The caller clears it when it accepts a
   *  send of the same text. */
  composer: string;
  onComposerChange: (text: string) => void;
  /** Sends the last failed input again. The caller holds that input: it has
   *  to outlive this panel, like the error line it belongs to. */
  onRetry: () => void;
  canRetry: boolean;
  /** Set while the canvas cannot take a turn's result (the parent report
   *  editor during an export or a redraft). The composer and the choices are
   *  disabled; nothing is shown. */
  paused?: boolean;
  /** Set when the conversation is closed for good (the student has left the
   *  class). The composer, the choices and 重试 are disabled and this line is
   *  shown above the composer. */
  closedReason?: string | null;
  /** Shown while the conversation is empty: what the AI does on this page
   *  and what to ask. Each surface passes its own — no surface's copy lives
   *  in this shared file. */
  intro: string;
  /** Example requests shown under the intro while the conversation is
   *  empty. A click sends one as her turn. */
  suggestions?: string[];
  /** The canvas. The shell renders it as-is and imposes no read-only state —
   *  the caller owns whether and how each cell can be edited. */
  children: ReactNode;
}

const AI_RADIUS = "rounded-[4px_13px_13px_13px]";
const HER_RADIUS = "rounded-[13px_4px_13px_13px]";
const BUBBLE = "inline-block max-w-[85%] px-4 py-3 text-mk-body text-mk-ink";

export function WorkspacePanel({
  turns,
  busy,
  error,
  choices,
  onSend,
  onChoose,
  composer,
  onComposerChange,
  onRetry,
  canRetry,
  paused = false,
  closedReason = null,
  intro,
  suggestions = [],
  children,
}: WorkspacePanelProps) {
  // `busy` keeps its own meaning (a turn in flight: thinking row, stop
  // button); `blocked` is every reason she cannot act.
  const closed = closedReason !== null;
  const blocked = busy || paused || closed;
  const errorLine = panelErrorLine(error, closedReason);
  // `choices` names the pending row; `answeredKey` names the row she already
  // acted on. A fresh set of choices (a new reply) carries a different key,
  // so the row reappears for THAT reply without any effect needed to reset it.
  const [answeredKey, setAnsweredKey] = useState<string | null>(null);

  const choicesKey = JSON.stringify(choices.map((c) => c.id));
  const lastIsAi = turns.length > 0 && turns[turns.length - 1]!.role === "ai";
  const showChoices = lastIsAi && choices.length > 0 && choicesKey !== answeredKey;

  function handleSend() {
    const text = composer.trim();
    if (!text || blocked) return;
    // The composer is not cleared here: the caller clears it when it accepts
    // the turn (`beginTurn`), so a refused send keeps her text.
    // Typing past an offered choice counts as having answered it too — the
    // row belongs to a question that is now moot either way. Only an
    // accepted send answers it.
    if (onSend(text)) setAnsweredKey(choicesKey);
  }

  function handleChoose(choice: Choice) {
    if (blocked) return;
    if (onChoose(choice.id)) setAnsweredKey(choicesKey);
  }

  function handleSuggestion(text: string) {
    if (blocked) return;
    onSend(text);
  }

  const endRef = useRef<HTMLDivElement | null>(null);
  useEffect(() => {
    endRef.current?.scrollIntoView?.({ behavior: "smooth", block: "nearest" });
  }, [turns.length, showChoices, busy]);

  return (
    <TeacherPage width="full">
      {/* Wide (≥900px): a fixed-measure conversation column beside a canvas
          that takes the rest. Narrow: one column, canvas FIRST — a teacher on
          a phone needs to see what she is making before she reads the chat
          that is making it. `order-*` does the flip; `grid-cols-[minmax(…)]`
          does the measure (report editor's two-column grid is a plain 1fr/1fr
          split, so it can't be reused verbatim here). */}
      <div className="grid grid-cols-1 items-start gap-6 min-[900px]:grid-cols-[minmax(320px,420px)_1fr]">
        <div className="order-2 flex min-w-0 flex-col gap-3 rounded-mk-lg border border-mk-border bg-mk-surface p-4 min-[900px]:order-1">
          <div className="mk-scroll flex max-h-[60vh] min-h-[220px] flex-col gap-3 overflow-y-auto pr-1">
            {turns.length === 0 && !busy && (
              <div className="flex flex-col gap-3 rounded-mk-md bg-mk-paper p-3">
                <p className="text-mk-small text-mk-muted">{intro}</p>
                {suggestions.length > 0 && (
                  <div className="flex flex-wrap gap-2">
                    {suggestions.map((s) => (
                      <button
                        key={s}
                        type="button"
                        disabled={blocked}
                        onClick={() => handleSuggestion(s)}
                        className="rounded-mk-full border border-mk-border bg-mk-surface px-3 py-1 text-left text-mk-small text-mk-ink transition-colors duration-[120ms] ease-mk hover:bg-mk-accent-50 disabled:opacity-50"
                      >
                        {s}
                      </button>
                    ))}
                  </div>
                )}
              </div>
            )}
            {turns.map((t, i) =>
              t.role === "ai" ? (
                <div key={i} className="flex items-start justify-start gap-2">
                  <span className="mt-0.5 shrink-0">
                    <Pebble state="idle" size={24} />
                  </span>
                  <div className={`${BUBBLE} ${AI_RADIUS} bg-mk-paper shadow-mk-xs`}>
                    <LiteChatMarkdown text={t.text} />
                  </div>
                </div>
              ) : (
                <div key={i} className="flex justify-end">
                  <div className={`${BUBBLE} ${HER_RADIUS} whitespace-pre-wrap bg-mk-accent-50`}>{t.text}</div>
                </div>
              ),
            )}

            {showChoices && (() => {
              const { cards, pills } = splitChoices(choices);
              return (
                // Article cards stack (full width, one per row); plain
                // options stay the compact wrapping pill row. Only ARTICLE
                // options became cards, not every option a form field.
                <div className="flex flex-col gap-2 pl-8">
                  {cards.length > 0 && (
                    <div className="flex flex-col gap-2">
                      {cards.map((c) => (
                        <ChoiceArticleCard
                          key={c.id}
                          article={c.article!}
                          disabled={blocked}
                          onClick={() => handleChoose(c)}
                        />
                      ))}
                    </div>
                  )}
                  {pills.length > 0 && (
                    <div className="flex flex-wrap gap-2">
                      {pills.map((c) => (
                        <button
                          key={c.id}
                          type="button"
                          disabled={blocked}
                          onClick={() => handleChoose(c)}
                          className="rounded-mk-full border border-mk-accent-200 bg-mk-accent-50 px-3 py-1 text-mk-small text-mk-accent-700 transition-colors duration-[120ms] ease-mk hover:bg-mk-accent-100 disabled:opacity-50"
                        >
                          {c.label}
                        </button>
                      ))}
                    </div>
                  )}
                </div>
              );
            })()}

            {busy && (
              <div className="flex items-start justify-start gap-2" aria-label="处理中">
                <span className="mt-0.5 shrink-0">
                  <Pebble state="thinking" size={24} />
                </span>
                <div className={`${BUBBLE} ${AI_RADIUS} flex items-center gap-2 bg-mk-paper`}>
                  <span className="mk-think-dot" />
                  <span className="mk-think-dot [animation-delay:0.15s]" />
                  <span className="mk-think-dot [animation-delay:0.3s]" />
                  <span className="text-mk-small text-mk-muted">处理中</span>
                </div>
              </div>
            )}
            <div ref={endRef} />
          </div>

          {errorLine && (
            <div className="flex flex-wrap items-center gap-2">
              <p className="text-mk-small text-mk-danger" role="alert">
                {errorLine}
              </p>
              <Button variant="secondary" size="sm" onClick={onRetry} disabled={blocked || !canRetry}>
                重试
              </Button>
            </div>
          )}

          {closed && (
            <p className="text-mk-small font-semibold text-mk-muted" role="status">
              {closedReason}
            </p>
          )}

          <Composer
            value={composer}
            onChange={onComposerChange}
            onSend={handleSend}
            state={busy ? "replying" : composer.trim() && !blocked ? "typing" : "empty"}
            disabled={blocked}
            placeholder="请输入"
          />
        </div>

        <div className="order-1 min-w-0 min-[900px]:order-2">{children}</div>
      </div>
    </TeacherPage>
  );
}

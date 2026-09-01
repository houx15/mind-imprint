import { useEffect, useMemo, useState } from "react";
import { Library, ArrowRight } from "lucide-react";
import { Button, Icon } from "@/ui";
import { ApiError } from "../api/client";
import { createWriting, listWritings, isWritingFinished, type Writing } from "../api/writings";
import { navigate, writingPath } from "../routing";
import { PromptTile } from "../shared/PromptTile";
import { WRITING_IDEA_KEY } from "../readings/ReadingQuestions";
import { WRITING_TOPICS, type WritingTopic } from "./topics";
import { WritingHistoryPanel, type WritingFilter } from "./WritingHistoryPanel";
import { apiErrorText } from "../api/errorText";

/**
 * WritingsLanding — 写作 tab's front door. Same skeleton as ReadingsLanding
 * (Task 16), built fresh rather than imported: `Greeting`/`PaperBloom`/
 * `RecommendationTile` there are module-private (unexported), and the two
 * pages differ in the one place that matters — reading's box takes an
 * article (title + long body + upload), writing's box takes ONE sentence
 * that becomes both the title and the first turn of the conversation
 * (`createWriting`'s `idea`, writings.go). Forcing the reading form onto a
 * one-line "just start typing" box would be the second-worse choice the
 * brief warns about, so this is a deliberate second component that reuses
 * only what actually generalises: the `@/ui` primitives, the `.lite-glow`/
 * `.lite-ink-ring` CSS (index.css, shared across both landings), and the
 * exact page shape (notice bar → greeting → box → shelf → drawer).
 *
 * ONE BOX, NOT A FORM: there is no separate title field. Typing a sentence
 * and pressing 开始写作 (or Enter) starts the writing immediately — the
 * sentence does double duty server-side (truncated → title, verbatim → the
 * first atom_message), so nothing else needs to be filled in first.
 *
 * NOTE for the next task (verbatim, load-bearing e2e strings):
 *   - greeting: 「Hi，今天想写点什么」 (写 is its own <span> for the ink ring;
 *     match by the container's textContent, not a single text node)
 *   - box placeholder: 「说说你想写点什么，直接开始」
 *   - submit button label: 「开始写作」
 *   - history entry label: 「我的写作」
 *   - unfinished notice: 「你有 N 篇还没写完」
 *   - topics heading: 「不知道写什么？」
 */

export function WritingsLanding() {
  const [idea, setIdea] = useState("");
  const [starting, setStarting] = useState(false);
  const [startError, setStartError] = useState<string | null>(null);

  const [history, setHistory] = useState<Writing[] | null>(null);
  const [historyError, setHistoryError] = useState<string | null>(null);
  const [panelOpen, setPanelOpen] = useState(false);
  // Which chip the drawer opens on. 我的写作 wants everything; the 还没写完
  // notice wants the ones it just counted. Mirrors ReadingsLanding.
  const [panelFilter, setPanelFilter] = useState<WritingFilter>("all");

  function openPanel(filter: WritingFilter) {
    setPanelFilter(filter);
    setPanelOpen(true);
  }

  useEffect(() => {
    let cancelled = false;
    listWritings()
      .then((rows) => {
        if (!cancelled) setHistory(rows);
      })
      .catch(() => {
        if (!cancelled) setHistoryError("我的写作暂时加载不出来，刷新一下再试试。");
      });
    return () => {
      cancelled = true;
    };
  }, []);

  // Pick up a question 印记 grew from a finished reading (Task 10's
  // ReadingQuestions, 去写一写). READ-AND-CLEAR, not read: the key is a
  // one-shot handoff for THIS arrival, not a standing preference — a
  // lingering key would silently refill this box on every future visit.
  useEffect(() => {
    try {
      const stashed = sessionStorage.getItem(WRITING_IDEA_KEY);
      if (stashed) {
        sessionStorage.removeItem(WRITING_IDEA_KEY);
        setIdea(stashed);
      }
    } catch {
      // Private mode / storage disabled: nothing to pick up, and no reason
      // to block the rest of the page over it.
    }
  }, []);

  const unfinishedCount = useMemo(
    () => (history ?? []).filter((w) => !isWritingFinished(w)).length,
    [history],
  );

  async function start(text: string, lang: "zh" | "en" = "zh") {
    const trimmed = text.trim();
    if (!trimmed || starting) return;
    setStarting(true);
    setStartError(null);
    try {
      const { id } = await createWriting({ idea: trimmed, lang });
      navigate(writingPath(id));
    } catch (err) {
      setStartError(apiErrorText(err));
      setStarting(false);
    }
  }

  function handleTopic(topic: WritingTopic) {
    void start(topic.idea, topic.lang);
  }

  return (
    <div className="relative min-h-full overflow-hidden">
      <div className="relative mx-auto flex w-full max-w-[760px] flex-col px-4 pb-20 pt-5 sm:px-6">
        <div className="flex justify-end">
          <button
            type="button"
            onClick={() => openPanel("all")}
            className="flex items-center gap-2 rounded-mk-full border border-mk-border bg-mk-surface px-3 py-1.5 text-mk-small text-mk-secondary shadow-mk-xs transition-colors duration-[120ms] ease-mk hover:border-mk-accent-200 hover:text-mk-accent-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
          >
            <Icon icon={Library} size={15} />
            我的写作
            {unfinishedCount > 0 && (
              <span
                className="rounded-mk-full px-1.5 text-mk-label text-white"
                style={{ background: "var(--mk-accent-500)" }}
              >
                {unfinishedCount}
              </span>
            )}
          </button>
        </div>

        {/* Same unwired slot as ReadingsLanding: P4's teacher-assigned tasks
            join this strip, ABOVE the unfinished line. */}
        <div className="flex min-h-[34px] justify-center pt-6">
          {unfinishedCount > 0 && (
            <button
              type="button"
              // Opens the SHELF, not a writing — same fix ReadingsLanding
              // already carries: a count's only honest offer is the list
              // behind it, not a guess at which one she meant.
              onClick={() => openPanel("open")}
              className="group flex items-center gap-1.5 rounded-mk-full px-3 py-1 text-mk-small text-mk-accent-700 transition-colors duration-[120ms] ease-mk focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
              style={{ background: "color-mix(in srgb, var(--mk-accent-500) 10%, transparent)" }}
            >
              你有 {unfinishedCount} 篇还没写完
              <Icon
                icon={ArrowRight}
                size={13}
                className="transition-transform duration-[120ms] ease-mk group-hover:translate-x-0.5 motion-reduce:transition-none"
              />
            </button>
          )}
        </div>

        <Greeting />

        <div className="lite-glow mt-9">
          <div className="flex flex-col gap-2 rounded-mk-lg border border-mk-border bg-mk-surface p-1.5 shadow-mk-sm">
            <textarea
              value={idea}
              onChange={(e) => setIdea(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter" && !e.shiftKey && !e.nativeEvent.isComposing) {
                  e.preventDefault();
                  void start(idea);
                }
              }}
              placeholder="说说你想写点什么，直接开始"
              disabled={starting}
              aria-label="想写点什么"
              className="min-h-[112px] w-full resize-none rounded-mk-sm bg-transparent px-3 pb-2 pt-2.5 text-mk-body-lg text-mk-ink outline-none placeholder:text-[#B8ADA2] disabled:cursor-not-allowed"
            />
            <div className="flex items-center justify-end px-1.5 pb-1">
              <Button onClick={() => void start(idea)} disabled={!idea.trim()} loading={starting}>
                开始写作
              </Button>
            </div>
          </div>
        </div>

        {startError && (
          <p role="alert" className="mt-3 text-center text-mk-small text-mk-danger">
            {startError}
          </p>
        )}

        <section className="mt-14">
          <div className="flex items-center justify-center gap-3">
            <Hairline />
            <span className="text-mk-caption text-mk-muted">不知道写什么？</span>
            <Hairline />
          </div>

          <div className="mt-5 grid grid-cols-1 gap-3 sm:grid-cols-2">
            {WRITING_TOPICS.map((topic, i) => (
              <PromptTile
                key={topic.id}
                index={i + 1}
                tag={topic.genre}
                title={topic.title}
                reason={topic.reason}
                tone={topic.tone}
                disabled={starting}
                onPick={() => handleTopic(topic)}
              />
            ))}
          </div>
        </section>
      </div>

      <WritingHistoryPanel
        open={panelOpen}
        initialFilter={panelFilter}
        onClose={() => setPanelOpen(false)}
        writings={history}
        error={historyError}
        onSelect={(w) => {
          setPanelOpen(false);
          navigate(writingPath(w.id));
        }}
      />
    </div>
  );
}

// ---------------------------------------------------------------------------

/** 「Hi，今天想写点什么」 — same ink-circle language as ReadingsLanding's
 *  Greeting, a fresh mark around 写 rather than an import (module-private
 *  there, and the two shapes are different words on different geometry). */
function Greeting() {
  return (
    <h1 className="mt-6 text-center text-[24px] font-normal leading-[1.3] tracking-tight text-mk-ink sm:text-[34px] lg:text-[40px]">
      Hi，今天想
      <InkCircledXie />
      点什么
    </h1>
  );
}

function InkCircledXie() {
  return (
    <span className="relative inline-block px-[0.3em] align-baseline">
      <span className="relative z-10 font-semibold">写</span>
      <svg
        aria-hidden="true"
        viewBox="0 0 148 118"
        fill="none"
        className="pointer-events-none absolute left-1/2 top-[44%] z-0 h-[1.98em] w-[2.34em] -translate-x-1/2 -translate-y-1/2"
        style={{ overflow: "visible" }}
      >
        <path
          className="lite-ink-ring"
          pathLength={1}
          d="M30 84 C8 68, 8 34, 38 18 C68 2, 120 8, 134 34 C146 56, 128 88, 94 96 C66 103, 34 98, 22 82 C16 74, 20 96, 32 120"
          stroke="var(--mk-accent-500)"
          strokeWidth={4}
          strokeLinecap="round"
          strokeLinejoin="round"
          opacity={0.88}
        />
      </svg>
    </span>
  );
}

function Hairline() {
  return <span aria-hidden="true" className="h-px w-14 bg-mk-border" />;
}

import { useEffect, useMemo, useState } from "react";
import { Paperclip, Library, ArrowRight } from "lucide-react";
import { Button, Icon } from "@/ui";
import { ApiError } from "../api/client";
import {
  createReading,
  listReadings,
  putReadingSource,
  renameReading,
  uploadReadingSourceFile,
  type Reading,
} from "../api/readings";
import { navigate, readingPath } from "../routing";
import { PromptTile } from "../shared/PromptTile";
import { RECOMMENDED_READINGS, type RecommendedReading } from "./recommendations";
import { ReadingHistoryPanel, isFinished } from "./ReadingHistoryPanel";

/**
 * ReadingsLanding — the lite edition's front door.
 *
 * DESIGN DIRECTION. Task 11 shipped this page as a form: a label, an input, a
 * textarea, a button. It worked and it read as scaffolding. What replaced it
 * is built around a single idea — 一句招呼，一个圈出来的字 — and everything
 * else on the page is kept quiet so that mark can carry it:
 *
 *  - **The greeting is the hero.** 「Hi，今天要读点什么」 set large and light,
 *    centered, with 读 circled in 朱砂 by a stroke that DRAWS ITSELF once on
 *    load and ends in a tail pointing down at the paste box. The gesture is
 *    圈点 — the red-pen circle a Chinese reader has been making in margins
 *    forever — so the page's one flourish comes from the subject's own world
 *    rather than from a gradient.
 *  - **The paste box breathes.** A blurred warm halo (`.lite-glow`, see
 *    src/index.css) that holds brighter while she types. Deliberately not the
 *    cyan/violet ring every AI product wears.
 *  - **One boldness, spent once.** Recommendation tiles, the history entry
 *    and the notice bar are all flat, hairline-quiet objects. The ink mark is
 *    the only thing on the page raising its voice.
 *
 * THREE WAYS IN, ONE BOX. Paste a body, paste a link, or upload a DOCX/PDF —
 * all three land in the same reading through the same server-side storage
 * call, so the room downstream cannot tell them apart. A body that is nothing
 * but a single http(s) token is sent as `url` (the server fetches it) rather
 * than stored as a one-line article.
 *
 * 铁律②: 今日推荐 is help for a student who does not know what to read — a
 * fixed shelf of four, no paging, no personalization, no reason to come back
 * and scroll. See recommendations.ts.
 *
 * NOTE for Task 14 (verbatim, load-bearing e2e strings):
 *   - greeting: 「Hi，今天要读点什么」 (读 is its own <span>, so match by
 *     the container's textContent, not by a single text node)
 *   - title input placeholder: 「给这次阅读起个名字（可留空）」 (unchanged)
 *   - body textarea placeholder: 「贴一个链接，或者把整篇正文粘进来——也可以上传 DOCX / PDF」
 *   - submit button label: 「开始阅读」 (unchanged)
 *   - upload button label: 「上传 DOCX / PDF」
 *   - history entry label: 「我的阅读」
 *   - unfinished notice: 「你有 N 篇还没读完」
 *   - recommendations heading: 「不知道读什么？」
 */

/** A body that is nothing but one http(s) token is a LINK, not an article —
 *  the server can fetch it into readable text, which is strictly better than
 *  storing a one-line "article" that says `https://…`. Anything with prose
 *  around it is treated as pasted text. */
export function asLink(body: string): string | null {
  const trimmed = body.trim();
  return /^https?:\/\/\S+$/i.test(trimmed) ? trimmed : null;
}

/** The reading's name when she did not type one. Taken from the first line
 *  of what she pasted — deterministic, and better than 未命名阅读 sitting in
 *  我的阅读 four times over. Blank when there is nothing usable, which lets
 *  the server's own fallback take it. */
export function deriveTitle(body: string): string {
  const first = body
    .split("\n")
    .map((line) => line.trim())
    .find((line) => line.length > 0);
  if (!first || asLink(first)) return "";
  const chars = [...first];
  return chars.length <= 24 ? first : `${chars.slice(0, 24).join("")}…`;
}

export function ReadingsLanding() {
  const [title, setTitle] = useState("");
  const [body, setBody] = useState("");
  const [starting, setStarting] = useState(false);
  const [startError, setStartError] = useState<string | null>(null);
  // The reading minted by a PREVIOUS attempt that then failed to take its
  // article. 开始阅读 is two calls (createReading, then the source call) and
  // only the second is retried here — without this, every retry after a
  // transient network blip left another empty reading behind in 我的阅读.
  const [pendingId, setPendingId] = useState<string | null>(null);

  const [history, setHistory] = useState<Reading[] | null>(null);
  const [historyError, setHistoryError] = useState<string | null>(null);
  const [panelOpen, setPanelOpen] = useState(false);

  useEffect(() => {
    let cancelled = false;
    listReadings()
      .then((rows) => {
        if (!cancelled) setHistory(rows);
      })
      .catch(() => {
        if (!cancelled) setHistoryError("我的阅读暂时加载不出来，刷新一下再试试。");
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const unfinishedCount = useMemo(
    () => (history ?? []).filter((r) => !isFinished(r)).length,
    [history],
  );

  /** Mint the reading (or reuse the half-created one) and hand its id back. */
  async function ensureReading(name: string, lang: "zh" | "en"): Promise<string> {
    if (pendingId) return pendingId;
    const { id } = await createReading({ title: name, lang });
    setPendingId(id);
    return id;
  }

  async function handleStart() {
    const text = body.trim();
    if (!text || starting) return;
    setStarting(true);
    setStartError(null);
    try {
      const link = asLink(text);
      const name = title.trim() || (link ? "" : deriveTitle(text));
      const id = await ensureReading(name, "zh");
      await putReadingSource(id, link ? { title: name, url: link } : { title: name, text });
      setPendingId(null);
      navigate(readingPath(id));
    } catch (err) {
      setStartError(err instanceof ApiError ? err.message : "开始阅读失败，请重试。");
      setStarting(false);
    }
  }

  async function handleFile(file: File | null | undefined) {
    if (!file || starting) return;
    setStarting(true);
    setStartError(null);
    try {
      const typed = title.trim();
      const id = await ensureReading(typed, "zh");
      await uploadReadingSourceFile(id, file);
      // The server names the reading after the document. Hers wins if she
      // took the trouble to type one — but a failure to rename must not cost
      // her the upload, so it is best-effort.
      if (typed) await renameReading(id, typed).catch(() => undefined);
      setPendingId(null);
      navigate(readingPath(id));
    } catch (err) {
      setStartError(err instanceof ApiError ? err.message : "上传失败，请重试。");
      setStarting(false);
    }
  }

  async function handleRecommendation(rec: RecommendedReading) {
    if (starting) return;
    setStarting(true);
    setStartError(null);
    try {
      const id = await ensureReading(rec.title, rec.lang);
      await putReadingSource(
        id,
        rec.source.kind === "text"
          ? { title: rec.title, text: rec.source.text }
          : { title: rec.title, url: rec.source.url },
      );
      setPendingId(null);
      navigate(readingPath(id));
    } catch (err) {
      setStartError(err instanceof ApiError ? err.message : "开始阅读失败，请重试。");
      setStarting(false);
    }
  }

  function openMostRecentUnfinished() {
    // lastActivityAt, not updatedAt: "most recently touched" has to mean the
    // one she was actually last reading. updatedAt moves only on rename and
    // finish, so this used to open an essentially arbitrary reading.
    const next = (history ?? [])
      .filter((r) => !isFinished(r))
      .sort((a, b) => b.lastActivityAt.localeCompare(a.lastActivityAt))[0];
    if (next) navigate(readingPath(next.id));
  }

  return (
    <div className="relative min-h-full overflow-hidden">
      <PaperBloom />

      <div className="relative mx-auto flex w-full max-w-[760px] flex-col px-4 pb-20 pt-5 sm:px-6">
        <div className="flex justify-end">
          <button
            type="button"
            onClick={() => setPanelOpen(true)}
            className="flex items-center gap-2 rounded-mk-full border border-mk-border bg-mk-surface px-3 py-1.5 text-mk-small text-mk-secondary shadow-mk-xs transition-colors duration-[120ms] ease-mk hover:border-mk-accent-200 hover:text-mk-accent-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
          >
            <Icon icon={Library} size={15} />
            我的阅读
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

        {/* NOTICES. Today the only notice is her own unfinished work. P4's
            teacher-assigned tasks (with deadlines) join this same strip,
            ABOVE the unfinished line — the slot is here and deliberately
            unwired: nothing in this build produces a teacher task. */}
        <div className="flex min-h-[34px] justify-center pt-6">
          {unfinishedCount > 0 && (
            <button
              type="button"
              onClick={openMostRecentUnfinished}
              className="group flex items-center gap-1.5 rounded-mk-full px-3 py-1 text-mk-small text-mk-accent-700 transition-colors duration-[120ms] ease-mk focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
              style={{ background: "color-mix(in srgb, var(--mk-accent-500) 10%, transparent)" }}
            >
              你有 {unfinishedCount} 篇还没读完
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
          <div className="rounded-mk-lg border border-mk-border bg-mk-surface p-1.5 shadow-mk-sm">
            <input
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder="给这次阅读起个名字（可留空）"
              disabled={starting}
              aria-label="阅读的名字"
              className="w-full rounded-mk-sm bg-transparent px-3 pb-1.5 pt-2 text-mk-small text-mk-secondary outline-none placeholder:text-[#C3B8AC] disabled:cursor-not-allowed"
            />
            <textarea
              value={body}
              onChange={(e) => setBody(e.target.value)}
              placeholder="贴一个链接，或者把整篇正文粘进来——也可以上传 DOCX / PDF"
              disabled={starting}
              aria-label="文章正文或链接"
              className="min-h-[176px] w-full resize-none rounded-mk-sm bg-transparent px-3 pb-2 text-mk-body-lg text-mk-ink outline-none placeholder:text-[#B8ADA2] disabled:cursor-not-allowed"
            />
            <div className="flex items-center justify-between gap-3 px-1.5 pb-1">
              <label className="inline-flex cursor-pointer items-center gap-1.5 rounded-mk-sm px-2 py-1.5 text-mk-small text-mk-muted transition-colors duration-[120ms] ease-mk hover:bg-mk-accent-50 hover:text-mk-accent-700 focus-within:ring-2 focus-within:ring-mk-accent-200">
                <Icon icon={Paperclip} size={15} />
                上传 DOCX / PDF
                <input
                  type="file"
                  accept=".pdf,.docx"
                  className="sr-only"
                  disabled={starting}
                  onChange={(e) => {
                    void handleFile(e.target.files?.[0]);
                    // Clear it so picking the SAME file twice still fires.
                    e.target.value = "";
                  }}
                />
              </label>
              <Button onClick={handleStart} disabled={!body.trim()} loading={starting}>
                开始阅读
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
            <span className="flex items-center gap-2 text-mk-caption text-mk-muted">
              <OpenBookMark />
              不知道读什么？
            </span>
            <Hairline />
          </div>

          <div className="mt-5 grid grid-cols-1 gap-3 sm:grid-cols-2">
            {RECOMMENDED_READINGS.map((rec, i) => (
              <PromptTile
                key={rec.id}
                index={i + 1}
                tag={rec.genre}
                title={rec.title}
                reason={rec.reason}
                tone={rec.tone}
                disabled={starting}
                onPick={() => void handleRecommendation(rec)}
              />
            ))}
          </div>
        </section>
      </div>

      <ReadingHistoryPanel
        open={panelOpen}
        onClose={() => setPanelOpen(false)}
        readings={history}
        error={historyError}
        onSelect={(r) => {
          setPanelOpen(false);
          navigate(readingPath(r.id));
        }}
      />
    </div>
  );
}

// ---------------------------------------------------------------------------
// The page's marks
// ---------------------------------------------------------------------------

/** A soft warm bloom behind the greeting. Not a decorative blob: it is what
 *  keeps the eye at the top of an otherwise very quiet page. Built with
 *  color-mix, because `bg-mk-accent/12` on a bare CSS variable emits nothing. */
function PaperBloom() {
  return (
    <div
      aria-hidden="true"
      className="pointer-events-none absolute left-1/2 top-0 h-[420px] w-[820px] -translate-x-1/2"
      style={{
        background:
          "radial-gradient(50% 55% at 50% 32%, color-mix(in srgb, var(--mk-accent-500) 9%, transparent) 0%, transparent 100%)",
      }}
    />
  );
}

/** 「Hi，今天要读点什么」 — the page's thesis, spoken by 豆豆.
 *
 *  Light weight at a display size, so a big line still reads as a question
 *  rather than a banner. The circled 读 is the only heavy element. */
function Greeting() {
  return (
    <h1 className="mt-6 text-center text-[24px] font-normal leading-[1.3] tracking-tight text-mk-ink sm:text-[34px] lg:text-[40px]">
      Hi，今天要
      <InkCircledDu />
      点什么
    </h1>
  );
}

/**
 * The signature: 读 with an ink ring around it.
 *
 * Hand-drawn, not geometric — the loop is an uneven egg, it OVERSHOOTS where
 * it closes (the pen crosses its own line, the way a real circle does), and
 * the overshoot keeps going into a tail that flicks down toward the paste
 * box. The gesture encodes the instruction: this word, then that box.
 *
 * Drawn as an SVG path rather than set in a handwriting face on purpose: a
 * Chinese display webfont is megabytes over the wire, and a badly-hinted one
 * would make the single most visible glyph on the page the worst-looking. The
 * character stays in the system face and typographically correct; the hand is
 * in the ink. `pathLength="1"` lets the draw-on animation (see index.css) use
 * an exact 1→0 dash offset instead of a measured constant.
 */
function InkCircledDu() {
  return (
    <span className="relative inline-block px-[0.3em] align-baseline">
      <span className="relative z-10 font-semibold">读</span>
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
          d="M32 88 C10 74, 6 38, 34 20 C64 2, 118 5, 134 32 C146 54, 130 87, 96 96 C68 103, 36 100, 24 85 C18 78, 21 98, 34 122"
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

/** A small open book, drawn in two strokes. The second hand-drawn mark on the
 *  page, and it earns its place by doing a different job: it labels the
 *  shelf, where the ink ring labels the invitation. */
function OpenBookMark() {
  return (
    <svg aria-hidden="true" viewBox="0 0 28 21" className="h-[15px] w-[20px]" fill="none">
      <path
        d="M14 5.4 C11 2.8, 6.6 2, 2.6 2.9 L2.6 15.8 C6.6 14.9, 11 15.7, 14 18.4"
        stroke="var(--mk-accent-300)"
        strokeWidth={1.5}
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <path
        d="M14 5.4 C17 2.8, 21.4 2, 25.4 2.9 L25.4 15.8 C21.4 14.9, 17 15.7, 14 18.4"
        stroke="var(--mk-accent-300)"
        strokeWidth={1.5}
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}

function Hairline() {
  return <span aria-hidden="true" className="h-px w-14 bg-mk-border" />;
}

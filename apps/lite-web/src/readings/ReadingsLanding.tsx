import { LandingHeader } from "../learning/LandingHeader";
import { useEffect, useMemo, useState } from "react";
import { Paperclip, Library, ArrowRight } from "lucide-react";
import { Button, Icon } from "@/ui";
import { ApiError } from "../api/client";
import {
  archiveReading,
  createReading,
  listReadings,
  putReadingSource,
  renameReading,
  uploadReadingSourceFile,
  type Reading,
} from "../api/readings";
import { navigate, readingPath, readingLibraryPath } from "../routing";
import { getLibraryShelf, startLibraryReading, type LibraryShelf } from "../api/library";
import { LibraryCard } from "./LibraryCard";
import { ReadingHistoryPanel, isFinished, type ReadingFilter } from "./ReadingHistoryPanel";
import { ReadingsTabs } from "./ReadingsTabs";
import { apiErrorText } from "../api/errorText";
import { AssignmentStrip } from "../inbox/AssignmentStrip";

/**
 * ReadingsLanding — the lite edition's front door.
 *
 * Presentation uses the shared Lite illustration header and scoped palette.
 *
 * THREE WAYS IN, ONE BOX. Paste a body, paste a link, or upload a DOCX/PDF —
 * all three land in the same reading through the same server-side storage
 * call, so the room downstream cannot tell them apart. A body that is nothing
 * but a single http(s) token is sent as `url` (the server fetches it) rather
 * than stored as a one-line article.
 *
 * 不知道读什么？ is answered by the 分级阅读库 (`GET /api/v1/library`): four
 * real articles picked from her interest tree, at the level her history
 * suggests, each with a photograph and its five difficulties. It replaced
 * four seed texts hardcoded in `recommendations.ts` — those could not be
 * filtered, searched, or connected to anything she had done. 查看全部 opens
 * the whole shelf at `/readings/library`.
 *
 * NOTE for Task 14 (verbatim, load-bearing e2e strings):
 *   - title input placeholder: 「给这次阅读起个名字（可留空）」 (unchanged)
 *   - body textarea placeholder: 「贴一个链接，或者把整篇正文粘进来——也可以上传 PDF / DOCX / TXT」
 *   - submit button label: 「开始阅读」 (unchanged)
 *   - upload button label: 「上传 DOCX / PDF」
 *   - history entry label: 「我的阅读」
 *   - unfinished notice: 「你有 N 篇还没读完」
 *   - recommendations heading: 「不知道读什么？」
 */

/** A body that is nothing but one link is a LINK, not an article — the server
 *  can fetch it into readable text, which is strictly better than storing a
 *  one-line "article" that says `https://…`. Anything with prose around it is
 *  treated as pasted text.
 *
 *  🚨 2026-09-22：少了 scheme 的那一份也要认。她从地址栏复制出来的常常是
 *  `www.bbc.com/news/…`（Chrome 复制的就是这个形状），原来那条正则不认，
 *  于是这一行 URL 被当成一篇一句话的文章存了下来 —— 产品负责人报的
 *  「粘贴链接识别不了」里的一种。补上 https:// 交给服务端去抓。 */
export function asLink(body: string): string | null {
  const trimmed = body.trim();
  if (/\s/.test(trimmed) || trimmed === "") return null;
  if (/^https?:\/\/\S+$/i.test(trimmed)) return trimmed;
  // 一个带点的域名开头，后面可以有路径。要求域名里有一个点且后缀是字母，
  // 免得把 `第3段。` 这种也当成链接。
  if (/^[a-z0-9-]+(\.[a-z0-9-]+)*\.[a-z]{2,}(\/\S*)?$/i.test(trimmed)) {
    return `https://${trimmed}`;
  }
  return null;
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

  const [shelf, setShelf] = useState<LibraryShelf | null>(null);
  const [history, setHistory] = useState<Reading[] | null>(null);
  const [historyError, setHistoryError] = useState<string | null>(null);
  const [panelOpen, setPanelOpen] = useState(false);
  // Which chip the drawer opens on. 我的阅读 wants everything; the 还没读完
  // notice wants the eight it just counted.
  const [panelFilter, setPanelFilter] = useState<ReadingFilter>("all");

  function openPanel(filter: ReadingFilter) {
    setPanelFilter(filter);
    setPanelOpen(true);
  }

  // 书架自己失败时不写 startError：那是「开始阅读」那一格的位置，一条关于
  // 推荐的报错摆在那里会看起来像是她粘的东西出了问题。书架取不到就不显示，
  // 页面其余部分照常。
  useEffect(() => {
    let cancelled = false;
    getLibraryShelf()
      .then((s) => {
        if (!cancelled) setShelf(s);
      })
      .catch(() => undefined);
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    let cancelled = false;
    listReadings()
      .then((rows) => {
        if (!cancelled) setHistory(rows);
      })
      .catch(() => {
        if (!cancelled) setHistoryError("加载阅读记录失败，请刷新重试。");
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
      setStartError(apiErrorText(err));
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
      setStartError(apiErrorText(err));
      setStarting(false);
    }
  }

  /** 从库里开一篇。服务端已经在开着同一篇同一档时把那一篇还回来，所以这里
   *  不必再靠标题去猜「是不是同一次阅读」—— 那是旧书架的做法，四篇写死的文章
   *  才有一个固定的标题可以比。 */
  async function handleLibraryStart(slug: string, tier: number) {
    if (starting) return;
    setStarting(true);
    setStartError(null);
    try {
      navigate(readingPath(await startLibraryReading(slug, tier)));
    } catch (err) {
      setStartError(apiErrorText(err));
      setStarting(false);
    }
  }

  return (
    <div className="learning-landing relative min-h-full overflow-hidden">

      <div className="learning-landing-measure relative mx-auto flex w-full flex-col">
        {/* 页签在中间、我的阅读在右边。左边那一格是空的占位，它存在只为让页签
            真的落在页面中线上 —— 没有它，页签会被右边那个按钮推得偏左。 */}
        <div className="learning-entry-toolbar flex items-center justify-between gap-3">
          <div className="hidden flex-1 sm:block" />
          <ReadingsTabs active="own" libraryCount={shelf?.articles.length} />
          <div className="flex flex-1 justify-end">
          <button
            type="button"
            onClick={() => openPanel("all")}
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
        </div>

        {/* NOTICES. Teacher-assigned readings (with deadlines) sit ABOVE her
            own unfinished line. `AssignmentStrip` renders nothing when no
            reading is assigned and still open, so the page is unchanged for a
            student without assignments. */}
        <AssignmentStrip kind="reading" className="learning-landing-notices" />
        <div className="learning-landing-notices flex justify-start">
          {unfinishedCount > 0 && (
            <button
              type="button"
              // Opens the SHELF, not a reading. It used to jump straight into
              // the most recently touched one, which answered a question she
              // hadn't asked — 「你有 8 篇还没读完」 is a count, and the only
              // sane thing a count can offer is the list behind it.
              onClick={() => openPanel("open")}
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

        <LandingHeader kind="reading" title="阅读" description="带来文章或链接，与印记一起理解内容、核查来源。" />

        <div className="learning-composer">
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
              placeholder="贴一个链接，或者把整篇正文粘进来——也可以上传 PDF / DOCX / TXT"
              disabled={starting}
              aria-label="文章正文或链接"
              className="min-h-[176px] w-full resize-none rounded-mk-sm bg-transparent px-3 pb-2 text-mk-body-lg text-mk-ink outline-none placeholder:text-[#B8ADA2] disabled:cursor-not-allowed"
            />
            <div className="flex items-center justify-between gap-3 px-1.5 pb-1">
              <label className="inline-flex cursor-pointer items-center gap-1.5 rounded-mk-sm px-2 py-1.5 text-mk-small text-mk-muted transition-colors duration-[120ms] ease-mk hover:bg-mk-accent-50 hover:text-mk-accent-700 focus-within:ring-2 focus-within:ring-mk-accent-200">
                <Icon icon={Paperclip} size={15} />
                上传 PDF / DOCX / TXT
                <input
                  type="file"
                  accept=".pdf,.docx,.txt,.md"
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

        {shelf && shelf.recommended.length > 0 && (
          <section className="mk-branch-hues mt-14">
            <div className="flex items-center justify-center gap-3">
              <Hairline />
              <span className="flex items-center gap-2 text-mk-caption text-mk-muted">
                <OpenBookMark />
                不知道读什么？
              </span>
              <Hairline />
            </div>

            <div className="mt-5 grid grid-cols-1 gap-4 sm:grid-cols-2">
              {shelf.recommended.map((rec) => {
                const article = shelf.articles.find((a) => a.slug === rec.slug);
                if (!article) return null;
                return (
                  <LibraryCard
                    key={rec.slug}
                    article={article}
                    defaultTier={rec.tier}
                    why={rec.why}
                    busy={starting}
                    onStart={(slug, tier) => void handleLibraryStart(slug, tier)}
                    onResume={(id) => navigate(readingPath(id))}
                  />
                );
              })}
            </div>

            <div className="mt-5 flex justify-center">
              <button
                type="button"
                onClick={() => navigate(readingLibraryPath())}
                className="flex items-center gap-1.5 rounded-mk-full border border-mk-border bg-mk-surface px-4 py-1.5 text-mk-small text-mk-secondary shadow-mk-xs transition-colors duration-[120ms] ease-mk hover:border-mk-accent-200 hover:text-mk-accent-700"
              >
                查看全部 {shelf.articles.length} 篇
                <Icon icon={ArrowRight} size={14} />
              </button>
            </div>
          </section>
        )}
      </div>

      <ReadingHistoryPanel
        open={panelOpen}
        initialFilter={panelFilter}
        onClose={() => setPanelOpen(false)}
        readings={history}
        error={historyError}
        onSelect={(r) => {
          setPanelOpen(false);
          navigate(readingPath(r.id));
        }}
        onArchive={async (r) => {
          await archiveReading(r.id);
          // 就地拿掉那一行，不整份重拉：列表已经在手上，重拉一次会让抽屉
          // 闪一下，而她刚刚做的那个动作的结果正是「这一行没了」。
          setHistory((rows) => (rows ?? []).filter((x) => x.id !== r.id));
        }}
      />
    </div>
  );
}

// ---------------------------------------------------------------------------
// The page's marks
// ---------------------------------------------------------------------------

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

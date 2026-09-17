import { LandingHeader } from "../learning/LandingHeader";
import { useEffect, useMemo, useState } from "react";
import { Library, ArrowRight, FileUp, Paperclip } from "lucide-react";
import { Button, Icon, Modal } from "@/ui";
import { ApiError } from "../api/client";
import { createWriting, extractDocument, listWritings, isWritingFinished, type Writing } from "../api/writings";
import { navigate, writingPath } from "../routing";
import { PromptTile } from "../shared/PromptTile";
import { WRITING_IDEA_KEY } from "../readings/ReadingQuestions";
import { WRITING_TOPICS, type WritingTopic } from "./topics";
import { WritingHistoryPanel, type WritingFilter } from "./WritingHistoryPanel";
import { apiErrorText } from "../api/errorText";
import { AssignmentStrip } from "../inbox/AssignmentStrip";
import { splitBroughtFile } from "./broughtFile";

/**
 * WritingsLanding — Lite writing entry. Presentation uses LandingHeader and
 * the scoped entry-page palette. Writing creation, imports and history retain
 * their existing API and navigation behavior.
 *
 * ONE BOX, NOT A FORM: there is no separate title field. Typing a sentence
 * and pressing 开始写作 (or Enter) starts the writing immediately — the
 * sentence does double duty server-side (truncated → title, verbatim → the
 * first atom_message), so nothing else needs to be filled in first.
 *
 * NOTE for the next task (verbatim, load-bearing e2e strings):
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

  // 带一篇写好的进来：一个弹窗，两个框（这是什么 + 正文）。
  const [bringOpen, setBringOpen] = useState(false);
  const [bringTitle, setBringTitle] = useState("");
  const [bringBody, setBringBody] = useState("");
  const [extracting, setExtracting] = useState(false);
  const [bringFileError, setBringFileError] = useState<string | null>(null);

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

  async function start(text: string, lang?: "zh" | "en") {
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

  /**
   * 上传一份文件，把里面的文字放进上面那个框。
   *
   * 🚨 取出来的文字**落进框里**，不直接建这一篇。她仍然看得见、改得动，
   * 按「请印记看看」的时候才真的交出去 —— 上传只是省掉复制粘贴那一下，
   * 不替她做决定。失败那一句原样来自服务端（扫描件和文件坏了是两回事）。
   */
  async function bringFile(file: File | undefined) {
    if (!file || extracting) return;
    setExtracting(true);
    setBringFileError(null);
    try {
      const got = splitBroughtFile(await extractDocument(file), file.name);
      setBringBody((prev) => (prev.trim() ? prev + "\n\n" + got.body : got.body));
      // 文件自带的标题只在她还没写标题的时候用 —— 她写过的不覆盖。
      setBringTitle((prev) => prev.trim() || got.title);
    } catch (err) {
      setBringFileError(apiErrorText(err));
    } finally {
      setExtracting(false);
    }
  }

  /**
   * 带一篇写好的进来。
   *
   * 和 `start` 走同一个接口，只是多给一个 body —— 服务端据此把这一篇直接放在
   * 成稿那一步、来源记成 brought。这里不做任何「看起来像不像一篇文章」的判断：
   * 她说这是她写完的，那就是。
   */
  async function bringIn() {
    const body = bringBody.trim();
    const title = bringTitle.trim() || body.slice(0, 40);
    if (!body || starting) return;
    setStarting(true);
    setStartError(null);
    try {
      const { id } = await createWriting({ idea: title, body });
      navigate(writingPath(id));
    } catch (err) {
      setStartError(apiErrorText(err));
      setStarting(false);
      setBringOpen(false);
    }
  }

  return (
    <div className="learning-landing relative min-h-full overflow-hidden">
      <div className="learning-landing-measure relative mx-auto flex w-full flex-col">
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

        {/* Same slot as ReadingsLanding: teacher-assigned writings sit ABOVE
            the unfinished line, and `AssignmentStrip` renders nothing when
            none is assigned and still open. */}
        <AssignmentStrip kind="writing" className="learning-landing-notices" />
        <div className="learning-landing-notices flex justify-start">
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

        <LandingHeader kind="writing" title="写作" description="整理想法、组织论证，也可以带来已有文章寻求建议。" />

        <div className="learning-composer">
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
            <div className="flex items-center justify-between px-1.5 pb-1">
              {/* 带一篇写好的进来。
                  产品负责人 2026-09-11：「we also make students available to
                  upload a written one to seek for advice」。
                  它和「开始写作」并排，不是藏在别处：写完了想要意见，
                  和从零开始，是两件同样正当的事。 */}
              <button
                type="button"
                onClick={() => setBringOpen(true)}
                disabled={starting}
                className="flex items-center gap-1.5 rounded-mk-sm px-2 py-1.5 text-mk-small text-mk-secondary transition-colors hover:text-mk-accent-700 disabled:cursor-not-allowed"
              >
                <Icon icon={FileUp} size={14} />
                带一篇写好的进来
              </button>
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

      {/* 带一篇写好的进来。
          两个框：这是什么（可以不填，不填就取正文开头）、正文。
          🚨 没有第三个框问文体、也没有问语言 —— 语言在进房间之后那个 设定
          弹窗里问，那是它本来的位置，在这儿再问一次只是多一道门。 */}
      <Modal
        open={bringOpen}
        onClose={() => setBringOpen(false)}
        title="带一篇写好的进来"
        footer={
          <>
            <Button variant="ghost" onClick={() => setBringOpen(false)}>
              取消
            </Button>
            <Button onClick={() => void bringIn()} disabled={!bringBody.trim()} loading={starting}>
              请印记看看
            </Button>
          </>
        }
      >
        <div className="flex flex-col gap-3">
          <p className="text-mk-body text-mk-muted">
            粘贴你已经写完的那一篇。印记会通篇看一遍，给你具体的意见。
            结构和段落两步不会再走一遍。
          </p>
          <label className="flex flex-col gap-1.5">
            <span className="text-mk-small text-mk-secondary">题目</span>
            <input
              value={bringTitle}
              onChange={(e) => setBringTitle(e.target.value)}
              placeholder="这一篇叫什么"
              className="w-full rounded-mk-sm border border-mk-input-border bg-mk-paper px-3 py-2 text-mk-body text-mk-ink outline-none placeholder:text-[#B8ADA2] focus-visible:border-mk-accent"
            />
          </label>
          <label className="flex flex-col gap-1.5">
            <span className="text-mk-small text-mk-secondary">正文</span>
            <textarea
              value={bringBody}
              onChange={(e) => setBringBody(e.target.value)}
              placeholder="把你写好的文章粘贴到这里"
              className="min-h-[220px] w-full resize-y rounded-mk-sm border border-mk-input-border bg-mk-paper px-3 py-2 text-mk-body text-mk-ink outline-none placeholder:text-[#B8ADA2] focus-visible:border-mk-accent"
            />
          </label>
          {/* 上传。走的是和阅读那边同一件工具（docextract），取出来的文字直接
              落进上面那个框 —— 她仍然看得见、改得动，按「请印记看看」的时候才
              真的交出去。 */}
          <div className="flex items-center gap-3">
            <label className="inline-flex cursor-pointer items-center gap-1.5 rounded-mk-sm px-2 py-1.5 text-mk-small text-mk-muted transition-colors duration-[120ms] ease-mk hover:bg-mk-accent-50 hover:text-mk-accent-700 focus-within:ring-2 focus-within:ring-mk-accent-200">
              <Icon icon={Paperclip} size={15} />
              上传 PDF / DOCX / TXT
              <input
                type="file"
                accept=".pdf,.docx,.txt,.md"
                className="sr-only"
                disabled={extracting || starting}
                onChange={(e) => {
                  void bringFile(e.target.files?.[0]);
                  // 清掉，否则同一个文件选第二次不会再触发。
                  e.target.value = "";
                }}
              />
            </label>
            {extracting && <span className="text-mk-small text-mk-muted">正在读取文件…</span>}
            {bringFileError && <span className="text-mk-small text-mk-danger">{bringFileError}</span>}
          </div>
        </div>
      </Modal>

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

function Hairline() {
  return <span aria-hidden="true" className="h-px w-14 bg-mk-border" />;
}

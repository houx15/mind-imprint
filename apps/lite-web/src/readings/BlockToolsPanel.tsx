import { useEffect, useMemo, useRef, useState } from "react";
import { X } from "lucide-react";
import { Icon } from "@/ui";
import { ChatMarkdown } from "@/studio/ai/ChatMarkdown";
import { segmentSentences } from "@/primitives/annotate/sentences";
import { explainReadingBlock, grammarHasContent, type ReadingBlockNote, type ReadingBlockTool } from "../api/readingRoom";
import { BlockToolbar } from "./BlockToolbar";
import { WordCards } from "./WordCards";
import { GrammarCards } from "./GrammarCards";
import { BLOCK_TOOL_ANSWER, type CoachCardAnswer } from "./CoachCard";
import { apiErrorText } from "../api/errorText";

/**
 * BlockToolsPanel — 点开一段，把它拆给她看。
 *
 * The product call: *"for lite-level students, they first need to be taught
 * about the paragraphs. for english reading materials — 翻译、关键单词讲解、
 * 语法讲解、写作解析; chinese paragraph — 成语/修辞运用、案例、结构解析. then
 * we can guide students to focus on information/subject lens then."*
 *
 * That **then** is the whole point: being able to read one paragraph is the
 * step below reading a whole article through a lens. The lens was always
 * here; this is the rung underneath it.
 *
 * ## Shape (2026-08-28)
 *
 * This used to be a card that opened under the paragraph carrying (a) a copy
 * of the paragraph, (b) the row of tool buttons, (c) the explanation. The
 * copy was the complaint:
 *
 *   > after clicking 详细带读, the paragraph appears again in that card, which
 *   > is really duplicated. so as above: click a paragraph, a row of clickable
 *   > operations appears near my mouse, with a AI mascot at the left.
 *
 * So the buttons left for a floating bar at her pointer (BlockToolbar), the
 * duplicated paragraph is gone — she is looking straight at the real one —
 * and what remains in the flow is **only the explanation**, hung under the
 * paragraph it explains.
 *
 * ## 两件工具不再产出散文（2026-09-16）
 *
 * - **关键单词**产出一组词卡（词 / 词性 / 意思 / 讲解 / 例句），由 `WordCards`
 *   渲染，同时这几个词会在正文里被荧光笔标出来。
 * - **语法**改成讲**一句话**：点它之后先请她在这一段里点一个句子，再讲那一句。
 *   产品负责人的原话是「currently we only have paragraph level which is
 *   strange」。哪些工具要先点句子由服务端的 `subject` 决定，不写死在这里。
 *
 * ## 铁律 check
 *
 * These are EXPLANATORY and that is why they are safe. 铁律① forbids the AI
 * writing the student's OWN prose; explaining someone else's published
 * paragraph is what a teacher does, and withholding it would make the room
 * less useful without making it more honest.
 *
 * The line that IS held, and held structurally: every tool is keyed on a
 * `blockId` of the ARTICLE, and nothing in this component can write to her
 * 摘要, her 批注 or her takeaway — it has no such prop and no such call. The
 * explanations also live in their own server-side table so a later report can
 * always tell them apart from what she wrote.
 *
 * Results are cached server-side by (blockId, tool, sentence), so re-opening
 * one is instant and free — which is most of why this reads as a set of tools
 * rather than as another chat.
 */
export function BlockToolsPanel({
  readingId,
  blockId,
  blockText,
  anchorEl,
  pointerX,
  tools,
  notes,
  autoTool,
  onAutoToolConsumed,
  onNote,
  onClose,
  ordinal,
  onToolAnswer,
}: {
  readingId: string;
  blockId: string;
  /** 这一段的正文。按句子讲的那件工具要靠它切出可点的句子。 */
  blockText: string;
  /** The paragraph element the floating bar pins itself to. Null → no bar
   *  (the explanation still renders; this is the path a bare render takes). */
  anchorEl?: HTMLElement | null;
  pointerX?: number;
  tools: ReadingBlockTool[];
  notes: ReadingBlockNote[];
  /** A tool 印记 reached for. Runs itself once, so its teaching lands as
   *  teaching rather than as a button she has to find and press. */
  autoTool?: string | null;
  onAutoToolConsumed?: () => void;
  onNote: (note: ReadingBlockNote) => void;
  onClose: () => void;
  /** 这一段是第几段。交给 印记 的那一行说明里要用。 */
  ordinal?: number;
  /**
   * 她在想一想 / 仿写 底下写好了一段，交给 印记 要反馈。
   * 返回的 promise 在这一轮落地（或失败）时结束，true = 送到了。
   */
  onToolAnswer?: (answer: CoachCardAnswer) => Promise<boolean>;
}) {
  const [busy, setBusy] = useState<string | null>(null);
  const [open, setOpen] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  /**
   * 正在等她点一个句子的那件工具（今天只有语法）。
   *
   * 它和 `open` 是两个状态，不是一个：她可以在挑句子的时候改主意去点别的工具，
   * 而那时候屏幕上不该还留着上一件工具的讲解。
   */
  const [picking, setPicking] = useState<ReadingBlockTool | null>(null);
  /** 当前显示的是哪一句的讲解。空 = 整段那一份。 */
  const [shownSentence, setShownSentence] = useState("");

  const sentences = useMemo(() => segmentSentences(blockText), [blockText]);

  const noteFor = (tool: string, sentence: string) =>
    notes.find(
      (n) => n.blockId === blockId && n.tool === tool && (n.subject ?? "") === sentence,
    );

  async function ask(tool: ReadingBlockTool, sentence: string) {
    setPicking(null);
    if (noteFor(tool.id, sentence)) {
      setOpen(tool.id);
      setShownSentence(sentence);
      return;
    }
    setBusy(tool.id);
    setError(null);
    try {
      const note = await explainReadingBlock(readingId, blockId, tool.id, sentence);
      onNote({
        blockId: note.blockId,
        tool: note.tool,
        body: note.body,
        subject: note.subject ?? sentence,
        words: note.words,
        grammar: note.grammar,
      });
      setOpen(tool.id);
      setShownSentence(sentence);
    } catch (err) {
      setError(apiErrorText(err));
    } finally {
      setBusy(null);
    }
  }

  async function run(tool: ReadingBlockTool) {
    // Already open → collapse. Already fetched → just show it, no call.
    if (open === tool.id && !picking) {
      setOpen(null);
      return;
    }
    if (tool.subject === "word") {
      // 查词：这一段本身就是选择器，每个词点得动。见 wordTokens。
      setOpen(null);
      setError(null);
      setPicking(tool);
      return;
    }
    if (tool.subject === "sentence") {
      // 一句话的段落没什么可挑的 —— 直接讲那一句，别摆一张只有一个选项的单子。
      if (sentences.length <= 1) {
        await ask(tool, sentences[0]?.text.trim() ?? blockText.trim());
        return;
      }
      setOpen(null);
      setError(null);
      setPicking(tool);
      return;
    }
    await ask(tool, "");
  }

  // Run the coach's chosen tool once. Keyed on blockId+tool so moving to a new
  // paragraph re-arms it, and guarded by a ref so a re-render never fires the
  // same call twice — this is a metered call, not a render effect.
  //
  // DO NOT pair this latch with a `let cancelled = false` cleanup flag. Under
  // StrictMode the cleanup cancels the closure that fired the call while the
  // latch skips the remount, so the one real reply is thrown away and the
  // busy state never clears — the bug that hung 段落's guide box (write-up in
  // `src/shared/useAlive.ts`). `run()` sets state unconditionally,
  // which is why this site is correct as written; if it ever needs an unmount
  // guard, use `useAlive()`.
  const autoFired = useRef<string | null>(null);
  useEffect(() => {
    if (!autoTool) return;
    const key = `${blockId}:${autoTool}`;
    if (autoFired.current === key) return;
    const tool = tools.find((t) => t.id === autoTool);
    if (!tool) {
      onAutoToolConsumed?.();
      return;
    }
    autoFired.current = key;
    void run(tool).finally(() => onAutoToolConsumed?.());
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [autoTool, blockId, tools]);

  const shown = open ? noteFor(open, shownSentence) : undefined;
  const shownTool = tools.find((t) => t.id === open);
  const shownLabel = shownTool?.label ?? "";
  const opened = new Set(notes.filter((n) => n.blockId === blockId).map((n) => n.tool));

  return (
    <>
      {anchorEl && (
        <BlockToolbar
          anchorEl={anchorEl}
          pointerX={pointerX ?? 0}
          tools={tools}
          openedTools={opened}
          busyTool={busy}
          activeTool={picking?.id ?? open}
          onPick={(tool) => void run(tool)}
          onClose={onClose}
        />
      )}
      {/* 🚨 这一块里的点击不算「点在工具条外面」（BlockToolbar 的 data-block-tools 判据）。
          2026-09-17 入口走查：点一个句子、一个词、想一想的输入框、语法卡的层标签，
          都先触发了工具条的「点外面就关」—— 这一块跟着卸掉，讲解永远出不来。 */}
      <div data-block-tools>

        {error && (
          <p role="alert" className="mt-2 text-mk-small text-mk-danger">
            {error}
          </p>
        )}

        {/* 请她点一个句子。摆的是这一段切出来的真句子，不是一个输入框 ——
            她要讲的那一句本来就在眼前，让她重打一遍是多余的。 */}
        {picking && (
          <div className="mk-sentence-pick">
            <div className="mk-sentence-pick__head">
              <span className="text-mk-caption text-mk-accent-700">
                {picking.label} · {picking.subject === "word" ? "请点一个词" : "请选择一个句子"}
              </span>
              <button
                type="button"
                aria-label="取消选择句子"
                onClick={() => setPicking(null)}
                className="shrink-0 rounded-mk-xs p-0.5 text-mk-faint hover:text-mk-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
              >
                <Icon icon={X} size={14} />
              </button>
            </div>
            {picking.subject === "word" ? (
              <p className="mk-word-pick">
                {wordTokens(blockText).map((tok, i) =>
                  tok.word ? (
                    <button
                      key={i}
                      type="button"
                      className="mk-word-pick__word"
                      disabled={busy !== null}
                      onClick={() => void ask(picking, tok.text)}
                    >
                      {tok.text}
                    </button>
                  ) : (
                    <span key={i}>{tok.text}</span>
                  ),
                )}
              </p>
            ) : (
            <ul className="mk-sentence-pick__list">
              {sentences.map((s) => (
                <li key={s.start}>
                  <button
                    type="button"
                    className="mk-sentence-pick__item"
                    disabled={busy !== null}
                    onClick={() => void ask(picking, s.text.trim())}
                  >
                    {s.text.trim()}
                  </button>
                </li>
              ))}
            </ul>
            )}
          </div>
        )}

        {shown && (
          <div
            className="mt-2 rounded-mk-md border p-3.5"
            style={{
              borderColor: "var(--mk-accent-200)",
              background: "color-mix(in srgb, var(--mk-accent-500) 4%, var(--mk-paper))",
            }}
          >
            <div className="mb-2 flex items-center justify-between gap-2">
              <span className="text-mk-caption text-mk-accent-700">{shownLabel}</span>
              <button
                type="button"
                aria-label="收起这段讲解"
                onClick={() => setOpen(null)}
                className="shrink-0 rounded-mk-xs p-0.5 text-mk-faint hover:text-mk-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
              >
                <Icon icon={X} size={14} />
              </button>
            </div>
            {/* 讲的是哪一句，摆在讲解上面。她可能已经往下读了两段 —— 不说清楚
                这一份讲的是哪一句，她得自己回去找。 */}
            {shown.subject && !shown.grammar ? (
              <p className="mk-block-note__subject">{shown.subject}</p>
            ) : null}
            {shown.grammar && grammarHasContent(shown.grammar) && shown.subject ? (
              <GrammarCards sentence={shown.subject} grammar={shown.grammar} />
            ) : shown.words && shown.words.length > 0 ? (
              <WordCards words={shown.words} />
            ) : (
              <div className="text-mk-body leading-relaxed text-mk-ink">
                <ChatMarkdown text={shown.body} />
              </div>
            )}
            {onToolAnswer && (shown.tool === "questions" || shown.tool === "imitate") && (
              <ToolAnswerBox
                key={`${blockId}-${shown.tool}`}
                label={shownLabel}
                prompt={toolAnswerPrompt(shownLabel, ordinal, shown.body)}
                blockId={blockId}
                kind={shown.tool}
                onSend={onToolAnswer}
              />
            )}
          </div>
        )}
      </div>
    </>
  );
}

/**
 * 把一段英文切成「词」和「词之间的东西」，拼回去逐字等于原文。
 *
 * 「查词」那张选择器就是这一段本身：词是按钮，标点和空格原样摆着。
 * 一个词 = 字母开头，后面跟字母、撇号（it's / O’Connor）或连字符
 * （non-invasive）。数字不算词 —— 没有人会点「66」去查它是什么意思。
 */
export function wordTokens(text: string): { text: string; word: boolean }[] {
  const out: { text: string; word: boolean }[] = [];
  const re = /[A-Za-z][A-Za-z'’-]*[A-Za-z]|[A-Za-z]/g;
  let last = 0;
  for (const m of text.matchAll(re)) {
    const at = m.index ?? 0;
    if (at > last) out.push({ text: text.slice(last, at), word: false });
    out.push({ text: m[0], word: true });
    last = at + m[0].length;
  }
  if (last < text.length) out.push({ text: text.slice(last), word: false });
  return out;
}

/** 服务端给卡片题目的上限（reading_coach.go 的 collapseCardPrompt）。超过的整句被丢掉。 */
const TOOL_PROMPT_MAX = 60;

/**
 * 交给 印记 的那一行说明：「仿写 · 第3段：先给一个日常场景，再解释背后的原理」。
 *
 * 它会作为「【印记问】」那一行存进对话 —— **不算她说的话**。所以这里只放工具
 * 名、段号和那件工具自己的内容（想一想的问题 / 仿写的写法），她写的另外走。
 * 截到服务端的上限以内：超过的那一行服务端会整行丢掉，印记 就不知道她在答什么。
 */
export function toolAnswerPrompt(label: string, ordinal: number | undefined, body: string): string {
  const head = ordinal && ordinal > 0 ? `${label} · 第${ordinal}段：` : `${label}：`;
  const firstLine =
    body
      .split("\n")
      // 先去掉加粗的标签，再去掉列表记号：反过来的话，「- 」那条规则会先吃掉
      // 「**这一段的写法**」开头的那个 *，标签就再也认不出来了。
      .map((l) => l.replace(/\*\*这一段的写法\*\*：/, "").replace(/^[-*]\s+/, "").trim())
      .find((l) => l.length > 0) ?? "";
  const room = TOOL_PROMPT_MAX - Array.from(head).length;
  const chars = Array.from(firstLine);
  const tail = chars.length > room ? chars.slice(0, Math.max(0, room - 1)).join("") + "…" : firstLine;
  return head + tail;
}

/**
 * 想一想 / 仿写 底下那个框。
 *
 * 🚨 产品负责人 2026-09-17：「想一想 and 仿写 actually these are things that need
 * students' input. how should we do that? put a box there to invite students to
 * write and give feedbacks?」—— 这两件工具原来只给题目、不收答案，她想完、写完
 * 都没有地方放，印记 也永远不知道。
 *
 * 写好交给 印记：反馈出现在对话里，和别的每一轮一样被记下来（报告要用）。
 * 回车发送、Shift+回车换行、中文输入法选词不误发 —— 和另外两个框同一套手势。
 */
function ToolAnswerBox({
  label,
  prompt,
  blockId,
  kind,
  onSend,
}: {
  label: string;
  prompt: string;
  blockId: string;
  kind: string;
  onSend: (answer: CoachCardAnswer) => Promise<boolean>;
}) {
  const [draft, setDraft] = useState("");
  const [state, setState] = useState<"idle" | "sending" | "sent" | "failed">("idle");
  const [sentText, setSentText] = useState("");

  async function submit() {
    const text = draft.trim();
    if (!text || state === "sending") return;
    setState("sending");
    const ok = await onSend({ type: BLOCK_TOOL_ANSWER, prompt, choice: text, blockId });
    if (ok) {
      setSentText(text);
      setDraft("");
      setState("sent");
    } else {
      setState("failed");
    }
  }

  return (
    <div className="mk-tool-answer">
      {state === "sent" && (
        <p className="mk-tool-answer__sent">
          <span className="mk-tool-answer__sent-label">已发送给印记，反馈在对话里</span>
          <span className="whitespace-pre-wrap">{sentText}</span>
        </p>
      )}
      <textarea
        className="mk-tool-answer__input"
        rows={3}
        value={draft}
        disabled={state === "sending"}
        placeholder={
          kind === "imitate"
            ? `请用这一段的写法写一段（${label}，回车发送，Shift+回车换行）`
            : `请写下你的想法（${label}，回车发送，Shift+回车换行）`
        }
        onChange={(e) => setDraft(e.target.value)}
        onKeyDown={(e) => {
          if (e.key !== "Enter" || e.shiftKey) return;
          if (e.nativeEvent.isComposing) return;
          e.preventDefault();
          void submit();
        }}
      />
      <div className="mk-tool-answer__row">
        {state === "failed" && <span className="text-mk-small text-mk-danger">发送失败，请重试</span>}
        <button
          type="button"
          className="mk-tool-answer__send"
          disabled={!draft.trim() || state === "sending"}
          onClick={() => void submit()}
        >
          {state === "sending" ? "处理中" : "交给印记"}
        </button>
      </div>
    </div>
  );
}

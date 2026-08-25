import { useEffect, useRef, useState } from "react";
import { putBuffer, type WritingDocKind } from "../../api/writing";
import { getDraft } from "../api/workspace";
import { MarkdownPreview } from "./MarkdownPreview";
import { countWords } from "./wordcount";

// ProsePane (Phase B) — the plain prose writing surface for the 研究提案. Unlike
// the essay room (大纲/片段/正文), the proposal is a short prose document: a
// single textarea + a Markdown preview toggle + debounced autosave to that
// document's buffer. No outline, no snippets, no 整稿体检 panel — 印记 chats about
// the proposal through the shared coach rail, never rewrites it (铁律①).
//
// It is doc-parameterized (defaults to "proposal") so the buffer it loads/saves
// is the SAME per-doc buffer the backend keys on `doc_kind` — the proposal and
// the essay never collide.
export function ProsePane({
  projectId,
  doc = "proposal",
  locked = false,
  placeholder = "在这里写你的研究提案——你想探究什么、为什么值得、打算怎么做。\n\n支持 Markdown。写完后点上方的「完成提案」。",
  onSendToCoach,
}: {
  projectId: string;
  doc?: WritingDocKind;
  locked?: boolean;
  placeholder?: string;
  // Bug 5b · select-to-quote: hands the selected text UP to be staged as an
  // editable/cancelable quote above the shared composer (not sent immediately),
  // so the student can add their own words or cancel before asking 印记.
  onSendToCoach?: (text: string) => void;
}) {
  const [text, setText] = useState("");
  const [preview, setPreview] = useState(false);
  const [saveStatus, setSaveStatus] = useState<"saved" | "dirty" | "saving">("saved");
  const [selPop, setSelPop] = useState<{ x: number; y: number; text: string } | null>(null);
  // "上传写好的文档" — a student who wrote this doc elsewhere can drop it in
  // (mirrors the essay DraftPane). .md/.txt is read straight in; .docx/.pdf is
  // accepted with a note (parsing lands later).
  const [mode, setMode] = useState<"write" | "upload">("write");
  const [uploadNote, setUploadNote] = useState<string | null>(null);
  const fileInput = useRef<HTMLInputElement | null>(null);
  const textRef = useRef("");
  const dirtyRef = useRef(false);
  const savingRef = useRef(false);
  const saveTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const paneRef = useRef<HTMLDivElement | null>(null);
  // CJK-aware word count matching the backend (agent.CountWords), so English
  // proposals aren't over-counted ~5× vs the "字" target (BUG-02).
  const words = countWords(text);

  // slice 3b · select-to-send. On mouse-up over a selection, float a "问印记"
  // chip; clicking pins the selection into the coach thread (text only). Mirrors
  // the essay DraftPane's selPop.
  function onMouseUp(e: React.MouseEvent<HTMLTextAreaElement>) {
    if (locked || !onSendToCoach) return;
    const ta = e.currentTarget;
    const sel = ta.value.slice(ta.selectionStart, ta.selectionEnd).trim();
    if (!sel) {
      setSelPop(null);
      return;
    }
    const paneBox = paneRef.current?.getBoundingClientRect();
    setSelPop({ x: e.clientX - (paneBox?.left ?? 0), y: e.clientY - (paneBox?.top ?? 0), text: sel });
  }

  // Load the persisted proposal buffer on mount / doc change ("" when none yet).
  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const content = await getDraft(projectId, doc);
        if (!cancelled) {
          setText(content);
          textRef.current = content;
        }
      } catch {
        /* leave empty; the placeholder shows */
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [projectId, doc]);

  async function flushSave() {
    if (savingRef.current || !dirtyRef.current) return;
    savingRef.current = true;
    setSaveStatus("saving");
    const attempted = textRef.current;
    try {
      await putBuffer(projectId, attempted, doc);
      if (textRef.current === attempted) {
        dirtyRef.current = false;
        setSaveStatus("saved");
      } else {
        setSaveStatus("dirty");
        scheduleSave(600);
      }
    } catch {
      // Best-effort retry shortly; keep the local (dirty) text untouched.
      setSaveStatus("dirty");
      scheduleSave(3000);
    } finally {
      savingRef.current = false;
    }
  }

  function scheduleSave(delayMs: number) {
    if (saveTimer.current) clearTimeout(saveTimer.current);
    saveTimer.current = setTimeout(() => {
      void flushSave();
    }, delayMs);
  }

  // Periodic safety-net flush (continuous typing keeps resetting the debounce).
  useEffect(() => {
    const id = setInterval(() => {
      if (dirtyRef.current && !savingRef.current) void flushSave();
    }, 18000);
    return () => clearInterval(id);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [projectId, doc]);

  // Flush a pending autosave on unmount / doc switch so a last edit is never lost.
  useEffect(
    () => () => {
      if (saveTimer.current) clearTimeout(saveTimer.current);
      if (dirtyRef.current) {
        void putBuffer(projectId, textRef.current, doc).catch(() => {
          /* best-effort; nothing left to retry against */
        });
      }
    },
    [projectId, doc],
  );

  function onChange(next: string) {
    setText(next);
    textRef.current = next;
    dirtyRef.current = true;
    if (!savingRef.current) setSaveStatus("dirty");
    setSelPop(null); // an edit invalidates the pending selection chip
    scheduleSave(1200);
  }

  async function handleFile(file: File) {
    const name = file.name.toLowerCase();
    if (name.endsWith(".md") || name.endsWith(".markdown") || name.endsWith(".txt")) {
      const content = await file.text();
      onChange(content);
      setUploadNote(null);
      setMode("write");
    } else {
      // .docx / .pdf — accepted but not parsed yet; don't crash, just note it.
      setUploadNote(`已上传「${file.name}」，正文解析稍后支持。`);
    }
  }

  const saveLabel = saveStatus === "saving" ? "保存中…" : saveStatus === "dirty" ? "未保存" : "已保存 ✓";

  return (
    // §4: no top icon bar — word count + save status live in a lower-right
    // overlay, with an unobtrusive preview toggle beside them.
    <div ref={paneRef} className="relative flex h-full min-h-0 flex-col">
      {/* small "上传写好的文档" entry — top-right, unobtrusive (mirrors the essay) */}
      {!locked && (
        <button
          type="button"
          onClick={() => setMode(mode === "write" ? "upload" : "write")}
          className="absolute right-4 top-3 z-10 rounded-mk-md border border-mk-border bg-mk-surface px-3 py-1 text-[12px] font-semibold text-mk-muted shadow-mk-xs transition hover:text-mk-accent"
        >
          {mode === "write" ? "上传写好的文档" : "← 回到写作"}
        </button>
      )}
      {/* surface */}
      <div className="min-h-0 flex-1 overflow-auto px-8 py-5">
        {mode === "upload" ? (
          <div
            className="mx-auto flex h-full max-w-[70ch] flex-col items-center justify-center rounded-mk-lg border-2 border-dashed border-mk-input-border bg-mk-surface px-6 text-center"
            onDragOver={(e) => e.preventDefault()}
            onDrop={(e) => { e.preventDefault(); const f = e.dataTransfer.files[0]; if (f) void handleFile(f); }}
          >
            <p className="text-[15px] font-bold text-mk-ink">把你写好的提案拖进来</p>
            <p className="mt-1 text-[14px] text-mk-faint">Word / PDF / Markdown——印记读进来后，也能和你聊这一稿</p>
            <input
              ref={fileInput}
              type="file"
              accept=".md,.markdown,.txt,.docx,.pdf"
              className="hidden"
              onChange={(e) => { const f = e.target.files?.[0]; if (f) void handleFile(f); e.target.value = ""; }}
            />
            <button type="button" onClick={() => fileInput.current?.click()} className="mt-4 rounded-mk-md bg-mk-accent px-4 py-2 text-[14px] font-bold text-white hover:bg-mk-accent-600">选择文件</button>
            {uploadNote && <p className="mt-3 text-[14px] font-semibold text-mk-warning">{uploadNote}</p>}
          </div>
        ) : preview ? (
          <div className="mx-auto max-w-[70ch]">
            {text.trim() === "" ? (
              <p className="text-[14px] text-mk-muted">还没有内容可预览。</p>
            ) : (
              <MarkdownPreview text={text} />
            )}
          </div>
        ) : (
          <textarea
            value={text}
            onChange={(e) => onChange(e.target.value)}
            onMouseUp={onMouseUp}
            readOnly={locked}
            placeholder={placeholder}
            aria-label="研究提案正文"
            className="mx-auto block h-full w-full max-w-[70ch] resize-none bg-transparent text-[14.5px] leading-relaxed text-mk-ink outline-none placeholder:text-mk-muted"
          />
        )}
        {/* slice 3b · the floating 问印记 chip next to a selection */}
        {selPop && mode === "write" && !preview && !locked && onSendToCoach && (
          <button
            type="button"
            onMouseDown={(e) => e.preventDefault()} // keep the textarea selection alive
            onClick={() => { onSendToCoach(selPop.text); setSelPop(null); }}
            style={{ left: selPop.x, top: selPop.y + 8 }}
            className="absolute z-10 rounded-mk-md bg-mk-accent px-2.5 py-1 text-[12px] font-bold text-white shadow-mk-md hover:bg-mk-accent-600"
          >
            问印记
          </button>
        )}
      </div>

      {/* lower-right status overlay (§4: word count + save status, right lower corner) */}
      <div className="pointer-events-none absolute bottom-3 right-4 flex items-center gap-3 rounded-mk-md border border-mk-border bg-mk-surface px-2.5 py-1 text-[12px] text-mk-muted shadow-mk-xs">
        <button
          type="button"
          onClick={() => setPreview((p) => !p)}
          className="pointer-events-auto font-bold text-mk-muted hover:text-mk-accent"
          title="切换 Markdown 预览"
        >
          {preview ? "编辑" : "预览"}
        </button>
        <span>{words} 字</span>
        <span>{saveLabel}</span>
      </div>
    </div>
  );
}

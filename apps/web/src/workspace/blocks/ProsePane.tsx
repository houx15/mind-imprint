import { useEffect, useRef, useState } from "react";
import { putBuffer, type WritingDocKind } from "../../api/writing";
import { getDraft } from "../api/workspace";
import { MarkdownPreview } from "./MarkdownPreview";

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
}: {
  projectId: string;
  doc?: WritingDocKind;
  locked?: boolean;
  placeholder?: string;
}) {
  const [text, setText] = useState("");
  const [preview, setPreview] = useState(false);
  const [saveStatus, setSaveStatus] = useState<"saved" | "dirty" | "saving">("saved");
  const textRef = useRef("");
  const dirtyRef = useRef(false);
  const savingRef = useRef(false);
  const saveTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const words = text.replace(/\s+/g, "").length;

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
    scheduleSave(1200);
  }

  const saveLabel = saveStatus === "saving" ? "保存中…" : saveStatus === "dirty" ? "未保存" : "已保存 ✓";

  return (
    <div className="flex h-full min-h-0 flex-col">
      {/* toolbar */}
      <div className="flex items-center gap-3 border-b border-mk-border bg-mk-surface px-8 py-2">
        <button
          type="button"
          onClick={() => setPreview((p) => !p)}
          className="rounded-mk-md border border-mk-border px-2.5 py-1 text-[12px] font-bold text-mk-muted hover:text-mk-accent"
          title="切换 Markdown 预览"
        >
          {preview ? "编辑" : "预览"}
        </button>
        <span className="text-[12px] text-mk-muted">{words} 字</span>
        <span className="ml-auto text-[12px] text-mk-muted">{saveLabel}</span>
      </div>

      {/* surface */}
      <div className="min-h-0 flex-1 overflow-auto px-8 py-5">
        {preview ? (
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
            readOnly={locked}
            placeholder={placeholder}
            aria-label="研究提案正文"
            className="mx-auto block h-full w-full max-w-[70ch] resize-none bg-transparent text-[14.5px] leading-relaxed text-mk-ink outline-none placeholder:text-mk-muted"
          />
        )}
      </div>
    </div>
  );
}

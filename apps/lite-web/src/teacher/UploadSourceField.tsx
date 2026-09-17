import { useState } from "react";
import { extractDocument } from "../api/writings";
import { useAlive } from "../shared/useAlive";
import { extractedCountText, failText, readExtractResult } from "./assignmentLogic";
import { INPUT_CLS, LABEL_CLS } from "./formParts";

/**
 * UploadSourceField — 上传文件 for a reading homework. The browser posts the
 * file to /documents/extract (lite edition, any role, 30 MB); the text lands
 * in the text source and nothing else is stored. The extracted text stays
 * editable so the teacher can check it before 发布.
 */
export function UploadSourceField({
  text,
  fileName,
  onExtracted,
  onTextChange,
}: {
  text: string;
  fileName: string;
  onExtracted: (r: { text: string; fileName: string; title: string }) => void;
  onTextChange: (text: string) => void;
}) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const alive = useAlive();

  async function extract(file: File) {
    setBusy(true);
    setError(null);
    try {
      const out = readExtractResult(await extractDocument(file), file.name);
      if (!alive.current) return;
      if (out.ok) onExtracted(out);
      else setError(out.error);
    } catch (e) {
      if (alive.current) setError(failText("提取", e));
    } finally {
      if (alive.current) setBusy(false);
    }
  }

  return (
    <div className="flex flex-col gap-3">
      <label className="flex flex-col gap-1.5">
        <span className={LABEL_CLS}>文件</span>
        <input
          type="file"
          accept=".pdf,.docx,.txt,.md"
          disabled={busy}
          onChange={(e) => {
            const file = e.target.files?.[0];
            // Cleared so choosing the same file again runs the extraction again.
            e.target.value = "";
            if (file) void extract(file);
          }}
          className="tc-file"
        />
      </label>
      <p className="text-mk-small text-mk-muted">
        支持 PDF、Word（.docx）、TXT、Markdown，不超过 30 MB。PDF 中的图片不会保留；扫描版 PDF 没有文字，无法提取。
      </p>
      {busy && <p className="text-mk-small text-mk-muted">处理中</p>}
      {error && (
        <p role="alert" className="break-words text-mk-small font-semibold text-mk-danger">
          {error}
        </p>
      )}
      {!busy && fileName && text.trim() && (
        <p className="text-mk-small text-mk-ink">
          {fileName} · {extractedCountText(text)}
        </p>
      )}
      {fileName && (
        <label className="flex flex-col gap-1.5">
          <span className={LABEL_CLS}>正文</span>
          <textarea value={text} onChange={(e) => onTextChange(e.target.value)} rows={10} className={INPUT_CLS} />
        </label>
      )}
    </div>
  );
}

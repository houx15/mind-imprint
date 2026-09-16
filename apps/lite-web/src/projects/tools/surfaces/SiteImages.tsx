import { Says, errorMarkdown } from "../../Says";
import { useState } from "react";
import { uploadUserImage } from "../../../api/oss";
import { putSiteSectionImage, type SiteState } from "../../../api/site";
import { apiErrorText } from "../../../api/errorText";

export function SiteImages({ site, onChange, onBusy }: { site: SiteState; onChange: (site: SiteState) => void; onBusy: (busy: boolean) => void }) {
  const [selected, setSelected] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const sections = site.content.sections ?? [];
  const key = sections.some(s => s.key === selected) ? selected : sections[0]?.key ?? "";
  const section = sections.find(s => s.key === key);
  if (!sections.length) return null;
  async function save(file?: File) {
    if (!key) return;
    setBusy(true); onBusy(true); setError(null);
    try {
      const objectKey = file ? await uploadUserImage(file) : "";
      onChange(await putSiteSectionImage(key, objectKey));
    } catch (err) { setError(apiErrorText(err)); }
    finally { setBusy(false); onBusy(false); }
  }
  return <section className="mt-5">
    <h3 className="text-mk-label text-mk-secondary">模块图片</h3>
    <p className="mt-1 text-mk-small text-mk-muted">请为模块选择自己的图片。图片说明可在项目对话中补充。</p>
    <select aria-label="图片所属模块" value={key} disabled={busy} onChange={e => setSelected(e.target.value)} className="mt-2 w-full rounded-mk-md border border-mk-border bg-mk-surface p-2">
      {sections.map(s => <option key={s.key} value={s.key}>{s.title}</option>)}
    </select>
    <label className="mt-2 block text-mk-small">
      {busy ? "图片处理中" : section?.imageKey ? "更换图片" : "上传图片"}
      <input aria-label="上传模块图片" type="file" accept="image/png,image/jpeg,image/webp,image/gif" disabled={busy} className="mt-1 block w-full text-mk-small" onChange={e => { const file = e.target.files?.[0]; e.target.value = ""; if (file) void save(file); }} />
    </label>
    {section?.imageKey && <button type="button" disabled={busy} onClick={() => void save()} className="mt-2 text-mk-small underline">移除图片</button>}
    {error && <div role="alert" className="mt-2 text-mk-small" style={{ color: "var(--mk-danger)" }}><Says content={errorMarkdown(error)} /></div>}
  </section>;
}

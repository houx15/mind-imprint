import { useEffect, useRef, useState } from "react";
import { ImagePlus, Sparkles, Loader2 } from "lucide-react";
import { apiErrorText } from "../api/errorText";
import { uploadUserImage } from "../api/oss";
import { generateShowcaseImage, resolveShowcaseImage } from "../api/showcase";
import { GrowingTextarea } from "../shared/GrowingTextarea";
import { Says, errorMarkdown } from "../projects/Says";

/** Image operations create an owned asset. Applying it changes the local draft only. */
export function ShowcaseImagePicker({purpose, currentUrl, prompt, onPromptChange, disabled, onBusy, onPick}: {
  purpose: "hero" | "avatar";
  currentUrl?: string;
  prompt: string;
  onPromptChange: (prompt: string) => void;
  disabled: boolean;
  onBusy: (busy: boolean) => void;
  onPick: (key: string, url: string) => void;
}) {
  const input = useRef<HTMLInputElement>(null);
  const lock = useRef(false);
  const [generating, setGenerating] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [error, setError] = useState("");
  const [imageFailed, setImageFailed] = useState(false);
  useEffect(() => setImageFailed(false), [currentUrl]);
  async function upload(file: File) {
    if (lock.current) return;
    if (!["image/png", "image/jpeg", "image/webp"].includes(file.type) || file.size > 10 * 1024 * 1024) {
      setError("请选择 10 MB 以内的 PNG、JPEG 或 WebP 图片。"); return;
    }
    lock.current = true; setUploading(true); onBusy(true); setError("");
    try { const key = await uploadUserImage(file); const url = await resolveShowcaseImage(key); onPick(key, url); }
    catch (err) { setError(`上传失败：${apiErrorText(err)}`); }
    finally { lock.current = false; setUploading(false); onBusy(false); }
  }
  async function generate() {
    if (!prompt.trim() || disabled || lock.current) return;
    lock.current = true; setGenerating(true); onBusy(true); setError("");
    try { const result = await generateShowcaseImage(prompt.trim(), purpose); onPick(result.objectKey, result.url); }
    catch (err) { setError(`生成失败：${apiErrorText(err)}`); }
    finally { lock.current = false; setGenerating(false); onBusy(false); }
  }
  return <section className="showcase-image-picker">
    {currentUrl && !imageFailed && <img onError={() => setImageFailed(true)} src={currentUrl} alt={purpose === "hero" ? "已选开场图片" : "已选个人图片"} className={purpose === "avatar" ? "is-avatar" : ""} />}
    {imageFailed && <p role="alert" className="showcase-image-error">图片加载失败，请刷新后重试或替换图片。</p>}
    <input type="file" accept="image/png,image/jpeg,image/webp" ref={input} hidden onChange={e => {const file = e.target.files?.[0]; e.target.value = ""; if (file) void upload(file);}} />
    <div className="showcase-image-actions"><button type="button" disabled={disabled} onClick={() => input.current?.click()}><ImagePlus size={15} />{uploading ? "上传中" : purpose === "hero" ? "上传开场图片" : "上传个人照片"}</button>{currentUrl && <button type="button" disabled={disabled} onClick={() => onPick("", "")}>移除图片</button>}</div>
    <small>支持 PNG、JPEG、WebP，最大 10 MB。</small>
    <p>{purpose === "hero" ? "首图预设：横向画面，适合全屏展示，标题区域留白。" : "头像预设：方形画面，主体居中，适合圆形裁切。"}系统会补充尺寸与构图要求；原始描述随草稿保存。</p>
    <label className="showcase-field">图片描述<GrowingTextarea value={prompt} disabled={disabled} maxLength={2000} rows={2} onChange={e => onPromptChange(e.target.value)} placeholder={purpose === "hero" ? "例如：极简的蓝色天空，一颗漂浮的星球，画面中央留白。" : "例如：戴着耳机的原创小机器人，圆形头像，浅色背景。"} /></label>
    <button type="button" className="showcase-generate" disabled={disabled || !prompt.trim()} onClick={() => void generate()}>{generating ? <Loader2 size={15} className="animate-spin" /> : <Sparkles size={15} />}{generating ? "图片生成中" : "生成图片"}</button>
    <p>图片描述会发送给平台配置的图像模型。生成或上传后，请预览并保存草稿；发布前仅自己可见。</p>
    {error && <div role="alert" className="showcase-image-error"><Says content={errorMarkdown(error)} /></div>}
  </section>;
}

import { GrowingTextarea } from "../../../shared/GrowingTextarea";
import { Says, errorMarkdown } from "../../Says";
import { apiErrorText } from "../../../api/errorText";
import { useEffect, useRef, useState } from "react";
import { Camera, Loader2, Plus, X } from "lucide-react";
import { Icon } from "@/ui";
import {
  NOTE_KINDS,
  NOTE_KIND_HINTS,
  listNotes,
  moveNote,
  noteKindMeta,
  type NoteKind,
} from "../../../api/notes";
import { ToolFrame } from "../ToolFrame";
import { boardSpot } from "../boardLayout";
import { useObservationDraft } from "../../useObservationDraft";
import { uploadUserImage, resolveUrl } from "../../../api/oss";
import { listMission, type MissionItem } from "../../../api/mission";
import type { ToolSurfaceProps } from "../registry";

/**
 * Observe —— 出去看看，然后回来。
 *
 * 产品负责人 2026-09-01：「The AI first explains how to generate question from
 * observation, with a simple observation method and gives them a small
 * real-world mission. Students return with information」。
 *
 * 这是一件要离开屏幕的工具，所以它的界面只在**回来之后**出现。印记交代的事
 * 就是 tool.reason，一直摆在最上面——她回来时多半已经忘了当时要她看什么。
 *
 * 带回来的东西直接变成便签，因为下一步就是把它们摊到板上。中间不该再有一次
 * 复制粘贴。
 */

interface Brought {
  kind: NoteKind;
  body: string;
  /** 这一行是从清单上哪一条来的。空 = 她自己加的一行。 */
  from?: string;
  /** 这一条带回来的照片在 OSS 里的 key，空 = 没拍。 */
  imageKey: string;
  /** 本地预览用的 blob URL。只活在这一次填写里，不进库。 */
  preview: string;
}

export function Observe({ projectId, tool, onFinish, onClose }: ToolSurfaceProps) {
  const draft = useObservationDraft(projectId, tool.id);
  const [previews, setPreviews] = useState<Record<string, string>>({});
  const [submitting, setSubmitting] = useState(false);
  const items: Brought[] = (draft.document.length ? draft.document : [{kind: "observation" as const, body: "", imageKey: ""}]).map(row => ({...row, preview: previews[row.imageKey] || ""}));
  const latestItems = useRef(items); latestItems.current = items;
  const setItems = (update: (rows: Brought[]) => Brought[]) => {
    const next = update(latestItems.current); latestItems.current = next;
    draft.change(next.map(({preview: _preview, ...row}) => row));
  };
  useEffect(() => {
    let live = true;
    for (const row of draft.document) if (row.imageKey && !previews[row.imageKey]) {
      void resolveUrl(row.imageKey).then(url => {if(live) setPreviews(old=>({...old,[row.imageKey]:url}));}).catch(()=>{});
    }
    return ()=>{live=false};
  }, [draft.document, previews]);
  const [error, setError] = useState<string | null>(null);
  // 正在上传的是第几条。同时只让传一张——她一条一条填，不需要并发。
  const [uploading, setUploading] = useState<number | null>(null);
  // 出门清单。回来时点掉的那几条已经是填好类别的底稿——她不用面对空白框。
  const [mission, setMission] = useState<MissionItem[]>([]);

  useEffect(() => {
    if (!draft.loaded) return;
    let alive = true;
    void listMission(projectId, tool.id)
      .then((ms) => {
        if (!alive || ms.length === 0) return;
        setMission(ms);
        // 🚨 只把**点掉的**那几条变成底稿。没做到的那几条不该在这里冒出一个
        // 空框等她补——她没做到就是没做到，那件事由印记接着问（回灌里有）。
        const seeded = ms
          .filter((m) => m.doneAt && !m.supersededAt)
          .map((m) => ({
            kind: (m.wantKind || "observation") as NoteKind,
            body: "",
            imageKey: "",
            preview: "",
            from: m.prompt,
          }));
        if (seeded.length > 0 && !latestItems.current.some(row => row.body || row.imageKey || row.from)) setItems(() => seeded);
      })
      .catch(() => {
        // 清单拉不到就退回原来的样子：一条空白的记录行，仍然能用。
      });
    return () => {
      alive = false;
    };
  }, [projectId, tool.id, draft.loaded]);

  const filled = items.filter((i) => i.body.trim());

  /**
   * 她拍的那张照片。
   *
   * 🚨 设计文档要的是「a photo, voice note, drawing, or short paragraph」，
   * 一直只做了最后一样。OSS 2026-07-27 就上线了，缺的只是这一段接线。
   *
   * 先上传拿 key，再把本地 blob 当预览挂上——预览只活在这一次填写里，存进库的
   * 永远是 key（签出来的 URL 会过期）。
   */
  async function attach(i: number, file: File) {
    setUploading(i);
    setError(null);
    try {
      const key = await uploadUserImage(file);
      setPreviews(old => ({...old, [key]: URL.createObjectURL(file)}));
      set(i, { imageKey: key });
    } catch (err) {
      setError(apiErrorText(err));
    } finally {
      setUploading(null);
    }
  }

  function set(i: number, patch: Partial<Brought>) {
    setItems((prev) => prev.map((it, n) => (n === i ? { ...it, ...patch } : it)));
  }

  async function finish() {
    if (submitting || uploading !== null) return;
    setSubmitting(true);
    try {
      const {notes: made, submittedRevision} = await draft.submit();
      // 🚨 带回来的便签要各占一格。createNotes 不给坐标，于是它们全落在
      // (0,0)——2026-09-02 线上实测：带回两条，板上看起来只有一张，另一张
      // 严丝合缝压在下面，她既看不见也拖不出来。板子上手动加的那条走的是
      // Board.add()，它会派位子；这条路原来没有。
      const existing = await listNotes(projectId);
      let seat = Math.max(0, existing.length - made.length);
      for (const n of made) {
        const spot = boardSpot(seat++);
        try {
          await moveNote(projectId, n.id, spot.x, spot.y);
        } catch {
          // 位子没排上不该让整件事失败——便签本身已经存下来了。
        }
      }
      // 便签已经贴在板上了，印记看得见。不用我们再替她复述一遍。
      onFinish({ brought: made.length, observationRevision: submittedRevision }, "");
    } catch (err) {
      setError(apiErrorText(err));
    } finally { setSubmitting(false); }
  }

  return (
    <ToolFrame
      title={tool.label}
      task="请逐条记录观察、原话、推论或问题"
      why={tool.reason}
      todo={filled.length === 0 ? "至少带回来一条" : ""}
      finishLabel="提交记录并讨论"
      onFinish={() => void finish()}
      busy={!draft.loaded || submitting || uploading !== null}
      onClose={() => { if (submitting || uploading !== null) return; void draft.flush().then(onClose).catch(()=>{}); }}
    >
      <p className="mb-3 text-mk-small text-mk-secondary">{!draft.loaded ? (draft.error ? "草稿读取失败" : "正在读取草稿…") : draft.saving ? "草稿保存中" : draft.dirty ? "草稿待保存" : "草稿已保存"} · 提交后才会加入便签板</p>
      {draft.error && <section role="alert" className="mb-3 space-y-2"><Says content={errorMarkdown(draft.error)}/><button className="rounded-mk-md border border-mk-border px-3 py-2 text-mk-small" onClick={()=>void draft.inspect()}>读取已保存草稿</button>{draft.remote && <><div className="grid gap-3" style={{gridTemplateColumns:"repeat(auto-fit, minmax(min(100%, 220px), 1fr))"}}>{[{title:"当前输入",rows:draft.document},{title:"已保存草稿",rows:draft.remote.document}].map(version=><section key={version.title} className="min-w-0 rounded-mk-md border border-mk-border bg-mk-surface p-3"><h3 className="mb-2 font-semibold">{version.title}</h3>{version.rows.length===0?<p>无记录</p>:version.rows.map((row,index)=><div key={index} className="border-t border-mk-border py-2 first:border-0"><p className="text-mk-small text-mk-secondary">{noteKindMeta(row.kind).label}</p><p className="whitespace-pre-wrap break-words">{row.body||"未填写正文"}</p>{row.from&&<p className="mt-1 text-mk-small text-mk-secondary">任务：{row.from}</p>}{row.imageKey&&<p className="mt-1 text-mk-small text-mk-secondary">已关联照片</p>}</div>)}</section>)}</div><div className="flex flex-wrap gap-2"><button className="rounded-mk-md border border-mk-border px-3 py-2 text-mk-small" onClick={()=>void draft.resolve(false)}>使用已保存草稿</button><button className="rounded-mk-md border border-mk-border px-3 py-2 text-mk-small" onClick={()=>void draft.resolve(true)}>保留当前输入并保存</button></div></>}</section>}
      <fieldset disabled={!draft.loaded || submitting || uploading !== null}>
      {error && (
        <div className="mb-2 text-mk-small" style={{ color: "var(--mk-danger)" }}>
          <Says content={errorMarkdown(error)} />
        </div>
      )}

      {/* 🚨 带回来的东西分四类，缺哪一类一眼看得出来。
          观察一趟只带回自己的推论，是这件工具最常见的失败法——她"看"了，
          但没带回任何一句别人的原话、任何一个数字。这条计数条就是那面镜子。 */}
      <p className="mb-2 text-mk-small font-semibold text-mk-secondary">本次待提交</p>
      <p className="mb-3 text-mk-small text-mk-muted">已提交的记录保存在项目材料的便签板中。</p>
      <div className="mb-3 flex flex-wrap gap-1.5">
        {NOTE_KINDS.filter((k) => k.kind !== "idea").map((k) => {
          const n = filled.filter((i) => i.kind === k.kind).length;
          return (
            <span
              key={k.kind}
              className="flex items-center gap-1 rounded-mk-full px-2 py-0.5 text-mk-small"
              style={{
                background: n > 0 ? `color-mix(in srgb, ${k.hue} 16%, transparent)` : "transparent",
                border: n > 0 ? `1px solid ${k.hue}` : "1px dashed var(--mk-border)",
                color: n > 0 ? "var(--mk-ink)" : "var(--mk-faint)",
              }}
            >
              {k.label}
              <b style={{ color: n > 0 ? k.hue : "var(--mk-faint)" }}>{n}</b>
            </span>
          );
        })}
      </div>
      {filled.length > 0 && filled.every((i) => i.kind === "assumption") && (
        <p
          className="mb-3 rounded-mk-md px-3 py-2 text-mk-small"
          style={{ background: "var(--mk-warning-bg)", color: "var(--mk-ink)" }}
        >
          当前记录全部为推论。推论需要事实支撑，请补充一条实际观察或一句原话。
        </p>
      )}

      {/* 🚨 没做到的那几条照实摆出来，不催也不空一个框等她补。
          「第 2 条没做到」是信号，不是过失——常常说明那一条本来就不现实。 */}
      {mission.some((m) => !m.doneAt && !m.supersededAt) && (
        <div
          className="mb-3 rounded-mk-md px-3 py-2"
          style={{ background: "var(--mk-paper)" }}
        >
          <p className="text-mk-small text-mk-secondary">尚未完成的观察任务</p>
          {mission
            .filter((m) => !m.doneAt && !m.supersededAt)
            .map((m) => (
              <p key={m.id} className="mt-0.5 text-mk-small text-mk-muted">
                {m.prompt}
              </p>
            ))}
        </div>
      )}

      <div className="space-y-2">
        {items.map((it, i) => (
          <div
            key={i}
            className="rounded-mk-md border p-2"
            // 整块淡底带出这一类的颜色，不挂左侧色条。
            style={{
              borderColor: "transparent",
              background: `color-mix(in srgb, ${noteKindMeta(it.kind).hue} 12%, var(--mk-surface))`,
            }}
          >
            <div className="flex items-start justify-between gap-2">
              <div className="flex flex-wrap gap-1">
                {NOTE_KINDS.filter((k) => k.kind !== "idea").map((k) => (
                  <button
                    key={k.kind}
                    type="button"
                    aria-pressed={it.kind === k.kind}
                    onClick={() => set(i, { kind: k.kind })}
                    className="rounded-mk-full px-2 py-0.5 text-mk-small"
                    style={
                      it.kind === k.kind
                        ? { background: k.hue, color: "#fff" }
                        : {
                            background: `color-mix(in srgb, ${k.hue} 14%, transparent)`,
                            color: "var(--mk-secondary)",
                          }
                    }
                  >
                    {k.label}
                  </button>
                ))}
              </div>
              {items.length > 1 && (
                <button
                  type="button"
                  aria-label="去掉这条"
                  onClick={() => setItems((prev) => prev.filter((_, n) => n !== i))}
                  className="shrink-0 text-mk-faint"
                >
                  <Icon icon={X} size={14} />
                </button>
              )}
            </div>
            {/* 这一行是清单上哪一条——她在现场点掉的那一句，原样摆在上面。 */}
            {it.from && (
              <p className="mt-1 text-mk-small text-mk-secondary">{it.from}</p>
            )}
            {/* 选中哪一类，就说清楚这一类怎么写才算写对了。 */}
            <p className="mt-1.5 text-mk-small text-mk-muted">{NOTE_KIND_HINTS[it.kind].how}</p>

            {/* 🚨 拍一张。设计文档要的是「a photo, voice note, drawing, or
                short paragraph」——一张照片常常比她写的三行字还实在，而且回来
                之后她自己也需要它来回忆当时看到了什么。 */}
            {it.preview ? (
              <div className="relative mt-1.5">
                <img
                  src={it.preview}
                  alt="你拍的"
                  className="max-h-40 w-full rounded-mk-sm object-cover"
                />
                <button
                  type="button"
                  aria-label="重新选择"
                  onClick={() => set(i, { imageKey: "", preview: "" })}
                  className="absolute right-1.5 top-1.5 rounded-mk-full px-2 py-0.5 text-mk-small"
                  style={{ background: "var(--mk-surface)", color: "var(--mk-secondary)" }}
                >
                  重新选择
                </button>
              </div>
            ) : (
              <label
                className="mt-1.5 flex cursor-pointer items-center justify-center gap-1.5 rounded-mk-sm border border-dashed border-mk-border py-1.5 text-mk-small text-mk-secondary"
              >
                <Icon icon={uploading === i ? Loader2 : Camera} size={14} />
                {uploading === i ? "上传中" : "添加照片"}
                <input
                  type="file"
                  accept="image/png,image/jpeg,image/webp"
                  className="hidden"
                  disabled={uploading !== null}
                  onChange={(e) => {
                    const f = e.target.files?.[0];
                    // 清掉 value，同一张照片再选一次也要能触发。
                    e.target.value = "";
                    if (f) void attach(i, f);
                  }}
                />
              </label>
            )}
            <GrowingTextarea
              aria-label={`${noteKindMeta(it.kind).label} ${i + 1}`}
              value={it.body}
              onChange={(e) => set(i, { body: e.target.value })}
              rows={2}
              placeholder={NOTE_KIND_HINTS[it.kind].placeholder}
              className="mt-1.5 w-full resize-none rounded-mk-sm border border-mk-input-border bg-mk-surface px-2 py-1.5 text-mk-small text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent-200"
            />
          </div>
        ))}
      </div>

      <button
        type="button"
        onClick={() => setItems((prev) => [...prev, { kind: "observation", body: "", imageKey: "", preview: "" }])}
        className="mt-2 flex w-full items-center justify-center gap-1 rounded-mk-md border border-dashed border-mk-border py-2 text-mk-small text-mk-secondary"
      >
        <Icon icon={Plus} size={14} />
        再加一条
      </button>

      </fieldset>
      <div className="mt-4 border-t border-mk-border pt-3">
        <p className="text-mk-small text-mk-muted">提交记录后可以继续观察。结束本次观察后，记录仍会保留。</p>
        <button type="button" disabled={!draft.loaded || submitting || uploading !== null || filled.length > 0} onClick={() => { void draft.flush().then(() => onFinish({endObservation: true}, "")).catch(() => {}); }} className="mt-2 rounded-mk-md border border-mk-border px-3 py-2 text-mk-small disabled:opacity-40">结束本次观察</button>
        {filled.length > 0 && <p className="mt-1 text-mk-small text-mk-muted">请先提交当前记录。</p>}
      </div>
      {/* 照片已经能带了（走 OSS，库里存 key）。录音和涂鸦还没做。 */}
    </ToolFrame>
  );
}

import { useCallback, useEffect, useState } from "react";
import { Footprints, PanelRight } from "lucide-react";
import { Icon } from "@/ui";
import type { ToolInstance } from "../../api/tools";
import { TOOL_TASKS } from "./registry";
import { editMission, listMission, tickMission, type MissionItem } from "../../api/mission";
import { GrowingTextarea } from "../../shared/GrowingTextarea";
import { beforeNavigate } from "../../routing";
import { noteKindMeta } from "../../api/notes";
import { Says, errorMarkdown } from "../Says";

/**
 * ToolInvite —— 印记把一件工具递到对话里。
 *
 * 邀请，不是弹窗（铁律②）。她可以不打开，而且**不打开必须和打开一样容易**：
 * 「跳过」是一次点击，不问理由。如果拒绝比接受费事，那就不是一个真的选择。
 *
 * world 工具（出门做的事）长得不一样：打开它不会跳出一个面板，而是把事情
 * 交给她，然后让开。
 */
export function ToolInvite({
  tool,
  onOpen,
  onDecline,
  busy,
}: {
  tool: ToolInstance;
  onOpen: () => void;
  onDecline: (note: string) => void;
  busy?: boolean;
}) {
  const [note, setNote] = useState("");
  const [declining, setDeclining] = useState(false);
  const away = tool.kind === "world";
  const task = TOOL_TASKS[tool.tool] ?? "";

  if (declining) {
    return (
      <div className="mt-2 rounded-mk-md border border-mk-border bg-mk-surface px-3 py-2.5">
        <p className="text-mk-small text-mk-secondary">请阐述原因（可不填）</p>
        <div className="mt-2 flex items-end gap-2">
          <input
            value={note}
            onChange={(e) => setNote(e.target.value)}
            placeholder="跳过的原因"
            className="flex-1 rounded-mk-md border border-mk-input-border bg-mk-surface px-2.5 py-1.5 text-mk-small text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent-200"
          />
          <button
            type="button"
            disabled={busy}
            onClick={() => onDecline(note.trim())}
            className="shrink-0 rounded-mk-full border border-mk-border px-3 py-1.5 text-mk-small text-mk-secondary disabled:opacity-40"
          >
            确认
          </button>
        </div>
      </div>
    );
  }

  return (
    <div
      // e2e 靠它找到这张卡。按文字找会随着文案改动而碎，而这张卡上每一句话
      // 都是会改的。
      data-testid={`tool-invite-${tool.tool}`}
      className="mt-2 rounded-mk-md border border-mk-border bg-mk-surface px-3 py-2.5"
    >
      <div className="flex items-start gap-2">
        <Icon
          icon={away ? Footprints : PanelRight}
          size={15}
          className="mt-0.5 shrink-0 text-mk-accent-500"
        />
        <div className="min-w-0">
          <p className="text-mk-small">
            <span className="font-semibold text-mk-ink">{tool.label}</span>
            {task && <span className="ml-1.5 text-mk-secondary">{task}</span>}
          </p>
          {/* 印记自己的话，不是模板。 */}
          <p className="mt-1 text-mk-small text-mk-muted">{tool.reason}</p>
          {away && (
            <p className="mt-1 text-mk-small text-mk-faint">请在合适的地方完成这项任务，完成后回来继续项目</p>
          )}
        </div>
      </div>
      <div className="mt-2.5 flex gap-2">
        <button
          type="button"
          disabled={busy}
          onClick={onOpen}
          className="rounded-mk-full px-3.5 py-1.5 text-mk-small text-white disabled:opacity-40"
          style={{ background: "var(--mk-accent-500)" }}
        >
          {away ? "接受任务" : "开始任务"}
        </button>
        <button
          type="button"
          disabled={busy}
          onClick={() => setDeclining(true)}
          className="rounded-mk-full border border-mk-border px-3.5 py-1.5 text-mk-small text-mk-secondary disabled:opacity-40"
        >
          跳过
        </button>
      </div>
    </div>
  );
}

/**
 * AwayCard —— 她答应了、人不在的那件事。
 *
 * 🚨 这张卡不能长得像待办。一件出门做的事挂上三天是正常的，显示成"未完成"
 * 就变成了催促，而催促正是这个产品说自己不做的事。
 */
export function AwayCard({
  projectId,
  tool,
  onBack,
  busy,
}: {
  projectId: string;
  tool: ToolInstance;
  onBack: () => void;
  busy?: boolean;
}) {
  // 🚨 清单要摆在这张卡上，因为这是她几天后回来时唯一还看得到的东西——也是她
  // 在现场掏出手机时看的那一屏。一条一条点掉，就是那个「简单的观察方法」。
  const [items, setItems] = useState<MissionItem[]>([]);
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);
  const [editing, setEditing] = useState<{item: MissionItem; text: string} | null>(null);
  const unsavedEdit = editing !== null && editing.text !== editing.item.prompt;
  const editSaving = editing !== null && saving;
  useEffect(() => {
    if (!unsavedEdit && !editSaving) return;
    const removeGuard = beforeNavigate(async () => {
      setError("请先保存或取消任务修改，再离开页面。");
      throw new Error("unsaved mission edit");
    });
    const warn = (event: BeforeUnloadEvent) => event.preventDefault();
    window.addEventListener("beforeunload", warn);
    return () => { removeGuard(); window.removeEventListener("beforeunload", warn); };
  }, [unsavedEdit, editSaving]);
  async function saveEdit() {
    if (!editing || saving || busy) return;
    setSaving(true);
    try {
      setItems(await editMission(projectId, editing.item, editing.text));
      setEditing(null);
      setError("");
    } catch (err) {
      setError(`修改任务失败：${err instanceof Error ? err.message : String(err)}`);
    } finally { setSaving(false); }
  }
  const load = useCallback(async () => {
    try {
      setItems(await listMission(projectId, tool.id));
      setError("");
    } catch (err) {
      setError(`读取观察清单失败：${err instanceof Error ? err.message : String(err)}`);
    }
  }, [projectId, tool.id]);
  useEffect(() => {
    if (!busy) void load();
  }, [load, busy]);

  async function toggle(m: MissionItem) {
    if (saving || busy) return;
    setSaving(true);
    try {
      const got = await tickMission(projectId, m.id, !m.doneAt);
      setItems((prev) => prev.map((x) => (x.id === got.id ? got : x)));
      setError("");
    } catch (err) {
      setError(`记录任务状态失败：${err instanceof Error ? err.message : String(err)}`);
    } finally {
      setSaving(false);
    }
  }

  const activeItems = items.filter((m) => !m.supersededAt);
  const history = items.filter((m) => m.supersededAt);
  const done = activeItems.filter((m) => m.doneAt).length;

  return (
    <div className="rounded-mk-md border border-mk-border bg-mk-surface px-3 py-2.5">
      <div className="flex items-start gap-2">
        <Icon icon={Footprints} size={15} className="mt-0.5 shrink-0 text-mk-accent-500" />
        <div className="min-w-0">
          <p className="text-mk-small font-semibold text-mk-ink">{tool.label}</p>
          <p className="mt-1 text-mk-small text-mk-muted">{tool.reason}</p>
          {/* 🚨 这一句原来只印在邀请卡上，她一答应就没了——而这张卡正是她几天
              后回来时唯一还能看到的东西。要她做的事必须一直留在上面。 */}
          <p className="mt-1 text-mk-small text-mk-faint">
            请在合适的地方完成这项任务，完成后回来记录结果
          </p>

          {/* 出门清单。在现场点掉一条就够——不用打字。 */}
          {items.length > 0 && (
            <div className="mt-2">
              <p className="text-mk-small text-mk-secondary">
                观察清单（已完成 {done}/{activeItems.length}）
              </p>
              <div className="mt-1 space-y-1">
                {activeItems.map((m) => {
                  const meta = m.wantKind ? noteKindMeta(m.wantKind) : null;
                  const on = !!m.doneAt;
                  return (
                    <div key={m.id} className="rounded-mk-sm border border-mk-border">
                    <button
                      type="button"
                      disabled={busy || saving || editing !== null}
                      aria-pressed={on}
                      onClick={() => void toggle(m)}
                      className="flex w-full items-start gap-2 rounded-mk-sm px-2 py-1.5 text-left"
                      style={{
                        background: on ? "var(--mk-success-bg)" : "var(--mk-paper)",
                      }}
                    >
                      <span
                        className="mt-0.5 flex h-4 w-4 shrink-0 items-center justify-center rounded-mk-full text-[10px]"
                        style={{
                          background: on ? "var(--mk-success)" : "transparent",
                          border: on ? "none" : "1.5px solid var(--mk-border)",
                          color: "var(--mk-surface)",
                        }}
                      >
                        {on ? "✓" : ""}
                      </span>
                      <span className="min-w-0 flex-1 text-mk-small text-mk-ink">
                        {m.prompt}
                        {meta && (
                          <span
                            className="ml-1.5 rounded-mk-full px-1.5 text-mk-small"
                            style={{
                              background: `color-mix(in srgb, ${meta.hue} 18%, transparent)`,
                              color: "var(--mk-secondary)",
                            }}
                          >
                            {meta.label}
                          </span>
                        )}
                      </span>
                    </button>
                    <div className="flex items-center justify-between px-2 pb-1.5 text-mk-small text-mk-muted">
                      <span>{m.editedByStudent ? "学生修改" : ""}</span>
                      <button type="button" disabled={busy || saving || editing !== null} onClick={() => {setEditing({item:m, text:m.prompt}); setError("");}} className="rounded-mk-sm px-2 py-1 text-mk-secondary disabled:opacity-40" aria-label={`修改任务 ${m.ordinal + 1}`}>修改任务</button>
                    </div>
                    </div>
                  );
                })}
              </div>
            </div>
          )}
        </div>
      </div>
      {editing && <section className="mt-3 rounded-mk-md border border-mk-border p-3">
        <h3 className="text-mk-small font-semibold">修改任务 {editing.item.ordinal + 1}</h3>
        <p className="mt-1 text-mk-small text-mk-muted">保存后保留原任务，新任务需要重新标记完成。</p>
        <GrowingTextarea aria-label="任务内容" rows={3} maxLength={2000} value={editing.text} disabled={saving || busy} onChange={e => setEditing({...editing, text:e.target.value})} className="mt-2 w-full rounded-mk-sm border border-mk-input-border bg-mk-surface p-2 text-mk-small" />
        <div className="mt-2 flex gap-2">
          <button type="button" disabled={saving || busy || !editing.text.trim()} onClick={() => void saveEdit()} className="rounded-mk-md border border-mk-border px-3 py-1.5 text-mk-small disabled:opacity-40">{saving ? "保存中" : "保存修改"}</button>
          <button type="button" disabled={saving} onClick={() => {setEditing(null); setError("");}} className="rounded-mk-md px-3 py-1.5 text-mk-small">取消</button>
        </div>
      </section>}
      {history.length > 0 && (
        <details className="mt-3 border-t border-mk-border pt-2 text-mk-small text-mk-muted">
          <summary className="cursor-pointer">历史任务（{history.length} 项）</summary>
          <p className="mt-2">以下任务已修订，不计入当前进度。</p>
          {history.map((m) => <p key={m.id} className="mt-2">{m.doneAt ? "已勾选" : "未勾选"}{m.editedByStudent ? " · 学生修改" : ""} · {m.prompt}</p>)}
        </details>
      )}
      {error && <div role="alert" className="mt-2"><Says content={errorMarkdown(error)} /><button type="button" disabled={busy || saving} onClick={() => void load()} className="mt-2 rounded-mk-md border border-mk-border px-3 py-1.5 text-mk-small">刷新清单</button></div>}
      <button
        type="button"
        disabled={busy || saving || editing !== null}
        onClick={onBack}
        className="mt-2.5 w-full rounded-mk-full border border-mk-border py-1.5 text-mk-small text-mk-secondary disabled:opacity-40"
      >
        {/* 🚨「已完成」是个状态词，印在一个还没做完的任务上，读起来像是这件事
            已经结束了。按钮要写她按下去会发生什么。 */}
        记录结果
      </button>
    </div>
  );
}

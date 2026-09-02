import { apiErrorText } from "../../../api/errorText";
import { useState } from "react";
import { Plus, X } from "lucide-react";
import { Icon } from "@/ui";
import { NOTE_KINDS, createNotes, listNotes, moveNote, type NoteKind } from "../../../api/notes";
import { ToolFrame } from "../ToolFrame";
import { boardSpot } from "../boardLayout";
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
}

export function Observe({ projectId, tool, onFinish, onClose }: ToolSurfaceProps) {
  const [items, setItems] = useState<Brought[]>([{ kind: "observation", body: "" }]);
  const [error, setError] = useState<string | null>(null);

  const filled = items.filter((i) => i.body.trim());

  function set(i: number, patch: Partial<Brought>) {
    setItems((prev) => prev.map((it, n) => (n === i ? { ...it, ...patch } : it)));
  }

  async function finish() {
    try {
      const made = await createNotes(
        projectId,
        filled.map((i) => ({ kind: i.kind, body: i.body.trim() })),
      );
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
      onFinish({ brought: filled.length }, "");
    } catch (err) {
      setError(apiErrorText(err));
    }
  }

  return (
    <ToolFrame
      title={tool.label}
      task="回来了。把你看到的、听到的，一条一条记下来"
      why={tool.reason}
      todo={filled.length === 0 ? "至少带回来一条" : ""}
      finishLabel="贴到板上"
      onFinish={() => void finish()}
      onClose={onClose}
    >
      {error && (
        <p className="mb-2 text-mk-small" style={{ color: "var(--mk-danger)" }}>
          {error}
        </p>
      )}

      <div className="space-y-2">
        {items.map((it, i) => (
          <div key={i} className="rounded-mk-md border border-mk-border p-2">
            <div className="flex items-start justify-between gap-2">
              <div className="flex flex-wrap gap-1">
                {NOTE_KINDS.filter((k) => k.kind !== "idea").map((k) => (
                  <button
                    key={k.kind}
                    type="button"
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
            <textarea
              value={it.body}
              onChange={(e) => set(i, { body: e.target.value })}
              rows={2}
              placeholder="具体一点：谁、在哪、做了什么"
              className="mt-1.5 w-full resize-none rounded-mk-sm border border-mk-input-border bg-mk-surface px-2 py-1.5 text-mk-small text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent-200"
            />
          </div>
        ))}
      </div>

      <button
        type="button"
        onClick={() => setItems((prev) => [...prev, { kind: "observation", body: "" }])}
        className="mt-2 flex w-full items-center justify-center gap-1 rounded-mk-md border border-dashed border-mk-border py-2 text-mk-small text-mk-secondary"
      >
        <Icon icon={Plus} size={14} />
        再加一条
      </button>

      {/* 占位：照片、录音、涂鸦。产品负责人提过「a photo, voice note, drawing」，
          图片要走 OSS（不进数据库），等 S4 那一刀一起做。 */}
    </ToolFrame>
  );
}

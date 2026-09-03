import { useCallback, useEffect, useState } from "react";
import { apiErrorText } from "../api/errorText";
import { listMaterials, type Material } from "../api/materials";
import { summonTool, type ToolInstance as Tool } from "../api/tools";
import type { PlanResolution, PlanState } from "../api/projectRoom";
import type { ToolInstance } from "../api/tools";
import { PlanPanel } from "./PlanPanel";
import { TOOL_TASKS, surfaceFor, type ToolSurfaceProps } from "./tools/registry";
import { ToolFrame } from "./tools/ToolFrame";
import { WidePaneProvider } from "./tools/wide";

/**
 * WorkPanel —— 右边这一栏。
 *
 * 左边是对话，右边是正在做的东西。计划一直在，工具打开时盖在它上面——盖住而
 * 不是并排，因为一次做一件事；标签页留在顶上，她随时看得见自己还能回到计划。
 *
 * 出门做的事单独一档，永远显示在计划那一页上：她需要看见"我还欠着一趟观察"，
 * 但那不该出现在待办清单里被催（铁律②）。
 */
export function WorkPanel({
  projectId,
  projectKind,
  wide,
  onToggleWide,
  plan,
  tools,
  openTool,
  onSelectTool,
  onFinishTool,
  onOpenSession,
  onResolve,
  onApprove,
  onToolsChanged,
  busy,
}: {
  projectId: string;
  projectKind: string;
  /** 工具是不是铺开占满了整个房间。 */
  wide: boolean;
  onToggleWide: () => void;
  plan: PlanState;
  tools: ToolInstance[];
  /** 当前打开的工具 id；null = 看计划。 */
  openTool: string | null;
  onSelectTool: (id: string | null) => void;
  onFinishTool: (tool: ToolInstance, result: unknown, summary: string) => void;
  /** 工具把她送进一条支线（服务端已经开好）。 */
  onOpenSession: (sessionId: string) => void;
  /** 材料清单自己开了一件工具，让房间把工具列表拉一遍。 */
  onToolsChanged: () => void;
  onResolve: (changeId: string, resolution: PlanResolution, reason: string) => Promise<void>;
  onApprove: (versionId: string) => Promise<void>;
  busy?: boolean;
}) {
  // 标签页只给当场做的工具。出门的那些不占标签——她人不在，一个空着的标签
  // 页只会像一件没做完的事。
  const openThinking = tools.filter((t) => t.kind === "thinking" && t.status === "accepted");
  // 🚨 打开的可以是任何一件已接受的工具，包括出门回来要汇报的那件——所以这里
  // 查的是全部 tools，不是 openThinking。少了这一句，「我回来了」按下去没反应。
  const active = tools.find((t) => t.id === openTool && t.status === "accepted") ?? null;

  return (
    <div className="flex h-full flex-col">
      {/* 🚨 工具打开时右栏整个变成那件工具，顶上不留标签页。
          产品负责人 2026-09-03，照 Claude Cowork 的分法：「when we review
          something, the right side becomes the preview things … and the task
          todo list disappears, close that then todo list appears again.」
          原来那排标签页既占地方，又让"我现在在哪儿"变成一个要她自己维护的
          状态；而且工具一多就横向滚动，当前那个还常常滚出视野。 */}
      <div className="min-h-0 flex-1">
        {active ? (
          <WidePaneProvider value={{ wide, toggle: onToggleWide }}>
            <ToolSurface
              projectId={projectId}
              projectKind={projectKind}
              tool={active}
              onFinish={(result, summary) => onFinishTool(active, result, summary)}
              onOpenSession={onOpenSession}
              onClose={() => onSelectTool(null)}
            />
          </WidePaneProvider>
        ) : (
          <div className="flex h-full flex-col overflow-y-auto">
            {/* 上半截：任务。出门在做的那件事不在这里单开一档（产品负责人
                2026-09-02：「we only show done, processing, plan three
                status」）。 */}
            <div className="shrink-0">
              <PlanPanel
                plan={plan.plan}
                pending={plan.pending}
                onResolve={onResolve}
                onApprove={onApprove}
              />
            </div>

            {/* 印记递来、她还没做的几件。原来它们只活在对话里那张卡片上，
                被后面的对话顶上去就等于消失了。 */}
            {openThinking.length > 0 && (
              <div className="shrink-0 border-t border-mk-border px-4 py-3">
                <p className="text-mk-small font-semibold text-mk-ink">进行中</p>
                <div className="mt-2 space-y-1.5">
                  {openThinking.map((t) => (
                    <button
                      key={t.id}
                      type="button"
                      onClick={() => onSelectTool(t.id)}
                      className="block w-full rounded-mk-md border border-mk-border px-3 py-2 text-left"
                      style={{ background: "var(--mk-accent-50)", borderColor: "transparent" }}
                    >
                      <span className="block text-mk-small font-medium text-mk-ink">{t.label}</span>
                      {t.reason && (
                        <span className="mt-0.5 block text-mk-small text-mk-muted">{t.reason}</span>
                      )}
                    </button>
                  ))}
                </div>
              </div>
            )}

            {/* 下半截：材料。她攒下来的东西，点一下回去看。 */}
            <MaterialsList
              projectId={projectId}
              tools={tools}
              onOpen={onSelectTool}
              onSummoned={onToolsChanged}
            />
          </div>
        )}
      </div>
    </div>
  );
}

/**
 * 材料清单 —— 这个项目攒下来的东西。
 *
 * 点一行就回到那件东西所在的工具里。她自己开的工具也要有理由（服务端拒绝
 * 没有理由的工具，这是对的：一件说不出为什么的工具对她是一次打断），所以
 * 这里如实写「你自己打开的」，不编一个像印记说的理由。
 */
function MaterialsList({
  projectId,
  tools,
  onOpen,
  onSummoned,
}: {
  projectId: string;
  tools: Tool[];
  onOpen: (id: string) => void;
  onSummoned: () => void;
}) {
  const [items, setItems] = useState<Material[]>([]);
  const [error, setError] = useState<string | null>(null);

  const reload = useCallback(async () => {
    try {
      setItems(await listMaterials(projectId));
    } catch (err) {
      setError(apiErrorText(err));
    }
  }, [projectId]);

  useEffect(() => {
    void reload();
    // tools 变了通常意味着她刚做完一件工具，材料多半也变了。
  }, [reload, tools]);

  async function open(m: Material) {
    // 已经开着的那件优先，不要给同一件工具再造一张卡。
    const live = tools.find((t) => t.tool === m.tool && t.status === "accepted");
    if (live) {
      onOpen(live.id);
      return;
    }
    const offered = tools.find((t) => t.tool === m.tool && t.status === "summoned");
    try {
      const got =
        offered ?? (await summonTool(projectId, { tool: m.tool, reason: "你自己打开的" }));
      onSummoned();
      onOpen(got.id);
    } catch (err) {
      setError(apiErrorText(err));
    }
  }

  return (
    <div className="border-t border-mk-border px-4 py-3">
      <p className="text-mk-small font-semibold text-mk-ink">材料</p>
      {error && (
        <p className="mt-1 text-mk-small" style={{ color: "var(--mk-danger)" }}>
          {error}
        </p>
      )}
      <div className="mt-2 space-y-1">
        {items.map((m) => {
          const empty = m.count === 0;
          return (
            <button
              key={m.tool}
              type="button"
              onClick={() => void open(m)}
              className="flex w-full items-center gap-2 rounded-mk-md px-2 py-1.5 text-left hover:bg-mk-paper"
            >
              {/* 🚨 一个小圆点，不是一条竖色带。六行各挂一条色带就是一排栅栏
                  ——产品负责人 2026-09-03 明确说了不要。 */}
              <span
                className="h-2 w-2 shrink-0 rounded-mk-full"
                style={{ background: empty ? "var(--mk-border)" : m.hue }}
              />
              <span className="min-w-0 flex-1">
                <span
                  className="block text-mk-small"
                  style={{ color: empty ? "var(--mk-faint)" : "var(--mk-ink)" }}
                >
                  {m.label}
                </span>
                {/* 有东西就说清是什么；还没有就照实说，不留一行没来由的标题。 */}
                <span className="block truncate text-mk-small text-mk-muted">
                  {m.detail || "还没有"}
                </span>
              </span>
            </button>
          );
        })}
      </div>
    </div>
  );
}

/** 有专门界面的用专门界面；没有的退回到一张朴素卡片，不白屏。 */
function ToolSurface(props: ToolSurfaceProps) {
  const Surface = surfaceFor(props.tool.tool);
  if (Surface) return <Surface {...props} />;
  return <PlainSurface {...props} />;
}

/**
 * 工具箱里没有的工具。
 *
 * 印记可以召出一件我们还没画界面的工具——名字是自由字符串就是为了这个。她
 * 在这里自己写做完的结果，一样进过程记录。
 */
function PlainSurface({ tool, onFinish, onClose }: ToolSurfaceProps) {
  const [text, setText] = useState("");
  return (
    <ToolFrame
      title={tool.label}
      task={TOOL_TASKS[tool.tool] ?? "把这件事做完，把结果写在下面"}
      why={tool.reason}
      todo={text.trim() ? "" : "还没写结果"}
      onFinish={() => onFinish({ text: text.trim() }, text.trim())}
      onClose={onClose}
    >
      <textarea
        value={text}
        onChange={(e) => setText(e.target.value)}
        rows={8}
        placeholder="写下你做出来的结果"
        className="w-full resize-none rounded-mk-md border border-mk-input-border bg-mk-surface px-3 py-2 text-mk-body text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent-200"
      />
    </ToolFrame>
  );
}

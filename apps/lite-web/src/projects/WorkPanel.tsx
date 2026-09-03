import { useState } from "react";
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
      {openThinking.length > 0 && (
        <nav className="flex shrink-0 gap-1 overflow-x-auto border-b border-mk-border px-3 py-2">
          <Tab label="计划" on={!active} onClick={() => onSelectTool(null)} />
          {openThinking.map((t) => (
            <Tab
              key={t.id}
              label={t.label}
              on={active?.id === t.id}
              onClick={() => onSelectTool(t.id)}
            />
          ))}
        </nav>
      )}

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
          <div className="flex h-full flex-col">
            {/* 出门在做的那件事不在这里单开一档（产品负责人 2026-09-02：
                「we only show done, processing, plan three status」）。它的
                入口在对话里那张邀请卡上，回来点「已完成」就行。 */}
            <div className="min-h-0 flex-1">
              <PlanPanel
                plan={plan.plan}
                pending={plan.pending}
                onResolve={onResolve}
                onApprove={onApprove}
              />
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

function Tab({ label, on, onClick }: { label: string; on: boolean; onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="shrink-0 whitespace-nowrap rounded-mk-full px-3 py-1 text-mk-small"
      style={
        on
          ? { background: "var(--mk-accent-500)", color: "#fff" }
          : { color: "var(--mk-secondary)" }
      }
    >
      {label}
    </button>
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

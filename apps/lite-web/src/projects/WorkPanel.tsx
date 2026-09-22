import { Says, errorMarkdown } from "./Says";
import { studentArtwork } from "../learning/StudentArtwork";
import { useCallback, useEffect, useState } from "react";
import { apiErrorText } from "../api/errorText";
import { listMaterials, type Material } from "../api/materials";
import { SELF_OPENED, acceptTool, summonTool, type ToolInstance as Tool } from "../api/tools";
import type { PlanResolution, PlanState, PlanStep } from "../api/projectRoom";
import type { ToolInstance } from "../api/tools";
import { PlanPanel } from "./PlanPanel";
import { TOOL_TASKS, surfaceFor, type ToolSurfaceProps } from "./tools/registry";
import { ToolFrame } from "./tools/ToolFrame";
import { WidePaneProvider } from "./tools/wide";
import { ArtifactLibrary } from "./ArtifactLibrary";

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
  boardAxes,
  onSetBoardAxes,
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
  onSubmit,
  onDiscuss,
  onOpenMaterial,
  busy,
}: {
  projectId: string;
  projectKind: string;
  boardAxes: boolean;
  onSetBoardAxes: (on: boolean) => Promise<void>;
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
  onOpenSession: (sessionId: string) => Promise<boolean>;
  /** 材料清单开了一件工具：房间把它放进列表并选中。 */
  onOpenMaterial: (tool: ToolInstance) => void;
  onResolve: (changeId: string, resolution: PlanResolution, reason: string) => Promise<void>;
  onApprove: (versionId: string) => Promise<void>;
  onSubmit: (stepId: string, note: string, url: string) => Promise<void>;
  onDiscuss: (step: PlanStep) => void;
  busy?: boolean;
}) {
  // 标签页只给当场做的工具。出门的那些不占标签——她人不在，一个空着的标签
  // 页只会像一件没做完的事。
  // 🚨 「进行中」只列印记递来的。她从材料清单点开的那些带着 SELF_OPENED 标记，
  // 不算待办——看一眼便签板不该在她的任务列表里留下一条。
  const openThinking = tools.filter(
    (t) => t.kind === "thinking" && t.status === "accepted" && t.reason !== SELF_OPENED,
  );
  // 🚨 打开的可以是任何一件已接受的工具，包括出门回来要汇报的那件——所以这里
  // 查的是全部 tools，不是 openThinking。少了这一句，「我回来了」按下去没反应。
  const active = tools.find((t) => t.id === openTool && t.status === "accepted") ?? null;
  const [showArtifacts, setShowArtifacts] = useState(false);
  useEffect(() => setShowArtifacts(false), [projectId, openTool]);

  async function openReview() {
    const existing = tools.find(t => t.tool === "review" && (t.status === "accepted" || t.status === "summoned"));
    const offered = existing ?? await summonTool(projectId, {tool: "review", reason: SELF_OPENED});
    const opened = offered.status === "accepted" ? offered : await acceptTool(projectId, offered.id);
    onOpenMaterial(opened);
    setShowArtifacts(false);
  }

  return (
    <div className="flex h-full flex-col">
      {/* 🚨 工具打开时右栏整个变成那件工具，顶上不留标签页。
          产品负责人 2026-09-03，照 Claude Cowork 的分法：「when we review
          something, the right side becomes the preview things … and the task
          todo list disappears, close that then todo list appears again.」
          原来那排标签页既占地方，又让"我现在在哪儿"变成一个要她自己维护的
          状态；而且工具一多就横向滚动，当前那个还常常滚出视野。 */}
      <div className="min-h-0 flex-1">
        {showArtifacts ? <ArtifactLibrary projectId={projectId} busy={busy} onClose={() => setShowArtifacts(false)} onReview={openReview} onOpenSession={id => { void onOpenSession(id).then(opened => { if (opened) setShowArtifacts(false); }); }} /> : active ? (
          <WidePaneProvider value={{ wide, toggle: onToggleWide }}>
            <ToolSurface
              projectId={projectId}
              projectKind={projectKind}
              boardAxes={boardAxes}
              onSetBoardAxes={onSetBoardAxes}
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
                onSubmit={onSubmit}
                onDiscuss={onDiscuss}
                busy={busy}
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
              projectKind={projectKind}
              tools={tools}
              onOpen={onOpenMaterial}
              onArtifacts={() => setShowArtifacts(true)}
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
 * 这里如实写「由你打开」，不编一个像印记说的理由。
 */
function MaterialsList({
  projectId,
  projectKind,
  tools,
  onOpen,
  onArtifacts,
}: {
  projectId: string;
  projectKind: string;
  tools: Tool[];
  /** 开这件工具：把它交给房间，房间负责放进列表并选中。 */
  onOpen: (tool: Tool) => void;
  onArtifacts: () => void;
}) {
  const [items, setItems] = useState<Material[]>([]);
  const [error, setError] = useState<string | null>(null);

  const reload = useCallback(async () => {
    try {
      setItems(await listMaterials(projectId, projectKind));
    } catch (err) {
      setError(apiErrorText(err));
    }
  }, [projectId, projectKind]);

  useEffect(() => {
    void reload();
    // tools 变了通常意味着她刚做完一件工具，材料多半也变了。
  }, [reload, tools]);

  async function open(m: Material) {
    // 🚨 空的那几行不开。点进去看到「暂时没有需要决策的内容」，和印记递一件
    // 点开是空的工具是同一种挫败——只不过这次是她自己撞上去的。
    if (m.count === 0 && !m.availableWhenEmpty) return;
    if (m.tool === "review") { onArtifacts(); return; }
    // 已经开着的那件优先，不要给同一件工具再造一张卡。
    const live = tools.find((t) => t.tool === m.tool && t.status === "accepted");
    if (live) {
      onOpen(live);
      return;
    }
    try {
      // 🚨 递出来的工具是 summoned，而右栏只认 accepted——所以光召不够，还要
      // 替她"打开"。第一版漏了这一步：点材料清单毫无反应，因为 active 永远
      // 是 null。2026-09-03 线上实测撞到的。
      const offered = tools.find((t) => t.tool === m.tool && t.status === "summoned");
      const summoned =
        offered ?? (await summonTool(projectId, { tool: m.tool, reason: SELF_OPENED }));
      const got = await acceptTool(projectId, summoned.id);
      // 🚨 把这件工具直接交给房间，而不是"通知它去重拉一遍"。重拉是异步的，
      // 而选中是同步的——先选中、后到货，右栏照样是空的。
      onOpen(got);
    } catch (err) {
      setError(apiErrorText(err));
    }
  }

  return (
    <div className="border-t border-mk-border px-4 py-3">
      <p className="text-mk-small font-semibold text-mk-ink">材料</p>
      {error && (
        <div className="mt-1 text-mk-small" style={{ color: "var(--mk-danger)" }}>
          <Says content={errorMarkdown(error)} />
        </div>
      )}
      {!error && items.length > 0 && items.every(m => m.count === 0 && !m.availableWhenEmpty) && (
        <div className="student-materials-empty">
          <img src={studentArtwork.project} alt="" />
          <p>暂无项目材料</p>
          <span>完成工具后，便签、方案与成果将显示在这里。</span>
        </div>
      )}
      <div className="mt-2 space-y-1">
        {items.filter(m => m.count > 0 || m.availableWhenEmpty).map((m) => {
          const empty = m.count === 0 && !m.availableWhenEmpty;
          return (
            <button
              key={m.tool}
              type="button"
              disabled={empty}
              onClick={() => void open(m)}
              className="flex w-full items-center gap-2 rounded-mk-md px-2 py-1.5 text-left enabled:hover:bg-mk-paper disabled:cursor-default"
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
                  {m.detail || "暂无"}
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
      task={TOOL_TASKS[tool.tool] ?? "请完成任务并记录结果"}
      why={tool.reason}
      todo={text.trim() ? "" : "请填写任务结果"}
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

import { useEffect, useState } from "react";
import { Button, Icon } from "@/ui";
import { ArrowLeft, ArrowRight, Check, ListOrdered } from "lucide-react";
import { Select } from "../teacher/controls/Select";
import "./writing-studio.css";
import { StructureDiagram } from "./StructureDiagram";
import { StageMap, type WritingStageKey } from "./StageMap";
import { MindMap } from "./MindMap";
import { moveOutlineNode, type OutlineMoveMode } from "./outlineMove";
import { outlineKindLabel, outlineKindOf } from "./outlineKind";
import { flowCanCarryMethod, flowLooksDone, type FlowStructure } from "./flowStructures";
import { handleWriteError } from "./writeErrors";
import {
  getWritingFlowStructures,
  putWritingFlow,
  putWritingOutline,
  type FlowMethodDTO,
  type WritingOutlineItem,
} from "../api/writingRoom";

/**
 * FlowStage —— 行文，第四步里排第二的那一步（结构 → **行文** → 段落 → 成稿）。
 *
 * # 它回答的那一问（同事 2026-09-20 的意见 4）
 *
 *	「前期逻辑讨论的部分需要增加一个对于行文方式的思考和梳理部分，
 *	  要在开始写之前先想好整个文章组织框架（**并非填充内容**）如何搭建，
 *	  现在只有文本内容的引导。」
 *
 * 结构那一步长出「有哪些点」，段落那一步直接开始写字，中间少了一问：
 * **这些点之间是什么关系、按什么顺序摆、每一条打算怎么证明。**
 * 她因此是一段一段攒出一篇文章，而不是先有一篇文章的形状再去填。
 *
 * # 这一步一个字的正文都不写
 *
 * 括号里那句话就是这块板的边界。板上只有三件事：
 *
 *   1. **顺序** —— 分论点可以拖着换先后（复用 outlineMove 的 "after"）。
 *   2. **整篇的论证结构** —— 四张卡选一张（总分式 / 并列式 / 层进式 / 对照式）。
 *   3. **每条分论点用哪个论证方法** —— 一个下拉。
 *
 * 🚨 这**不是** 2026-08-27 被否掉的那个「从库里挑一副骨架往里填」。
 * 那次否的是**在她想之前**给她一张表去挑；这一步发生在她自己的分论点都已经
 * 在图上之后，选的是她已经摆出来的那些点之间是什么关系 ——
 * 先有东西，再给它命名，和「方法名等她做出来再点」是同一条教学动作。
 *
 * 🚨 也不是关卡。顶上那条导航一直点得动，底栏那颗按钮只是一条明路。
 */
export function FlowStage({
  writingId,
  title,
  onJump,
  outline,
  structureKey,
  onOutline,
  onStructureKey,
  onDone,
  onBack,
  onLocked,
}: {
  writingId: string;
  title?: string;
  onJump?: (stage: WritingStageKey) => void;
  outline: WritingOutlineItem[];
  structureKey: string;
  onOutline: (next: WritingOutlineItem[]) => void;
  onStructureKey: (next: string) => void;
  onDone: () => void;
  onBack: () => void;
  onLocked: () => void;
}) {
  const [structures, setStructures] = useState<FlowStructure[]>([]);
  const [methods, setMethods] = useState<FlowMethodDTO[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    let alive = true;
    void getWritingFlowStructures(writingId)
      .then((r) => {
        if (!alive) return;
        setStructures(r.structures);
        setMethods(r.methods);
      })
      .catch(() => {
        // 读不到不该让这一步整个打不开：顺序仍然拖得动。
        if (!alive) return;
        setStructures([]);
        setMethods([]);
      });
    return () => {
      alive = false;
    };
  }, [writingId]);

  const ordered = outline.slice().sort((a, b) => a.position - b.position);
  const blocks = ordered.filter((o) => flowCanCarryMethod(outlineKindOf(o)));

  async function save(nextStructure: string, nextMethods: Record<string, string>) {
    setSaving(true);
    try {
      const next = await putWritingFlow(writingId, nextStructure, nextMethods);
      onOutline(next);
      onStructureKey(nextStructure);
      setError(null);
    } catch (err) {
      handleWriteError(err, onLocked, setError);
    } finally {
      setSaving(false);
    }
  }

  function pickStructure(id: string) {
    const next = id === structureKey ? "" : id; // 再点一下 = 取消
    void save(next, methodMapOf(ordered));
  }

  function setMethod(outlineId: string, methodId: string) {
    const map = methodMapOf(ordered);
    map[outlineId] = methodId;
    void save(structureKey, map);
  }

  /** 拖着换分论点的先后。算新清单的是纯函数，这里只负责落库。 */
  function moveNode(draggedId: string, targetId: string, mode: OutlineMoveMode) {
    const next = moveOutlineNode(outline, draggedId, targetId, mode);
    if (!next) return;
    void (async () => {
      try {
        onOutline(
          await putWritingOutline(
            writingId,
            next.map((n) => ({
              text: n.text,
              role: n.role,
              kind: outlineKindOf(n),
              method: n.method ?? "",
              depth: n.depth,
              source: n.source,
            })),
          ),
        );
      } catch (err) {
        handleWriteError(err, onLocked, setError);
      }
    })();
  }

  const methodOptions = [
    { value: "", label: "还没定" },
    ...methods.map((m) => ({ value: m.id, label: m.name })),
  ];

  return (
    <div className="writing-flow mk-scroll">
      <header className="writing-flow__header">
        <button
          type="button"
          onClick={onBack}
          aria-label="返回"
          className="text-mk-muted hover:text-mk-ink"
        >
          <Icon icon={ArrowLeft} size={18} />
        </button>
        <div className="flex flex-col">
          <span className="writing-eyebrow">写作工作台 · 行文</span>
          <h1>{title || "组织你的论证"}</h1>
          {/* 名词 + 一句说明它为什么值得做（文案规则 5：先说为什么，再请她做）。 */}
          <p className="text-mk-small text-mk-muted">
            请确定论点的顺序、关系和论证方法。这一步不写正文。
          </p>
        </div>
        {onJump && <StageMap stage="flow" onJump={onJump} />}
      </header>

      {error && (
        <p className="rounded-mk-md px-3 py-2 text-mk-small" style={{ background: "var(--mk-danger-bg)", color: "var(--mk-danger)" }}>
          {error}
        </p>
      )}

      <div className="writing-flow__layout">
      <section className="writing-flow__map">
        <div className="writing-section-title"><span>01</span><h2>论点顺序</h2><Icon icon={ListOrdered} size={18} /></div>
        <p className="text-mk-small text-mk-muted">请拖动节点调整顺序；落点提示会显示移动结果。</p>
        <div className="writing-flow__canvas" style={{ borderColor: "var(--mk-border)" }}>
          <MindMap items={outline} justAdded={[]} onMove={moveNode} />
        </div>
      </section>

      <div className="writing-flow__decisions">
      {structures.length > 0 && (
        <section className="flex flex-col gap-2">
          <div className="writing-section-title"><span>02</span><h2>论证结构</h2></div>
          <p className="text-mk-small text-mk-muted">请选择论点之间的关系，再次点击可取消。</p>
          <div className="writing-structures">
            {structures.map((s) => {
              const on = s.id === structureKey;
              return (
                <button
                  key={s.id}
                  type="button"
                  onClick={() => pickStructure(s.id)}
                  disabled={saving}
                  aria-pressed={on}
                  className="writing-structure"
                  style={{
                    borderColor: on ? "var(--mk-accent-500)" : "var(--mk-border)",
                    background: on ? "var(--mk-accent-50)" : "var(--mk-surface)",
                  }}
                >
                  <StructureDiagram kind={s.id} />
                  <span className="writing-structure__title">{s.name}{on && <Icon icon={Check} size={16} />}</span>
                  <span className="text-mk-small text-mk-muted">{s.definition}</span>
                  {/* 借来的示范：光给定义，「层进式」和「并列式」在一个中学生
                      眼里是同一句话。它讲的是别的题目，不是她的。 */}
                  {s.example && (
                    <span className="writing-structure__example" style={{ borderColor: "var(--mk-border)" }}>
                      比如：{s.example}
                    </span>
                  )}
                </button>
              );
            })}
          </div>
        </section>
      )}

      {blocks.length > 0 && (
        <section className="flex flex-col gap-2">
          <div className="writing-section-title"><span>03</span><h2>论证方法</h2></div>
          <p className="text-mk-small text-mk-muted">为分论点选择论证方法，也可以在写作时补充。</p>
          <ul className="flex list-none flex-col gap-2">
            {blocks.map((o, i) => (
              <li
                key={o.id}
                className="writing-method"
                style={{ borderColor: "var(--mk-border)", background: "var(--mk-surface)" }}
              >
                <span className="writing-method__number">{String(i + 1).padStart(2, "0")}</span>
                <span className="flex min-w-0 flex-1 flex-col">
                  <span className="text-mk-label text-mk-accent-700">{outlineKindLabel(outlineKindOf(o))}</span>
                  <span className="text-mk-body text-mk-ink">{o.text}</span>
                </span>
                <Select
                  value={o.method ?? ""}
                  options={methodOptions}
                  onChange={(v) => setMethod(o.id, v)}
                  ariaLabel={`「${o.text}」用哪个论证方法`}
                  size="sm"
                  disabled={saving}
                />
              </li>
            ))}
          </ul>
        </section>
      )}

      </div>
      </div>
      <footer className="writing-flow__footer">
        <span className="text-mk-small text-mk-muted">
          {flowLooksDone(structureKey) ? "组织方式已确定。" : "尚未选择论证结构，可继续写作或稍后补充。"}
        </span>
        <Button onClick={onDone} iconEnd={<Icon icon={ArrowRight} size={16} />}>开始写段落</Button>
      </footer>
    </div>
  );
}

/** 把当前这份提纲上已经标好的方法收成一张表。 */
function methodMapOf(items: WritingOutlineItem[]): Record<string, string> {
  const out: Record<string, string> = {};
  for (const o of items) out[o.id] = o.method ?? "";
  return out;
}

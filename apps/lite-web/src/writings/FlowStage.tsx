import { useEffect, useState } from "react";
import { Button, Icon } from "@/ui";
import { ArrowLeft, ArrowRight, Check } from "lucide-react";
import "./writing-studio.css";
import { StageMap, type WritingStageKey } from "./StageMap";
import { StructureDiagram } from "./StructureDiagram";
import { FlowOrderList } from "./FlowOrderList";
import { PromptSidebar } from "./PromptSidebar";
import { moveOutlineNode, type OutlineMoveMode } from "./outlineMove";
import { outlineKindOf } from "./outlineKind";
import { flowLooksDone, type FlowStructure } from "./flowStructures";
import { handleWriteError } from "./writeErrors";
import type { Writing } from "../api/writings";
import {
  getWritingFlowStructures,
  putWritingFlow,
  putWritingOutline,
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
 * **这些点之间是什么关系、按什么顺序摆。**
 * 她因此是一段一段攒出一篇文章，而不是先有一篇文章的形状再去填。
 *
 * # 这一步一个字的正文都不写
 *
 * 括号里那句话就是这块板的边界。板上只有两件事：
 *
 *   1. **顺序** —— 正文那几块排成文章的顺序（FlowOrderList）。
 *   2. **整篇的论证结构** —— 几张卡选一张（总分式 / 并列式 / 层进式 / 对照式）。
 *
 * # 🚨 2026-09-22 拿掉了第三件事
 *
 * 原来还有一块「每一条打算怎么证明」：每条分论点一个下拉，从十一个论证方法里
 * 挑一个。两个理由拿掉它：
 *
 *   - 它**喂给了空气**。`writing_outline.method` 存下来之后没有任何一条提示词
 *     读它，全仓 grep 只有存和取两处。她挑那一下什么都不影响。
 *   - 产品负责人 2026-09-22：「this is awkward this selection... how would you
 *     think that students can use this to write? they would hate this.」
 *
 * 论证方法该由印记在她写那一段的时候顺着她手上的材料说一个（段落引导那条路
 * 本来就在做这件事），不该做成一张进门就要填的表。
 *
 * 🚨 也不是关卡。顶上那条导航一直点得动，底栏那颗按钮只是一条明路。
 */
export function FlowStage({
  writing,
  onJump,
  writingId,
  outline,
  structureKey,
  onOutline,
  onStructureKey,
  onDone,
  onBack,
  onLocked,
}: {
  /** 题目那一栏要它 —— 她排顺序的时候也得看得见题目。 */
  writing: Writing;
  onJump?: (stage: WritingStageKey) => void;
  writingId: string;
  outline: WritingOutlineItem[];
  structureKey: string;
  onOutline: (next: WritingOutlineItem[]) => void;
  onStructureKey: (next: string) => void;
  onDone: () => void;
  onBack: () => void;
  onLocked: () => void;
}) {
  const [structures, setStructures] = useState<FlowStructure[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    let alive = true;
    void getWritingFlowStructures(writingId)
      .then((r) => {
        if (!alive) return;
        setStructures(r.structures);
      })
      .catch(() => {
        // 读不到不该让这一步整个打不开：顺序仍然拖得动。
        if (!alive) return;
        setStructures([]);
      });
    return () => {
      alive = false;
    };
  }, [writingId]);

  function pickStructure(id: string) {
    const next = id === structureKey ? "" : id; // 再点一下 = 取消
    setSaving(true);
    void (async () => {
      try {
        onOutline(await putWritingFlow(writingId, next, {}));
        onStructureKey(next);
        setError(null);
      } catch (err) {
        handleWriteError(err, onLocked, setError);
      } finally {
        setSaving(false);
      }
    })();
  }

  /** 拖着换正文那几块的先后。算新清单的是纯函数，这里只负责落库。 */
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

  return (
    <div className="flex h-full">
      {/* 题目那一栏。行文这一步也在题目底下做 —— 同事 2026-09-22 的意见 1。 */}
      <PromptSidebar writing={writing} />
      <div className="writing-flow mk-scroll min-w-0 flex-1">
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
            <h1 className="text-mk-title font-semibold text-mk-ink">行文</h1>
            {/* 名词 + 一句说明它为什么值得做（文案规则 5：先说为什么，再请她做）。 */}
            <p className="text-mk-small text-mk-muted">
              请安排段落顺序，并确定各部分之间的关系。
            </p>
          </div>
          {onJump && <StageMap stage="flow" onJump={onJump} />}
        </header>

        {error && (
          <p
            className="rounded-mk-md px-3 py-2 text-mk-small"
            style={{ background: "var(--mk-danger-bg)", color: "var(--mk-danger)" }}
          >
            {error}
          </p>
        )}

        <div className="writing-flow__layout"><section className="writing-flow__map">
          <div className="writing-section-title"><span>01</span><h2>段落顺序</h2></div>
          <p className="text-mk-small text-mk-muted">
            列表从上到下对应文章的顺序。请拖动条目或使用右侧按钮调整顺序，下属材料会随之移动。
          </p>
          <FlowOrderList items={outline} onMove={moveNode} lang={writing.lang} />
        </section>

        <div className="writing-flow__decisions">{structures.length > 0 && (
          <section className="flex flex-col gap-2">
            <div className="writing-section-title"><span>02</span><h2>论证结构</h2></div>
            <p className="text-mk-small text-mk-muted">
              请选择分论点之间的关系，再次点击可取消选择。
            </p>
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
                    <StructureDiagram kind={s.id} /><span className="text-mk-body font-semibold text-mk-ink">{s.name}</span>{on && <Icon icon={Check} size={16} />}
                    <span className="text-mk-small text-mk-muted">{s.definition}</span>
                    {/* 借来的示范：光给定义，「层进式」和「并列式」在一个中学生
                        眼里是同一句话。它讲的是别的题目，不是她的。 */}
                    {s.example && (
                      <span
                        className="writing-structure__example"
                        style={{ borderColor: "var(--mk-border)" }}
                      >
                        比如：{s.example}
                      </span>
                    )}
                  </button>
                );
              })}
            </div>
          </section>
        )}</div></div>

        <div
          className="writing-flow__footer"
          style={{ borderColor: "var(--mk-border)" }}
        >
          <span className="text-mk-small text-mk-muted">
            {flowLooksDone(structureKey) ? "组织方式已确定。" : "尚未选择论证结构，可继续写作或稍后补充。"}
          </span>
          <Button onClick={onDone} iconEnd={<Icon icon={ArrowRight} size={16} />}>开始写段落</Button>
        </div>
      </div>
    </div>
  );
}

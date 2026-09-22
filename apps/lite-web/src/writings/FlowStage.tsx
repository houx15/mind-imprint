import { useEffect, useState } from "react";
import { Button, Icon } from "@/ui";
import { ArrowLeft } from "lucide-react";
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
      <div className="mk-scroll flex h-full min-w-0 flex-1 flex-col gap-5 overflow-y-auto p-6">
        <div className="flex items-center gap-2">
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
              写之前先定这篇怎么组织：几条理由按什么顺序摆、它们之间是什么关系。这一步不写正文。
            </p>
          </div>
        </div>

        {error && (
          <p
            className="rounded-mk-md px-3 py-2 text-mk-small"
            style={{ background: "var(--mk-danger-bg)", color: "var(--mk-danger)" }}
          >
            {error}
          </p>
        )}

        <section className="flex flex-col gap-2">
          <h2 className="text-mk-label font-semibold text-mk-accent-700">顺序</h2>
          <p className="text-mk-small text-mk-muted">
            从上到下就是文章的顺序。拖一条到别处，或者用右边的上下按钮。挂在一条底下的材料跟着它一起走。
          </p>
          <FlowOrderList items={outline} onMove={moveNode} />
        </section>

        {structures.length > 0 && (
          <section className="flex flex-col gap-2">
            <h2 className="text-mk-label font-semibold text-mk-accent-700">论证结构</h2>
            <p className="text-mk-small text-mk-muted">
              这几条理由之间是什么关系。选一个；再点一下取消。
            </p>
            <div className="grid gap-2 sm:grid-cols-2">
              {structures.map((s) => {
                const on = s.id === structureKey;
                return (
                  <button
                    key={s.id}
                    type="button"
                    onClick={() => pickStructure(s.id)}
                    disabled={saving}
                    aria-pressed={on}
                    className="flex flex-col gap-1 rounded-mk-md border p-3 text-left transition-colors duration-[120ms] ease-mk"
                    style={{
                      borderColor: on ? "var(--mk-accent-500)" : "var(--mk-border)",
                      background: on ? "var(--mk-accent-50)" : "var(--mk-surface)",
                    }}
                  >
                    <span className="text-mk-body font-semibold text-mk-ink">{s.name}</span>
                    <span className="text-mk-small text-mk-muted">{s.definition}</span>
                    {/* 借来的示范：光给定义，「层进式」和「并列式」在一个中学生
                        眼里是同一句话。它讲的是别的题目，不是她的。 */}
                    {s.example && (
                      <span
                        className="mt-1 border-l pl-2 text-mk-small text-mk-faint"
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
        )}

        <div
          className="flex items-center justify-between gap-3 border-t pt-4"
          style={{ borderColor: "var(--mk-border)" }}
        >
          <span className="text-mk-small text-mk-muted">
            {flowLooksDone(structureKey) ? "组织方式已确定。" : "选一个论证结构，这一步就算想清楚了。"}
          </span>
          <Button onClick={onDone}>完成，去写段落</Button>
        </div>
      </div>
    </div>
  );
}

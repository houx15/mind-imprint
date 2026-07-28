import { useState } from "react";
import { Icon, BLOCK_META } from "./Icon";
import { PlanBlock } from "./blocks/PlanBlock";
import { ReadingBlock } from "./blocks/ReadingBlock";
import { WritingBlock } from "./blocks/WritingBlock";
import { ReviewBlock } from "./blocks/ReviewBlock";
import { project, type BlockKey, type PlanItem } from "./protoData";

// Design-only prototype of the redesigned studio: a project is a workspace of
// four rooms (Plan · Reading · Writing · Reflection) the student moves between
// freely. Mounted via ?proto, fully self-contained mock state — touches none of
// the shipped StudioContainer / API. See docs discussion for the design intent.
export function StudioPrototype() {
  const [block, setBlock] = useState<BlockKey>("plan");
  // Prototype-only switch: view the seeded example vs a brand-new empty project.
  const [fresh, setFresh] = useState(false);

  // Clicking a plan card jumps to the matching room (doorway model).
  function openItem(item: PlanItem) {
    if (item.tag === "read") setBlock("reading");
    else if (item.tag === "write") setBlock("writing");
    else setBlock("reflection");
  }

  return (
    <div className="flex h-full w-full bg-mk-bg font-sans text-mk-ink">
      <Rail block={block} setBlock={setBlock} fresh={fresh} setFresh={setFresh} />
      <main className="min-w-0 flex-1 overflow-hidden">
        {/* key forces a fresh remount when flipping example ⇄ new project */}
        {block === "plan" && <PlanBlock key={fresh ? "new" : "seed"} fresh={fresh} onOpenItem={openItem} />}
        {block === "reading" && <ReadingBlock key={fresh ? "new" : "seed"} fresh={fresh} />}
        {block === "writing" && <WritingBlock />}
        {block === "reflection" && <ReviewBlock />}
      </main>
    </div>
  );
}

function Rail({ block, setBlock, fresh, setFresh }: { block: BlockKey; setBlock: (b: BlockKey) => void; fresh: boolean; setFresh: (f: boolean) => void }) {
  return (
    <nav className="flex w-[236px] flex-none flex-col border-r border-mk-border bg-mk-surface">
      <div className="border-b border-mk-border px-5 py-5">
        <button type="button" className="mb-3 flex items-center gap-1 text-[13px] font-semibold text-mk-muted-2 hover:text-mk-primary">
          <Icon name="back" size={16} /> 全部项目
        </button>
        <h1 className="font-sans text-[15.5px] font-bold leading-snug text-mk-ink">{fresh ? "未命名项目" : project.title}</h1>
        <span className="mt-2 inline-block rounded-full bg-mk-primary-tint px-2.5 py-1 text-[11px] font-bold text-mk-primary">{fresh ? "刚创建" : project.qualLabel}</span>
      </div>

      <div className="flex flex-1 flex-col gap-1 px-3 py-4">
        {BLOCK_META.map((b, i) => {
          const active = block === b.key;
          return (
            <button
              key={b.key}
              type="button"
              onClick={() => setBlock(b.key)}
              className={`group relative flex items-center gap-3 rounded-mk px-3 py-2.5 text-left transition ${
                active ? "bg-mk-primary-tint" : "hover:bg-mk-bg"
              }`}
            >
              {active && <span className="absolute left-0 top-1/2 h-6 w-1 -translate-y-1/2 rounded-r bg-mk-primary" />}
              <span className={active ? "text-mk-primary" : "text-mk-muted-2 group-hover:text-mk-muted"}>
                <Icon name={b.key} size={19} />
              </span>
              <span className="flex flex-col">
                <span className={`text-[14px] font-bold ${active ? "text-mk-primary" : "text-mk-ink"}`}>{b.label}</span>
                <span className="text-[11px] font-medium tracking-wide text-mk-muted-2">
                  {String(i + 1).padStart(2, "0")} · {b.sub}
                </span>
              </span>
            </button>
          );
        })}
      </div>

      <div className="border-t border-mk-border px-4 py-3">
        <span className="mb-2 block text-[10px] font-bold uppercase tracking-wider text-mk-muted-2">原型视角</span>
        <div className="flex rounded-mk border border-mk-border bg-mk-bg p-0.5">
          <button type="button" onClick={() => setFresh(false)} className={`flex-1 rounded-[10px] px-2 py-1.5 text-[12px] font-bold transition ${!fresh ? "bg-mk-surface text-mk-primary shadow-sm" : "text-mk-muted-2"}`}>
            示例项目
          </button>
          <button type="button" onClick={() => setFresh(true)} className={`flex-1 rounded-[10px] px-2 py-1.5 text-[12px] font-bold transition ${fresh ? "bg-mk-surface text-mk-primary shadow-sm" : "text-mk-muted-2"}`}>
            全新项目
          </button>
        </div>
      </div>
    </nav>
  );
}

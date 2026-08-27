import { useEffect, useState } from "react";
import { Sparkles, RefreshCw, HelpCircle } from "lucide-react";
import { Button, Icon } from "@/ui";
import { ApiError } from "../api/client";
import {
  listWritingStructures,
  recommendWritingStructure,
  applyWritingStructure,
  putWritingOutline,
  guideWritingBlock,
  type WritingOutlineItem,
  type WritingStructure,
  type WritingBlockGuide,
} from "../api/writingRoom";
import { GuideBox } from "./GuideBox";

/**
 * StructureStage — 结构. This file replaces OutlineStage, and the difference
 * between them is the 2026-08-27 ruling in one screen.
 *
 * OutlineStage had a 「帮我拟一份候选」 button that sent her whole 构思
 * transcript to the model and got a finished outline back. She then "edited"
 * something already written for her. That is the AI doing the thinking.
 *
 * Here the model can only ever point at one row of a fixed, generic library
 * (印记的建议). The skeleton it points at contains block LABELS and nothing
 * else — 「你的立场」「反方最强的说法」— identical for every student in the
 * school. Every sentence under those labels is typed by her, and the field
 * she types into starts empty, always.
 *
 * She can also ignore the suggestion entirely and pick from the shelf below
 * it, which is why the library is rendered whether or not a recommendation
 * ever arrives (including when the model call fails).
 */

type Props = {
  writingId: string;
  lang: string;
  structureKey: string;
  outline: WritingOutlineItem[];
  onOutlineChange: (next: WritingOutlineItem[]) => void;
  onStructureChange: (key: string) => void;
};

export function StructureStage({
  writingId,
  lang,
  structureKey,
  outline,
  onOutlineChange,
  onStructureChange,
}: Props) {
  const [library, setLibrary] = useState<WritingStructure[] | null>(null);
  const [recommendation, setRecommendation] = useState<{ structureKey: string; reason: string } | null>(null);
  const [recommending, setRecommending] = useState(false);
  const [applying, setApplying] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  // The 409 the server raises rather than silently discarding written blocks.
  // Holding the pending key here turns that refusal into a question she
  // answers, instead of an error she has to decode.
  const [confirmSwap, setConfirmSwap] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    listWritingStructures(lang)
      .then((rows) => {
        if (!cancelled) setLibrary(rows);
      })
      .catch(() => {
        if (!cancelled) setLibrary([]);
      });
    return () => {
      cancelled = true;
    };
  }, [lang]);

  async function askForRecommendation() {
    setRecommending(true);
    setError(null);
    try {
      setRecommendation(await recommendWritingStructure(writingId));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "这次没能给出建议，你可以直接自己挑一个。");
    } finally {
      setRecommending(false);
    }
  }

  async function apply(key: string, force = false) {
    setApplying(key);
    setError(null);
    try {
      const res = await applyWritingStructure(writingId, key, { force });
      onOutlineChange(res.outline);
      onStructureChange(res.structureKey);
      setConfirmSwap(null);
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        setConfirmSwap(key);
      } else {
        setError(err instanceof ApiError ? err.message : "换结构失败，请重试。");
      }
    } finally {
      setApplying(null);
    }
  }

  const chosen = library?.find((s) => s.key === structureKey) ?? null;

  return (
    <div className="flex flex-col gap-5">
      <div className="flex flex-col gap-1.5">
        <h2 className="text-mk-h2 text-mk-ink">结构</h2>
        <p className="text-mk-body text-mk-muted">
          先挑一副骨架，再一块一块填上你自己的想法。骨架只说每一块的作用，写什么由你。
        </p>
      </div>

      {!structureKey && (
        <RecommendationPanel
          recommendation={recommendation}
          recommending={recommending}
          library={library}
          onAsk={() => void askForRecommendation()}
          onAccept={(key) => void apply(key)}
          applying={applying}
        />
      )}

      {/*
        Gated on the OUTLINE, never on the library lookup. `chosen` can be null
        for reasons that have nothing to do with her — the library fetch failed,
        or she is on a skeleton that was later retired — and if this were gated
        on `chosen`, every block she has written would silently vanish from the
        page while still sitting in the database. Her rows carry their own
        `role`, so they render perfectly well on their own; the skeleton only
        contributes a name and the per-block hints, and both degrade to
        nothing.
      */}
      {outline.length > 0 && (
        <ChosenBlocks
          writingId={writingId}
          structureName={chosen?.name ?? ""}
          hints={chosen?.blocks.map((b) => b.hint) ?? []}
          outline={outline}
          onOutlineChange={onOutlineChange}
        />
      )}

      {error && <p className="text-mk-small text-mk-danger">{error}</p>}

      {confirmSwap && (
        <div className="rounded-mk-md border border-mk-border bg-mk-paper p-4">
          <p className="text-mk-body text-mk-ink">换一副结构会清掉你已经在这些块里写下的内容。确认要换吗？</p>
          <div className="mt-3 flex gap-2">
            <Button size="sm" onClick={() => void apply(confirmSwap, true)}>
              确认换
            </Button>
            <Button size="sm" variant="ghost" onClick={() => setConfirmSwap(null)}>
              不换
            </Button>
          </div>
        </div>
      )}

      <StructureShelf
        library={library}
        currentKey={structureKey}
        applying={applying}
        onPick={(key) => void apply(key)}
      />
    </div>
  );
}

// ---------------------------------------------------------------------------

function RecommendationPanel({
  recommendation,
  recommending,
  library,
  onAsk,
  onAccept,
  applying,
}: {
  recommendation: { structureKey: string; reason: string } | null;
  recommending: boolean;
  library: WritingStructure[] | null;
  onAsk: () => void;
  onAccept: (key: string) => void;
  applying: string | null;
}) {
  if (!recommendation) {
    return (
      <div className="flex flex-col items-start gap-2 rounded-mk-md border border-mk-border bg-mk-paper p-4">
        <p className="text-mk-body text-mk-muted">不知道挑哪一副？印记只帮你选，不替你写。</p>
        <Button
          variant="secondary"
          size="sm"
          onClick={onAsk}
          loading={recommending}
          iconStart={<Icon icon={Sparkles} size={14} />}
        >
          帮我挑一副
        </Button>
      </div>
    );
  }

  const rec = library?.find((s) => s.key === recommendation.structureKey);
  if (!rec) return null;

  return (
    <div
      className="flex flex-col gap-3 rounded-mk-md border p-4"
      style={{ borderColor: "var(--mk-accent-300)", background: "color-mix(in srgb, var(--mk-accent-500) 5%, var(--mk-paper))" }}
    >
      <div className="flex items-center gap-1.5">
        <span
          className="rounded-mk-full px-2 py-0.5 text-mk-label font-semibold"
          style={{ background: "var(--mk-accent-100)", color: "var(--mk-accent-700)" }}
        >
          印记的建议
        </span>
        <span className="text-mk-small text-mk-muted">下面也都能挑</span>
      </div>
      <div>
        <p className="text-mk-h3 text-mk-ink">{rec.name}</p>
        <p className="mt-1 text-mk-small text-mk-muted">{recommendation.reason}</p>
      </div>
      <BlockChips blocks={rec.blocks.map((b) => b.role)} />
      <div>
        <Button size="sm" onClick={() => onAccept(rec.key)} loading={applying === rec.key}>
          就用这一副
        </Button>
      </div>
    </div>
  );
}

/** The skeleton's block labels, rendered as a chain. Labels only — this is
 *  the shape of the essay, never a word of its content. */
function BlockChips({ blocks }: { blocks: string[] }) {
  return (
    <div className="flex flex-wrap items-center gap-1">
      {blocks.map((role, i) => (
        <span key={`${role}-${i}`} className="flex items-center gap-1">
          <span className="rounded-mk-xs bg-mk-surface px-2 py-0.5 text-mk-label text-mk-secondary shadow-mk-xs">
            {role}
          </span>
          {i < blocks.length - 1 && (
            <span aria-hidden="true" className="text-mk-label text-mk-faint">
              →
            </span>
          )}
        </span>
      ))}
    </div>
  );
}

function StructureShelf({
  library,
  currentKey,
  applying,
  onPick,
}: {
  library: WritingStructure[] | null;
  currentKey: string;
  applying: string | null;
  onPick: (key: string) => void;
}) {
  if (library === null) return <p className="text-mk-small text-mk-faint">正在取结构库…</p>;
  if (library.length === 0) return null;

  return (
    <section className="flex flex-col gap-2">
      <span className="text-mk-label text-mk-faint">{currentKey ? "换一副" : "全部结构"}</span>
      <div className="grid grid-cols-1 gap-2 lg:grid-cols-2">
        {library.map((s) => {
          const on = s.key === currentKey;
          return (
            <button
              key={s.key}
              type="button"
              onClick={() => onPick(s.key)}
              disabled={applying !== null}
              aria-pressed={on}
              className="flex flex-col items-start gap-2 rounded-mk-md border bg-mk-surface p-3.5 text-left transition-colors duration-[120ms] ease-mk hover:border-mk-accent-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200 disabled:cursor-not-allowed"
              style={on ? { borderColor: "var(--mk-accent-500)" } : { borderColor: "var(--mk-border)" }}
            >
              <div className="flex w-full items-center justify-between gap-2">
                <span className="text-mk-body font-semibold text-mk-ink">{s.name}</span>
                {on && (
                  <span className="text-mk-label" style={{ color: "var(--mk-accent-700)" }}>
                    正在用
                  </span>
                )}
              </div>
              <span className="text-mk-small text-mk-muted">{s.blurb}</span>
              <BlockChips blocks={s.blocks.map((b) => b.role)} />
            </button>
          );
        })}
      </div>
    </section>
  );
}

/**
 * ChosenBlocks — one row per block: the skeleton's label (fixed, not
 * editable) and HER one-line point for it (the only editable thing here).
 *
 * The label is rendered as text, never as an input, and that is deliberate:
 * if she could type over 「反方最强的说法」 the distinction between the
 * template and her thinking would collapse in the data too, and the process
 * report reads exactly that distinction.
 */
function ChosenBlocks({
  writingId,
  structureName,
  hints,
  outline,
  onOutlineChange,
}: {
  writingId: string;
  /** Empty when the library could not be read — the blocks still render. */
  structureName: string;
  hints: string[];
  outline: WritingOutlineItem[];
  onOutlineChange: (next: WritingOutlineItem[]) => void;
}) {
  const [drafts, setDrafts] = useState<Record<string, string>>({});
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const ordered = outline.slice().sort((a, b) => a.position - b.position);
  const valueFor = (item: WritingOutlineItem) => drafts[item.id] ?? item.text;

  async function saveAll() {
    setSaving(true);
    setError(null);
    try {
      const saved = await putWritingOutline(
        writingId,
        ordered.map((it) => ({ text: valueFor(it).trim(), role: it.role, depth: it.depth })),
      );
      onOutlineChange(saved);
      setDrafts({});
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "保存失败，请重试。");
    } finally {
      setSaving(false);
    }
  }

  const dirty = Object.keys(drafts).length > 0;

  return (
    <section className="flex flex-col gap-3">
      <div className="flex items-center justify-between gap-2">
        <span className="text-mk-small text-mk-muted">
          {structureName ? `正在用《${structureName}》· ` : ""}每一块写一句你自己的要点
        </span>
        {dirty && (
          <Button size="sm" variant="secondary" onClick={() => void saveAll()} loading={saving}>
            保存
          </Button>
        )}
      </div>

      <div className="flex flex-col gap-2.5">
        {ordered.map((item, i) => {
          const hint = hints[i] ?? "";
          return (
            <BlockRow
              key={item.id}
              writingId={writingId}
              item={item}
              index={i}
              hint={hint}
              value={valueFor(item)}
              onChange={(v) => setDrafts((d) => ({ ...d, [item.id]: v }))}
              onCommit={() => {
                if (drafts[item.id] !== undefined) void saveAll();
              }}
            />
          );
        })}
      </div>

      {error && (
        <p className="flex items-center gap-1.5 text-mk-small text-mk-danger">
          <Icon icon={RefreshCw} size={13} /> {error}
        </p>
      )}
    </section>
  );
}

/**
 * BlockRow — one skeleton block: the fixed label, her one-line point, and
 * 「想不出来？」.
 *
 * The guiding box lives HERE as well as in 段落 because guidance was asked for
 * at both moments — *"AI should guide me to think about the outlines"* and
 * *"the key is the AI-generated guiding box"* for the paragraphs. Same
 * mechanism at two zoom levels: what this block is for, and then what goes in
 * it. Working out what 「反方最强的说法」 means for HER topic is exactly where
 * a student stalls, and it is upstream of every paragraph she writes after.
 *
 * No card offer is wired here: the summon surface lives in the room's rail and
 * 结构 has no way to hand one over. GuideBox drops the offer when
 * `onSummonCard` is absent rather than rendering a button that goes nowhere.
 */
function BlockRow({
  writingId,
  item,
  index,
  hint,
  value,
  onChange,
  onCommit,
}: {
  writingId: string;
  item: WritingOutlineItem;
  index: number;
  hint: string;
  value: string;
  onChange: (v: string) => void;
  onCommit: () => void;
}) {
  const [guide, setGuide] = useState<WritingBlockGuide | null>(null);
  const [guiding, setGuiding] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const label = item.role || `第 ${index + 1} 块`;

  async function ask() {
    setGuiding(true);
    setError(null);
    try {
      setGuide(await guideWritingBlock(writingId, item.id));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "这次没问出问题来，再试一次。");
    } finally {
      setGuiding(false);
    }
  }

  return (
    <div className="flex flex-col gap-2 rounded-mk-md border border-mk-border bg-mk-surface p-3.5">
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <div className="flex min-w-0 flex-wrap items-baseline gap-2">
          <span
            className="shrink-0 rounded-mk-xs px-1.5 py-0.5 text-mk-label font-semibold"
            style={{ background: "var(--mk-accent-50)", color: "var(--mk-accent-700)" }}
          >
            {label}
          </span>
          {hint && <span className="text-mk-small text-mk-muted">{hint}</span>}
        </div>
        <Button
          variant="ghost"
          size="sm"
          onClick={() => void ask()}
          loading={guiding}
          iconStart={<Icon icon={HelpCircle} size={13} />}
        >
          想不出来？
        </Button>
      </div>

      {guide && <GuideBox guide={guide} onDismiss={() => setGuide(null)} />}

      <input
        value={value}
        onChange={(e) => onChange(e.target.value)}
        onBlur={onCommit}
        placeholder="用一句话写下你在这一块想说什么"
        aria-label={label}
        className="w-full rounded-mk-xs border border-mk-input-border bg-mk-paper px-3 py-2 text-mk-body text-mk-ink outline-none placeholder:text-[#B8ADA2] focus-visible:border-mk-accent focus-visible:ring-2 focus-visible:ring-mk-accent-200"
      />
      {error && <p className="text-mk-small text-mk-danger">{error}</p>}
    </div>
  );
}

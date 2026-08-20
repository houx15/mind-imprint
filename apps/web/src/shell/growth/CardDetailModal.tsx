import { useEffect, useMemo, useState } from "react";
import { createPortal } from "react-dom";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import type { Components } from "react-markdown";
import { Star, X, ArrowRight } from "lucide-react";
import { CARD_REGISTRY, type CardCatalogEntry, type CourseSummary } from "@mind-imprint/contracts";
import { Icon, Badge, coverGradientStyle } from "@/ui";

// The wide two-column tool-card detail modal, shared by the 图鉴 gallery and the
// course detail page (a course's mapped cards open this same modal). Kept in its
// own file so course detail doesn't have to import the whole gallery module.

/** Join truthy class fragments with a single space; drops falsy/empty ones. */
function cx(...parts: Array<string | false | null | undefined>): string {
  return parts.filter(Boolean).join(" ");
}

const SURFACE_LABEL: Record<string, string> = { project: "项目", course: "课程", chat: "聊天" };

/**
 * Read-only 5-star proficiency display; fill is warning gold (spec §8), never
 * accent. `onDark` swaps the empty-star color for legibility over the card
 * tile's dark cover scrim; the default (light-background contexts, e.g. the
 * detail modal) uses a neutral border tone instead.
 */
export function Stars({ n, onDark = false }: { n: number; onDark?: boolean }) {
  return (
    <span aria-label={`熟练度 ${n} 星`} className="inline-flex items-center gap-0.5">
      {Array.from({ length: 5 }, (_, i) => (
        <Icon
          key={i}
          icon={Star}
          size={12}
          className={cx(
            i < n ? "text-mk-warning" : onDark ? "text-white/50" : "text-mk-faint",
            onDark && "drop-shadow-sm",
          )}
          fill={i < n ? "currentColor" : "none"}
        />
      ))}
    </span>
  );
}

type Detail = CardCatalogEntry;

function SectionLabel({ children }: { children: React.ReactNode }) {
  return <div className="mb-1.5 text-mk-label text-mk-faint">{children}</div>;
}

// A labeled field whose body is arbitrary content (markdown, chips, …).
function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div>
      <SectionLabel>{label}</SectionLabel>
      <div>{children}</div>
    </div>
  );
}

// Token-styled markdown for card copy. The card `purpose`/`methodology` fields
// are authored with **bold**, lists and `>` quotes; they were previously dropped
// in as raw strings (markup showed literally) — this renders them, using design
// tokens rather than the chat renderer's hardcoded palette.
const CARD_MD: Components = {
  p: ({ children }) => <p style={{ margin: "0 0 8px", lineHeight: 1.7 }}>{children}</p>,
  strong: ({ children }) => <strong style={{ fontWeight: 700, color: "var(--mk-ink)" }}>{children}</strong>,
  em: ({ children }) => <em>{children}</em>,
  ul: ({ children }) => <ul style={{ margin: "0 0 8px", paddingLeft: 20, listStyle: "disc" }}>{children}</ul>,
  ol: ({ children }) => <ol style={{ margin: "0 0 8px", paddingLeft: 20, listStyle: "decimal" }}>{children}</ol>,
  li: ({ children }) => <li style={{ margin: "3px 0", lineHeight: 1.65 }}>{children}</li>,
  a: ({ children, href }) => (
    <a href={href} target="_blank" rel="noreferrer" style={{ color: "var(--mk-accent-600)", textDecoration: "underline" }}>{children}</a>
  ),
  code: ({ children }) => (
    <code style={{ background: "var(--mk-paper)", borderRadius: 4, padding: "1px 5px", fontSize: "0.92em", fontFamily: "ui-monospace, SFMono-Regular, Menlo, monospace" }}>{children}</code>
  ),
  blockquote: ({ children }) => (
    <blockquote style={{ margin: "0 0 8px", paddingLeft: 12, borderLeft: "3px solid var(--mk-border)", color: "var(--mk-muted)" }}>{children}</blockquote>
  ),
};
function CardMd({ text }: { text: string }) {
  return (
    <div style={{ marginBottom: -8, fontSize: 14, color: "var(--mk-secondary)" }}>
      <ReactMarkdown remarkPlugins={[remarkGfm]} components={CARD_MD}>{text}</ReactMarkdown>
    </div>
  );
}

// The courses that teach a card, as tap-through rows — the "去学它" surface the
// student asked for. Empty → renders nothing (caller decides the empty copy).
function RelatedCourses({ label, courses, onOpenCourse }: { label: string; courses: CourseSummary[]; onOpenCourse?: (slug: string) => void }) {
  if (courses.length === 0) return null;
  return (
    <div>
      <SectionLabel>{label}</SectionLabel>
      <div className="flex flex-col gap-2">
        {courses.map((co) => (
          <button
            key={co.slug}
            type="button"
            onClick={() => onOpenCourse?.(co.slug)}
            disabled={!onOpenCourse}
            className={cx(
              "group flex items-center justify-between gap-3 rounded-mk-md border border-mk-border bg-mk-paper px-3.5 py-2.5 text-left transition-colors duration-[120ms] ease-mk",
              onOpenCourse ? "hover:border-mk-accent-300" : "cursor-default",
            )}
          >
            <span className="min-w-0">
              <span className="block truncate text-mk-body font-semibold text-mk-ink">{co.title}</span>
              <span className="text-mk-small text-mk-muted">{co.branch} · {co.time_label}</span>
            </span>
            {onOpenCourse && <Icon icon={ArrowRight} size={16} className="shrink-0 text-mk-muted group-hover:text-mk-accent-600" />}
          </button>
        ))}
      </div>
    </div>
  );
}

function HStat({ value, label, small = false }: { value: string; label: string; small?: boolean }) {
  return (
    <div className="rounded-mk-md border border-mk-border bg-mk-paper px-3 py-3 text-center">
      <div className={cx("truncate font-extrabold text-mk-ink", small ? "text-mk-body" : "text-[20px]")}>{value}</div>
      <div className="mt-0.5 text-mk-small font-semibold text-mk-muted">{label}</div>
    </div>
  );
}

function TabButton({ active, onClick, children }: { active: boolean; onClick: () => void; children: React.ReactNode }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cx(
        "relative px-3 py-2.5 text-mk-body font-semibold transition-colors duration-[120ms] ease-mk",
        active ? "text-mk-accent-600" : "text-mk-muted hover:text-mk-secondary",
      )}
    >
      {children}
      {active && <span className="absolute inset-x-2 -bottom-px h-[2.5px] rounded-mk-full bg-mk-accent-500" />}
    </button>
  );
}

// Wide two-column detail: left = cover art, right = header + two tabs (介绍 /
// 我的练习历史). Built as its own portal rather than the shared Modal so it can be
// wide and full-height (the shared Modal is hard-locked to a narrow width).
export function CardDetailModal({ c, courses, onClose, onOpenCourse }: { c: Detail; courses: CourseSummary[]; onClose: () => void; onOpenCourse?: (slug: string) => void }) {
  const [tab, setTab] = useState<"intro" | "history">("intro");
  const [imgFailed, setImgFailed] = useState(false);
  const spec = CARD_REGISTRY[c.cardId];
  const methodology = spec?.steps?.[0]?.methodology;
  const example = c.example || methodology?.example || methodology?.how || "";
  const relatedCourses = useMemo(() => courses.filter((co) => co.card_ids.includes(c.cardId)), [courses, c.cardId]);
  const showImg = Boolean(c.coverUrl) && !imgFailed;

  useEffect(() => {
    function onKey(e: KeyboardEvent) { if (e.key === "Escape") onClose(); }
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [onClose]);

  if (typeof document === "undefined") return null;

  return createPortal(
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      <div className="absolute inset-0 bg-black/40" onClick={onClose} aria-hidden="true" />
      {/* Fixed height (not max-height) so switching tabs never resizes the modal;
          each tab's body scrolls inside the constant frame. */}
      <div role="dialog" aria-modal="true" aria-label={c.name} className="relative z-10 flex h-[min(620px,88vh)] w-full max-w-[820px] overflow-hidden rounded-mk-lg bg-mk-surface shadow-mk-lg">
        {/* left — cover art shown WHOLE (object-contain): the covers carry text,
            so cropping would cut it off. Letterboxed on paper. */}
        <div className="hidden w-[40%] shrink-0 items-center justify-center bg-mk-paper p-4 sm:flex">
          {showImg ? (
            <img
              src={c.coverUrl}
              alt={c.name}
              onError={() => setImgFailed(true)}
              className={cx("max-h-full max-w-full rounded-mk-sm object-contain shadow-mk-sm", !c.encountered && "opacity-90")}
            />
          ) : (
            <div className="flex h-full w-full items-center justify-center rounded-mk-sm p-5" style={coverGradientStyle(c.cardId)}>
              <span className="text-center text-mk-h3 font-bold leading-snug text-white drop-shadow-sm">{c.name}</span>
            </div>
          )}
        </div>

        {/* right — header + tabs + body */}
        <div className="flex min-w-0 flex-1 flex-col">
          <div className="flex items-start justify-between gap-3 border-b border-mk-border px-6 pb-4 pt-5">
            <div className="min-w-0">
              <h2 className="text-[22px] font-extrabold leading-tight text-mk-ink">{c.name}</h2>
              {c.nameEn && <div className="mt-1 text-mk-small text-mk-muted">{c.nameEn}</div>}
              <div className="mt-2.5 flex flex-wrap items-center gap-2.5">
                <Badge tone="draft">{c.category}</Badge>
                {c.encountered ? <Stars n={c.stars} /> : <span className="text-mk-small font-semibold text-mk-faint">还没遇到</span>}
              </div>
            </div>
            <button type="button" onClick={onClose} aria-label="关闭" className="shrink-0 rounded-mk-full p-1.5 text-mk-muted transition-colors duration-[120ms] ease-mk hover:bg-mk-paper hover:text-mk-ink">
              <Icon icon={X} size={18} />
            </button>
          </div>

          <div className="flex gap-1 border-b border-mk-border px-4">
            <TabButton active={tab === "intro"} onClick={() => setTab("intro")}>介绍</TabButton>
            <TabButton active={tab === "history"} onClick={() => setTab("history")}>我的练习历史</TabButton>
          </div>

          <div className="mk-scroll min-h-0 flex-1 overflow-y-auto px-6 py-5">
            {tab === "intro" ? (
              <div className="flex flex-col gap-5">
                <Field label="这张卡帮你做什么"><CardMd text={c.purpose} /></Field>
                {c.stages.length > 0 && (
                  <div>
                    <SectionLabel>适用阶段</SectionLabel>
                    <div className="flex flex-wrap gap-1.5">{c.stages.map((s) => <Badge key={s} tone="done">{s}</Badge>)}</div>
                  </div>
                )}
                {methodology?.why && <Field label="为什么用"><CardMd text={methodology.why} /></Field>}
                {methodology?.how && <Field label="怎么用"><CardMd text={methodology.how} /></Field>}
                {methodology?.when && <Field label="什么时候用"><CardMd text={methodology.when} /></Field>}
                {example && <Field label="一个例子"><CardMd text={example} /></Field>}
                <RelatedCourses label="在这些课程里学它" courses={relatedCourses} onOpenCourse={onOpenCourse} />
              </div>
            ) : c.encountered ? (
              <div className="flex flex-col gap-5">
                <div className="grid grid-cols-3 gap-2.5">
                  <HStat value={String(c.uses)} label="练习次数" />
                  <HStat value={String(c.stars)} label="熟练度（星）" />
                  <HStat small value={c.surfaces.length ? c.surfaces.map((s) => SURFACE_LABEL[s] ?? s).join("·") : "—"} label="出现场景" />
                </div>
                <div className="text-mk-body leading-relaxed text-mk-secondary">
                  你已经练过这张卡 <b className="text-mk-ink">{c.uses}</b> 次
                  {c.lastUsed && <>，最近一次在 <b className="text-mk-ink">{c.lastUsed.slice(0, 10)}</b></>}。
                </div>
                {methodology?.why && <Field label="你练的思路"><CardMd text={methodology.why} /></Field>}
                <RelatedCourses label="再练一次" courses={relatedCourses} onOpenCourse={onOpenCourse} />
              </div>
            ) : (
              <div className="flex flex-col gap-4">
                <div className="rounded-mk-md border border-dashed border-mk-border bg-mk-paper px-4 py-6 text-center">
                  <div className="text-mk-body font-semibold text-mk-secondary">你还没在项目里练过这张卡</div>
                  <div className="mt-1.5 text-mk-small text-mk-muted">在下面的课程里第一次遇到它，练过就会记录在这里。</div>
                </div>
                <RelatedCourses label="去学这张卡" courses={relatedCourses} onOpenCourse={onOpenCourse} />
              </div>
            )}
          </div>
        </div>
      </div>
    </div>,
    document.body,
  );
}

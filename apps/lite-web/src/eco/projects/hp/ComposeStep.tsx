import { useState } from "react";
import { Check, CornerDownLeft } from "lucide-react";
import { useEco } from "../../store";
import { MODE_SCRIPTS, workModeById } from "../../data/homepage";
import { READINGS, WRITINGS } from "../../data/library";
import type { HomepageSection } from "../../data/types";
import { Bold, Field, Panel, Sys, cx } from "../../ui";

/**
 * Step 4 · 写内容 — where the three working modes stop being labels.
 *
 * 🚨 The one thing this screen must never do is produce a sentence she can
 * paste. Each mode gives her something different, and none of them is prose:
 *   - `ask`     一次一个问题。她的回答就是正文。
 *   - `propose` 2–3 个**留空的骨架**。空格是她的。
 *   - `tidy`    她先写，AI 只标出重复 / 可合并 / 缺例子，逐条由她决定。
 * If a future edit makes any of these emit a finished sentence, PBL#0 stops
 * teaching and starts ghost-writing.
 *
 * Sections with a `picker` are the other half of the brief: her READING
 * HISTORY and her WRITINGS can be chosen onto the page.
 */
export function ComposeStep() {
  const { state, hpWrite, hpPick } = useEco();
  const hp = state.homepage;
  const enabled = hp.sections.filter((s) => s.enabled);
  const [activeId, setActive] = useState(enabled[0]?.id ?? "intro");
  const active = enabled.find((s) => s.id === activeId) ?? enabled[0];
  const mode = workModeById(hp.mode ?? "ask");

  if (!active) return null;

  return (
    <div className="grid gap-6 lg:grid-cols-[224px_minmax(0,1fr)]">
      {/* section nav */}
      <nav>
        <Sys className="mb-2 block">这一页的几块</Sys>
        <ul className="space-y-1">
          {enabled.map((s, i) => {
            const filled = s.picker ? (s.picked?.length ?? 0) > 0 : s.value.trim().length > 0;
            const on = s.id === active.id;
            return (
              <li key={s.id}>
                <button
                  type="button"
                  onClick={() => setActive(s.id)}
                  className={cx(
                    "flex w-full items-center gap-2.5 rounded-mk-md px-3 py-2.5 text-left transition-colors",
                    "duration-[120ms] ease-mk focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200",
                    on ? "bg-mk-accent-50" : "hover:bg-mk-surface",
                  )}
                >
                  <span
                    className={cx(
                      "flex h-5 w-5 shrink-0 items-center justify-center rounded-mk-full font-mono text-[10px]",
                      filled ? "bg-mk-success text-white" : "border border-mk-input-border text-mk-faint",
                    )}
                  >
                    {filled ? <Check size={11} strokeWidth={3} /> : i + 1}
                  </span>
                  <span
                    className={cx(
                      "min-w-0 flex-1 truncate text-mk-body",
                      on ? "font-semibold text-mk-ink" : "text-mk-secondary",
                    )}
                  >
                    {s.label}
                  </span>
                </button>
              </li>
            );
          })}
        </ul>

        <div className="mt-5 rounded-mk-md p-3.5" style={{ background: "var(--mk-surface)", border: "1px solid var(--mk-border)" }}>
          <Sys>合作方式</Sys>
          <p className="mt-1 text-mk-body font-semibold text-mk-ink">{mode.label}</p>
          <p className="mt-1 text-mk-small leading-relaxed text-mk-muted">
            <Bold text={mode.blurb} />
          </p>
        </div>
      </nav>

      {/* editor */}
      <div>
        {active.picker ? (
          <PickerSection section={active} onPick={(id) => hpPick(active.id, id)} />
        ) : (
          <ProseSection
            section={active}
            modeId={hp.mode ?? "ask"}
            onWrite={(v) => hpWrite(active.id, v)}
          />
        )}
      </div>
    </div>
  );
}

function ProseSection({
  section,
  modeId,
  onWrite,
}: {
  section: HomepageSection;
  modeId: "ask" | "propose" | "tidy";
  onWrite: (v: string) => void;
}) {
  const script = MODE_SCRIPTS[modeId][section.id];
  const [handled, setHandled] = useState<number[]>([]);
  // 🚨 整理式 reads what she wrote. With an empty box it has nothing to read,
  // and printing 「读完了。第 1 句和第 3 句说的是同一件事」 over a blank textarea is
  // exactly the canned-plausible-answer failure the codebase has a rule about
  // ([AI errors must surface, never fake]). Say the true thing instead.
  const tooEarlyToTidy = modeId === "tidy" && section.value.trim().length < 30;

  return (
    <div className="space-y-4">
      <Field
        label={section.label}
        hint={section.hint}
        value={section.value}
        onChange={onWrite}
        rows={7}
        placeholder="从这里开始写。写坏了也没关系，你随时可以改。"
        right={
          <span className="font-mono text-mk-small text-mk-faint">
            {section.value.trim().length} 字
          </span>
        }
      />

      {script ? (
        <Panel className="p-5">
          <div className="flex items-center gap-2">
            <span
              className="flex h-6 w-6 items-center justify-center rounded-mk-full"
              style={{ background: "linear-gradient(140deg,var(--mk-accent-400),var(--mk-accent-600))" }}
            >
              <span className="text-[11px] font-bold text-white">印</span>
            </span>
            <Sys>{workModeById(modeId).short}</Sys>
          </div>
          <p className="mt-2.5 text-mk-body-lg leading-[1.85] text-mk-ink">
            {tooEarlyToTidy
              ? section.value.trim().length === 0
                ? "你还没写。整理式是你先写、我再挑毛病——先写几句，哪怕写得很烂，我再看。"
                : "才几个字，还看不出哪里啰嗦。再写几句我就能帮上忙了。"
              : script.lead}
          </p>

          {/* ── ask ─────────────────────────────────────────────────────── */}
          {modeId === "ask" ? (
            <div className="mt-4">
              {script.items.map((q) => (
                <p key={q} className="text-mk-report-quote leading-[1.7] text-mk-ink">
                  {q}
                </p>
              ))}
              <p className="mt-3 flex items-center gap-2 text-mk-small text-mk-muted">
                <CornerDownLeft size={14} strokeWidth={2} />
                在上面的框里回答。你写的就是正文——我一个字都不会替你写。
              </p>
            </div>
          ) : null}

          {/* ── propose ─────────────────────────────────────────────────── */}
          {modeId === "propose" ? (
            <ul className="mt-4 space-y-2">
              {script.items.map((sk) => (
                <li key={sk}>
                  <button
                    type="button"
                    onClick={() => onWrite(section.value ? `${section.value}\n${stripTag(sk)}` : stripTag(sk))}
                    className="w-full rounded-mk-md border border-mk-border bg-mk-paper p-3.5 text-left
                               transition-colors duration-[140ms] ease-mk hover:border-mk-accent-200
                               hover:bg-mk-accent-50 focus-visible:outline-none focus-visible:ring-2
                               focus-visible:ring-mk-accent-200"
                  >
                    <span className="block text-mk-body leading-[1.85] text-mk-ink">{sk}</span>
                    <span className="mt-1.5 block text-mk-small text-mk-accent-700">
                      用这个骨架 → 空格我自己填
                    </span>
                  </button>
                </li>
              ))}
            </ul>
          ) : null}

          {/* ── tidy ────────────────────────────────────────────────────── */}
          {modeId === "tidy" && !tooEarlyToTidy ? (
            <ul className="mt-4 space-y-2">
              {script.items.map((s, i) => {
                const done = handled.includes(i);
                return (
                  <li
                    key={s}
                    className={cx(
                      "rounded-mk-md border p-3.5 transition-opacity duration-[200ms]",
                      done ? "border-mk-border opacity-50" : "border-mk-border",
                    )}
                    style={{ background: "var(--mk-paper)" }}
                  >
                    <p className="text-mk-body leading-[1.85] text-mk-ink">
                      <span className="eco-mono mr-2 text-mk-faint">{String(i + 1).padStart(2, "0")}</span>
                      {s}
                    </p>
                    <div className="mt-2.5 flex gap-2">
                      <button
                        type="button"
                        onClick={() => setHandled((h) => (h.includes(i) ? h : [...h, i]))}
                        className="rounded-mk-full border border-mk-accent px-3 py-1 text-mk-small text-mk-accent-700
                                   transition-colors hover:bg-mk-accent-50 focus-visible:outline-none"
                      >
                        我改了
                      </button>
                      <button
                        type="button"
                        onClick={() => setHandled((h) => (h.includes(i) ? h : [...h, i]))}
                        className="rounded-mk-full border border-mk-border px-3 py-1 text-mk-small text-mk-secondary
                                   transition-colors hover:bg-mk-surface focus-visible:outline-none"
                      >
                        不改，我有我的理由
                      </button>
                    </div>
                  </li>
                );
              })}
              <li className="pt-1 text-mk-small text-mk-muted">
                我只标位置和理由，不动你的句子——改不改是你的决定。
              </li>
            </ul>
          ) : null}
        </Panel>
      ) : null}
    </div>
  );
}

function PickerSection({
  section,
  onPick,
}: {
  section: HomepageSection;
  onPick: (id: string) => void;
}) {
  const { state } = useEco();
  const picked = section.picked ?? [];

  const items =
    section.picker === "readings"
      ? READINGS.map((r) => ({ id: r.id, title: r.title, meta: `${r.source} · ${r.date}`, note: r.takeaway ?? "" }))
      : section.picker === "writings"
        ? WRITINGS.map((w) => ({ id: w.id, title: w.title, meta: `${w.date} · ${w.words} 字 · ${w.spine}`, note: w.body[0] }))
        : state.projects.map((p) => ({
            id: p.id,
            title: p.title,
            meta: p.status === "published" ? "已发布" : "进行中",
            note: p.summary ?? p.motivation?.who ? `为了 ${p.motivation?.who ?? ""}` : "",
          }));

  return (
    <div>
      <h3 className="text-mk-h2 text-mk-ink">{section.label}</h3>
      <p className="mt-1 text-mk-body text-mk-secondary">{section.hint}</p>
      <p className="mt-1 text-mk-small text-mk-muted">
        已选 <span className="font-mono font-bold text-mk-ink">{picked.length}</span> 个。
        原则 03 说：你自己挑出最好的五件——挑的时候克制一点。
      </p>

      <ul className="mt-4 space-y-2">
        {items.map((it) => {
          const on = picked.includes(it.id);
          return (
            <li key={it.id}>
              <button
                type="button"
                onClick={() => onPick(it.id)}
                className={cx(
                  "flex w-full items-start gap-3 rounded-mk-md border p-4 text-left transition-all duration-[140ms]",
                  "ease-mk focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200",
                  on ? "border-mk-accent bg-mk-accent-50" : "border-mk-border bg-mk-surface hover:border-mk-accent-200",
                )}
              >
                <span
                  className={cx(
                    "mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center rounded-mk-sm border",
                    on ? "border-mk-accent bg-mk-accent" : "border-mk-input-border",
                  )}
                >
                  {on ? <Check size={12} strokeWidth={3} color="#fff" /> : null}
                </span>
                <span className="min-w-0">
                  <span className="block text-mk-h3 text-mk-ink">{it.title}</span>
                  <span className="mt-0.5 block font-mono text-[11px] text-mk-faint">{it.meta}</span>
                  {it.note ? (
                    <span className="mt-1.5 line-clamp-2 block text-mk-body leading-[1.8] text-mk-secondary">
                      {it.note}
                    </span>
                  ) : null}
                </span>
              </button>
            </li>
          );
        })}
      </ul>
    </div>
  );
}

/** Skeletons are shown with a `【角度】` prefix so she can tell them apart in
 *  the list; the prefix is a label, not part of her sentence, so it is dropped
 *  when the skeleton lands in her draft. */
function stripTag(s: string): string {
  return s.replace(/^【[^】]*】/, "").trim();
}

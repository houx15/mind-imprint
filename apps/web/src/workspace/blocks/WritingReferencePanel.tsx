import { useEffect, useState } from "react";
import type { Proposal, Reference } from "@mind-imprint/contracts";
import { Tabs } from "@/ui";
import { getLibrary } from "../api/workspace";

/**
 * WritingReferencePanel — the writing stage's left "reference" sub-pane
 * (agentic studio, spec §2/§6). Read-only context the student writes AGAINST,
 * in three tabs:
 *   提案要点  — the proposal (P1: the four kick-off dimensions, read-only; P2
 *              upgrades these to AI-proposed→confirmed structured notes)
 *   阅读笔记  — per-source reading notes pulled from the library
 *   批注      — 印记's whole-draft feedback (P1: placeholder; wired in a later
 *              phase — there is no annotation stream yet)
 *
 * 铁律: this panel never WRITES the student's thinking — it only surfaces what
 * she already produced. The writing itself stays in the right pane.
 *
 * GOTCHA (design-system convention): exactly ONE Tailwind class per competing
 * CSS property (alphabetical emit order), and never `bg-mk-<token>/<opacity>`
 * (mk tokens are hex → invalid alpha); tints use solid tokens.
 */

type RefTab = "proposal" | "reading" | "notes";

const TABS = [
  { key: "proposal", label: "提案要点" },
  { key: "reading", label: "阅读笔记" },
  { key: "notes", label: "批注" },
];

const PROPOSAL_SECTIONS: { key: keyof Proposal; label: string }[] = [
  { key: "objective", label: "研究问题 / 目标" },
  { key: "reason", label: "动机与意义" },
  { key: "activities", label: "活动计划" },
  { key: "resources", label: "资源与文献" },
];

export function WritingReferencePanel({ projectId, proposal }: { projectId: string; proposal: Proposal }) {
  const [tab, setTab] = useState<RefTab>("proposal");
  const [refs, setRefs] = useState<Reference[] | null>(null);

  // Load the library once when the 阅读笔记 tab is first opened (lazy — the
  // panel is context, not the main event, so don't fetch until it's looked at).
  useEffect(() => {
    if (tab !== "reading" || refs != null) return;
    let cancelled = false;
    getLibrary(projectId)
      .then((lib) => {
        if (!cancelled) setRefs(lib.references);
      })
      .catch(() => {
        if (!cancelled) setRefs([]);
      });
    return () => {
      cancelled = true;
    };
  }, [tab, refs, projectId]);

  return (
    <div className="flex h-full min-h-0 flex-col border-r border-mk-border bg-mk-surface">
      <div className="shrink-0 px-4 pt-3">
        <Tabs tabs={TABS} value={tab} onChange={(k) => setTab(k as RefTab)} />
      </div>
      <div className="mk-scroll min-h-0 flex-1 overflow-y-auto px-4 py-3">
        {tab === "proposal" && <ProposalTab proposal={proposal} />}
        {tab === "reading" && <ReadingTab refs={refs} />}
        {tab === "notes" && <NotesTab />}
      </div>
    </div>
  );
}

function ProposalTab({ proposal }: { proposal: Proposal }) {
  const allEmpty = PROPOSAL_SECTIONS.every((s) => !proposal[s.key].trim());
  if (allEmpty) {
    return (
      <p className="text-[12.5px] leading-relaxed text-mk-faint">
        提案要点还没成形。回到「立项」和印记聊聊你的目标、动机、计划与资源，这里就会长出你的要点。
      </p>
    );
  }
  return (
    <div className="flex flex-col gap-4">
      {PROPOSAL_SECTIONS.map((s) => {
        const value = proposal[s.key].trim();
        return (
          <div key={s.key}>
            <p className="text-[11px] font-bold uppercase tracking-wider text-mk-faint">{s.label}</p>
            {value ? (
              <p className="mt-1 whitespace-pre-line text-[12.5px] leading-relaxed text-mk-ink">{value}</p>
            ) : (
              <p className="mt-1 text-[12.5px] text-mk-faint">还没写</p>
            )}
          </div>
        );
      })}
    </div>
  );
}

function ReadingTab({ refs }: { refs: Reference[] | null }) {
  if (refs == null) {
    return <p className="text-[12.5px] text-mk-faint">加载阅读笔记中…</p>;
  }
  const withNotes = refs.filter(
    (r) => (r.readingNote ?? "").trim() || r.notes.length > 0 || r.takeaway != null,
  );
  if (withNotes.length === 0) {
    return (
      <p className="text-[12.5px] leading-relaxed text-mk-faint">
        还没有阅读笔记。在「阅读」里和印记逐句共读一篇来源，你确认的发现会出现在这里，写作时随手可查。
      </p>
    );
  }
  return (
    <div className="flex flex-col gap-4">
      {withNotes.map((r) => (
        <div key={r.id} className="rounded-mk-sm border border-mk-border bg-mk-paper p-2.5">
          <p className="text-[12.5px] font-bold leading-snug text-mk-ink">{r.title}</p>
          {(r.readingNote ?? "").trim() && (
            <p className="mt-1.5 whitespace-pre-line text-[12px] leading-relaxed text-mk-muted">{r.readingNote}</p>
          )}
          {r.notes.length > 0 && (
            <ul className="mt-1.5 flex flex-col gap-1.5">
              {r.notes.map((n, i) => (
                <li key={i} className="text-[12px] leading-relaxed text-mk-muted">
                  <span className="text-mk-faint">「{n.quote}」</span> — {n.finding}
                </li>
              ))}
            </ul>
          )}
        </div>
      ))}
    </div>
  );
}

function NotesTab() {
  return (
    <p className="text-[12.5px] leading-relaxed text-mk-faint">
      写完后，让印记做一次「整稿体检」——它对结构、论证、清晰度的批注会出现在这里。
    </p>
  );
}

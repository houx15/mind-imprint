import type { Reference, ReferenceRef, Snippet } from "@mind-imprint/contracts";

// resolveReferences turns 印记's curated ReferenceRef[] (studio_state.reference —
// just {kind,id,label} pointers) into rich, renderable content for the dynamic
// reference panel (P3 Task 3), by looking each id up in the student's own
// library (lib) and snippet board (snippets). Pure — no fetching, no I/O; the
// panel owns loading lib/snippets and passing them in.
export type ResolvedRef =
  | { kind: "material"; id: string; label: string; title: string; fragments: string[]; missing?: false }
  | { kind: "note"; id: string; label: string; quote: string; finding: string; missing?: false }
  | { kind: "annotation"; id: string; label: string; text: string; missing?: false }
  | { kind: ReferenceRef["kind"]; id: string; label: string; missing: true };

// materialFragments flattens a Reference's note-bearing fields into individually
// readable strings — same fields/order as MaterialsSidebar.tsx's referenceChunks
// (我的笔记 → 归纳/proposalImpact → findings → key quotes → per-note quote→finding),
// collapsed to plain text since the panel doesn't need per-chunk labels.
function materialFragments(r: Reference): string[] {
  const out: string[] = [];
  if (r.readingNote && r.readingNote.trim()) out.push(r.readingNote.trim());
  const tk = r.takeaway;
  if (tk) {
    if (tk.proposalImpact && tk.proposalImpact.trim()) out.push(tk.proposalImpact.trim());
    for (const f of tk.findings ?? []) if (f.trim()) out.push(f.trim());
    for (const q of tk.keyQuotes ?? []) {
      if (!q.quote?.trim()) continue;
      out.push(q.why?.trim() ? `"${q.quote.trim()}" — ${q.why.trim()}` : `"${q.quote.trim()}"`);
    }
  }
  for (const n of r.notes ?? []) {
    const parts = [n.quote?.trim(), n.finding?.trim()].filter(Boolean);
    if (parts.length) out.push(parts.join(" — "));
  }
  return out;
}

export function resolveReferences(refs: ReferenceRef[], lib: Reference[], snippets: Snippet[]): ResolvedRef[] {
  return refs.map((ref): ResolvedRef => {
    if (ref.kind === "material") {
      const r = lib.find((x) => x.id === ref.id);
      if (!r) return { kind: "material", id: ref.id, label: ref.label, missing: true };
      return { kind: "material", id: ref.id, label: ref.label, title: r.title, fragments: materialFragments(r) };
    }

    if (ref.kind === "note") {
      // A "note" ref's id is loosely scoped today: it may be a snippet id (the
      // student's saved writing-room fragment) OR — once reference-level notes
      // grow a stable id of their own — a Reference.notes[] entry. Snippets are
      // checked first since they're the concrete, addressable case; the
      // Reference.notes[] scan is best-effort until that id scheme exists.
      const snip = snippets.find((s) => s.id === ref.id);
      if (snip) return { kind: "note", id: ref.id, label: ref.label, quote: snip.section ?? "", finding: snip.text };

      for (const r of lib) {
        const found = r.notes?.find((_n, i) => `${r.id}:${i}` === ref.id);
        if (found) return { kind: "note", id: ref.id, label: ref.label, quote: found.quote, finding: found.finding };
      }

      return { kind: "note", id: ref.id, label: ref.label, missing: true };
    }

    // kind === "annotation": no annotation entity exists yet in the data model
    // (deferred — see PR description). The resolved shape is kept ready so the
    // panel can render it once annotations land, but every lookup misses today.
    return { kind: "annotation", id: ref.id, label: ref.label, missing: true };
  });
}

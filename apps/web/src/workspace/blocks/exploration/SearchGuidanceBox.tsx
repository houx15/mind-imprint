import { useState } from "react";
import type { SearchSuggestion } from "@mind-imprint/contracts";
import { proposeSearchGuidance } from "../../../api/searchGuidance";

// SearchGuidanceBox — slice 5 (§113/§116) · the reading-room guidance box. 印记
// proposes 2–3 search directions (keyword + why); the student runs one with a
// single click, or ignores them (铁律②). Generation is student-initiated (a tap),
// so it never auto-spends tokens or nags. This is the 检索方向审视 posture: help the
// student pick WHERE to look — never fetch or read for them (铁律①).
export function SearchGuidanceBox({
  projectId,
  onSearch,
}: {
  projectId: string;
  onSearch: (keyword: string) => void;
}) {
  const [suggestions, setSuggestions] = useState<SearchSuggestion[] | null>(null);
  const [loading, setLoading] = useState(false);

  async function propose() {
    setLoading(true);
    try {
      setSuggestions(await proposeSearchGuidance(projectId));
    } catch {
      setSuggestions([]);
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="rounded-mk-md border border-mk-border bg-mk-surface p-3">
      <div className="flex items-center gap-2">
        <span className="rounded-full bg-mk-accent-50 px-2 py-0.5 text-[12px] font-bold text-mk-accent">检索方向</span>
        <p className="min-w-0 flex-1 truncate text-[13px] text-mk-muted">不知道搜什么？让印记根据你的问题给几个方向。</p>
        <button
          type="button"
          onClick={() => void propose()}
          disabled={loading}
          className="flex-none rounded-mk border border-mk-border px-2.5 py-1 text-[12px] font-bold text-mk-accent hover:bg-mk-accent-50 disabled:opacity-50"
        >
          {loading ? "印记在想…" : suggestions ? "换一批" : "让印记建议检索方向"}
        </button>
      </div>

      {suggestions && suggestions.length > 0 && (
        <ul className="mt-2.5 flex flex-col gap-1.5">
          {suggestions.map((s, i) => (
            <li key={i} className="flex items-start gap-2 rounded-mk border border-mk-border bg-mk-paper px-2.5 py-1.5">
              <div className="min-w-0 flex-1">
                <p className="truncate text-[13.5px] font-semibold text-mk-ink">{s.keyword}</p>
                {s.why && <p className="mt-0.5 text-[12px] leading-relaxed text-mk-muted">{s.why}</p>}
              </div>
              <button
                type="button"
                onClick={() => onSearch(s.keyword)}
                className="flex-none rounded-mk bg-mk-accent px-2.5 py-1 text-[12px] font-bold text-white hover:bg-mk-accent-600"
              >
                搜索
              </button>
            </li>
          ))}
        </ul>
      )}
      {suggestions && suggestions.length === 0 && !loading && (
        <p className="mt-2 text-[12px] text-mk-faint">这次没给出方向，先确认你已经写下研究问题，再试一次。</p>
      )}
    </div>
  );
}

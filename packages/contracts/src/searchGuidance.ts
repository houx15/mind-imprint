import { z } from "zod";

// searchGuidance.ts — slice 5 (§113/§116) · 印记's reading-room search guidance:
// 2–3 suggested search directions, each a keyword + a one-line "why", that the
// student can run with one click. Generated on demand (POST spends; 铁律② — the
// student asks), never auto-nagging.
export const SearchSuggestion = z.object({
  keyword: z.string(),
  why: z.string(),
});
export type SearchSuggestion = z.infer<typeof SearchSuggestion>;

export const SearchGuidance = z.object({
  suggestions: z.array(SearchSuggestion),
});
export type SearchGuidance = z.infer<typeof SearchGuidance>;

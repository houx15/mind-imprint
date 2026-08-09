import { describe, it, expect } from "vitest";
import { ResourceNeed, SearchSuggestion } from "../src/index";

describe("slice 5 contracts", () => {
  it("ResourceNeed defaults done=false", () => {
    expect(ResourceNeed.parse({ id: "a", text: "x" }).done).toBe(false);
  });
  it("SearchSuggestion parses", () => {
    const s = SearchSuggestion.parse({ keyword: "k", why: "w" });
    expect(s.keyword).toBe("k");
  });
});

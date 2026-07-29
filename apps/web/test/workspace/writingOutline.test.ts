import { describe, it, expect } from "vitest";
import { outlineKey } from "../../src/workspace/blocks/WritingBlock";

type Row = { id: string; text: string; depth: number };

// A deterministic id factory so results are assertable without touching the
// module's monotonic temp-id counter.
const fixedId = (v: string) => () => v;

const rows = (): Row[] => [
  { id: "a", text: "第一条", depth: 0 },
  { id: "b", text: "", depth: 1 },
  { id: "c", text: "第三条", depth: 2 },
];

describe("outlineKey (Write room outline keyboard editing)", () => {
  it("enter inserts a blank sibling at the same depth right below and focuses it", () => {
    const res = outlineKey(rows(), "enter", "a", fixedId("new"));
    expect(res.rows.map((r) => r.id)).toEqual(["a", "new", "b", "c"]);
    expect(res.rows[1]).toEqual({ id: "new", text: "", depth: 0 });
    expect(res.focus).toEqual({ id: "new", atEnd: false });
  });

  it("indent increases depth clamped at 2 and keeps caret put (focus null)", () => {
    const res = outlineKey(rows(), "indent", "a");
    expect(res.rows.find((r) => r.id === "a")!.depth).toBe(1);
    expect(res.focus).toBeNull();
  });

  it("indent is a no-op at max depth (returns the same reference)", () => {
    const before = rows();
    const res = outlineKey(before, "indent", "c");
    expect(res.rows).toBe(before);
    expect(res.focus).toBeNull();
  });

  it("outdent decreases depth clamped at 0", () => {
    expect(outlineKey(rows(), "outdent", "c").rows.find((r) => r.id === "c")!.depth).toBe(1);
    const atZero = outlineKey(rows(), "outdent", "a");
    expect(atZero.rows).toEqual(rows()); // already at 0 → unchanged
  });

  it("backspace on an empty non-first row deletes it and focuses the end of the previous row", () => {
    const res = outlineKey(rows(), "backspace", "b");
    expect(res.rows.map((r) => r.id)).toEqual(["a", "c"]);
    expect(res.focus).toEqual({ id: "a", atEnd: true });
  });

  it("backspace is a no-op on a non-empty row", () => {
    const before = rows();
    const res = outlineKey(before, "backspace", "c");
    expect(res.rows).toBe(before);
    expect(res.focus).toBeNull();
  });

  it("backspace is a no-op on the first row even when empty", () => {
    const input: Row[] = [
      { id: "a", text: "", depth: 0 },
      { id: "b", text: "x", depth: 0 },
    ];
    const res = outlineKey(input, "backspace", "a");
    expect(res.rows).toBe(input);
    expect(res.focus).toBeNull();
  });

  it("returns the input untouched for an unknown id", () => {
    const before = rows();
    const res = outlineKey(before, "enter", "missing");
    expect(res.rows).toBe(before);
    expect(res.focus).toBeNull();
  });
});

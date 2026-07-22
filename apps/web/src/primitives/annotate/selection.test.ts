import { describe, it, expect, afterEach } from "vitest";
import { rangeToSpan, selectionToSpan } from "./selection";

function block(id: string, textNodes: (string | { mark: string })[]): { el: HTMLElement; nodes: Text[] } {
  const p = document.createElement("p");
  p.setAttribute("data-block-id", id);
  const nodes: Text[] = [];
  for (const part of textNodes) {
    if (typeof part === "string") {
      const t = document.createTextNode(part);
      p.appendChild(t);
      nodes.push(t);
    } else {
      const mark = document.createElement("mark");
      const t = document.createTextNode(part.mark);
      mark.appendChild(t);
      p.appendChild(mark);
      nodes.push(t);
    }
  }
  document.body.appendChild(p);
  return { el: p, nodes };
}

afterEach(() => {
  document.body.innerHTML = "";
});

describe("rangeToSpan", () => {
  it("yields blockId + rune offsets + text for a selection inside one block", () => {
    const { nodes } = block("b1", ["hello world"]);
    const range = document.createRange();
    range.setStart(nodes[0]!, 0);
    range.setEnd(nodes[0]!, 5);

    expect(rangeToSpan(range)).toEqual({ blockId: "b1", start: 0, end: 5, text: "hello" });
  });

  it("round trips rune offsets for Chinese text", () => {
    const full = "过去二十年里发生了一件几乎没人注意到的事";
    const { nodes } = block("b1", [full]);
    const range = document.createRange();
    range.setStart(nodes[0]!, 2);
    range.setEnd(nodes[0]!, 6);

    const span = rangeToSpan(range);
    expect(span).not.toBeNull();
    expect(Array.from(full).slice(span!.start, span!.end).join("")).toBe(span!.text);
  });

  it("returns null for a collapsed selection", () => {
    const { nodes } = block("b1", ["hello world"]);
    const range = document.createRange();
    range.setStart(nodes[0]!, 3);
    range.setEnd(nodes[0]!, 3);

    expect(rangeToSpan(range)).toBeNull();
  });

  it("returns null for a selection spanning two blocks", () => {
    const { nodes: n1 } = block("b1", ["hello"]);
    const { nodes: n2 } = block("b2", ["world"]);
    const range = document.createRange();
    range.setStart(n1[0]!, 0);
    range.setEnd(n2[0]!, 3);

    expect(rangeToSpan(range)).toBeNull();
  });

  it("returns null for a selection outside any [data-block-id]", () => {
    const div = document.createElement("div");
    const t = document.createTextNode("no block here");
    div.appendChild(t);
    document.body.appendChild(div);
    const range = document.createRange();
    range.setStart(t, 0);
    range.setEnd(t, 5);

    expect(rangeToSpan(range)).toBeNull();
  });

  it("accumulates offsets across sibling nodes when selection starts inside a <mark> run and continues into a plain run", () => {
    const { nodes } = block("b1", [{ mark: "XXXX" }, "def"]);
    const range = document.createRange();
    range.setStart(nodes[0]!, 2); // inside <mark>XXXX</mark>, offset 2
    range.setEnd(nodes[1]!, 2); // inside the plain "def" run, offset 2

    // full block text = "XXXX" + "def" = "XXXXdef"
    // start = 0 + runeLen("XX") = 2; end = 4 + runeLen("de") = 6
    expect(rangeToSpan(range)).toEqual({ blockId: "b1", start: 2, end: 6, text: "XXde" });
  });
});

describe("selectionToSpan", () => {
  it("delegates to rangeToSpan via window.getSelection()", () => {
    const { nodes } = block("b1", ["abcdef"]);
    const range = document.createRange();
    range.setStart(nodes[0]!, 0);
    range.setEnd(nodes[0]!, 3);
    const sel = window.getSelection()!;
    sel.removeAllRanges();
    sel.addRange(range);

    expect(selectionToSpan()).toEqual({ blockId: "b1", start: 0, end: 3, text: "abc" });
  });

  it("returns null when there is no selection", () => {
    window.getSelection()!.removeAllRanges();
    expect(selectionToSpan()).toBeNull();
  });
});

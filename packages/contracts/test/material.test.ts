import { describe, it, expect } from "vitest";
import { Material } from "../src/material";

const valid = {
  id: "m1",
  task_id: "t1",
  kind: "article",
  source: "fetched",
  title: "卫星图看中国变绿",
  source_url: "https://example.com/a",
  blocks: [{ id: "b0", text: "第一段。" }, { id: "b1", text: "第二段。" }],
  scratch: "",
  created_at: "2026-07-06T00:00:00Z",
};

describe("Material contract", () => {
  it("parses a valid material", () => {
    expect(Material.parse(valid)).toMatchObject({ id: "m1", kind: "article" });
  });
  it("allows a null source_url (pasted draft)", () => {
    expect(Material.parse({ ...valid, source: "pasted", kind: "draft", source_url: null }).source_url).toBeNull();
  });
  it("rejects an unknown kind", () => {
    expect(Material.safeParse({ ...valid, kind: "pdf" }).success).toBe(false);
  });
  it("rejects an unknown source", () => {
    expect(Material.safeParse({ ...valid, source: "scraped" }).success).toBe(false);
  });
});

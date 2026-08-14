import { describe, it, expect, vi, afterEach } from "vitest";
import { recordCitation } from "@/api/citations";
import { recordAnnotationOpen } from "@/api/proposalAnnotations";

afterEach(() => { vi.restoreAllMocks(); });

describe("recordCitation", () => {
  it("POSTs to .../citations with referenceId + section and expects no body", async () => {
    const spy = vi.fn(async () => new Response(JSON.stringify({ id: "c1" }), {
      status: 201, headers: { "Content-Type": "application/json" },
    }));
    vi.stubGlobal("fetch", spy);

    await recordCitation("p1", "ref-9", "claim:abc");

    const [url, init] = spy.mock.calls[0] as unknown as [string, RequestInit & { body: string }];
    expect(url).toContain("/api/v1/projects/p1/citations");
    expect(init.method).toBe("POST");
    expect(JSON.parse(init.body)).toEqual({ referenceId: "ref-9", section: "claim:abc" });
  });
});

describe("recordAnnotationOpen", () => {
  it("POSTs to .../annotations/open with annotationId + doc (204, no body)", async () => {
    const spy = vi.fn(async () => new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", spy);

    await recordAnnotationOpen("p1", "anno-3", "essay");

    const [url, init] = spy.mock.calls[0] as unknown as [string, RequestInit & { body: string }];
    expect(url).toContain("/api/v1/projects/p1/annotations/open");
    expect(init.method).toBe("POST");
    expect(JSON.parse(init.body)).toEqual({ annotationId: "anno-3", doc: "essay" });
  });

  it("defaults doc to proposal", async () => {
    const spy = vi.fn(async () => new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", spy);

    await recordAnnotationOpen("p1", "anno-1");

    const [, init] = spy.mock.calls[0] as unknown as [string, RequestInit & { body: string }];
    expect(JSON.parse(init.body)).toEqual({ annotationId: "anno-1", doc: "proposal" });
  });
});

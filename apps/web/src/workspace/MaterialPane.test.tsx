import React from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import type { Material } from "@mind-imprint/contracts";

vi.mock("../api", async (orig) => {
  const real = await orig<typeof import("../api")>();
  return {
    ...real,
    api: { ...real.api, listMaterials: vi.fn(), fetchMaterialFromSeed: vi.fn(), createMaterial: vi.fn(), saveScratch: vi.fn() },
  };
});

import { api, MaterialFetchError } from "../api";
import { MaterialPane } from "./MaterialPane";

const article: Material = {
  id: "m1", task_id: "t1", kind: "article", source: "fetched", title: "卫星图看中国变绿",
  source_url: "https://x", blocks: [{ id: "b0", text: "第一段。" }, { id: "b1", text: "第二段。" }], scratch: "", created_at: "1",
};

describe("MaterialPane", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders existing material blocks", async () => {
    (api.listMaterials as any).mockResolvedValue([article]);
    render(<MaterialPane taskId="t1" seedUrl="https://x" />);
    expect(await screen.findByText("第一段。")).toBeInTheDocument();
    expect(screen.getByText("第二段。")).toBeInTheDocument();
    expect(api.fetchMaterialFromSeed).not.toHaveBeenCalled();
  });

  it("auto-fetches from seed when empty and renders the result", async () => {
    (api.listMaterials as any).mockResolvedValue([]);
    (api.fetchMaterialFromSeed as any).mockResolvedValue(article);
    render(<MaterialPane taskId="t1" seedUrl="https://x" />);
    expect(await screen.findByText("第一段。")).toBeInTheDocument();
    expect(api.fetchMaterialFromSeed).toHaveBeenCalledWith("t1");
  });

  it("shows the paste fallback when the fetch fails, and creates on submit", async () => {
    (api.listMaterials as any).mockResolvedValue([]);
    (api.fetchMaterialFromSeed as any).mockRejectedValue(new MaterialFetchError("blocked"));
    (api.createMaterial as any).mockResolvedValue({ ...article, source: "pasted", blocks: [{ id: "b0", text: "我粘的。" }] });
    render(<MaterialPane taskId="t1" seedUrl="https://x" />);
    const box = await screen.findByPlaceholderText(/把材料贴进来/);
    fireEvent.change(box, { target: { value: "我粘的。" } });
    fireEvent.click(screen.getByText("加入材料"));
    await waitFor(() => expect(api.createMaterial).toHaveBeenCalledWith("t1", expect.objectContaining({ text: "我粘的。" })));
    expect(await screen.findByText("我粘的。")).toBeInTheDocument();
  });

  it("shows the paste fallback immediately when there is no seed", async () => {
    (api.listMaterials as any).mockResolvedValue([]);
    render(<MaterialPane taskId="t1" seedUrl={null} />);
    expect(await screen.findByPlaceholderText(/把材料贴进来/)).toBeInTheDocument();
    expect(api.fetchMaterialFromSeed).not.toHaveBeenCalled();
  });

  it("renders under StrictMode without hanging", async () => {
    (api.listMaterials as any).mockResolvedValue([article]);
    render(
      <React.StrictMode>
        <MaterialPane taskId="t1" seedUrl="https://x" />
      </React.StrictMode>,
    );
    expect(await screen.findByText("第一段。")).toBeInTheDocument();
  });

  it("fetches from seed at most once under StrictMode", async () => {
    (api.listMaterials as any).mockResolvedValue([]);
    (api.fetchMaterialFromSeed as any).mockResolvedValue(article);
    render(
      <React.StrictMode>
        <MaterialPane taskId="t1" seedUrl="https://x" />
      </React.StrictMode>,
    );
    expect(await screen.findByText("第一段。")).toBeInTheDocument();
    expect(api.fetchMaterialFromSeed).toHaveBeenCalledTimes(1);
  });
});

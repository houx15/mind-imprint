import { render } from "@testing-library/react";
import { getBlockRenderer, blockRenderers } from "../src/blocks/registry";
import { NotImplementedRenderer } from "../src/blocks/NotImplementedRenderer";
import type { AssetResolver, SliceEmitter } from "@mind-imprint/course-runtime";

const assetResolver: AssetResolver = { resolve: (p) => `/resolved/${p}` };
const emit: SliceEmitter = () => {};

describe("block registry", () => {
  it("returns a component for a built-in type", () => {
    const R = getBlockRenderer("text");
    expect(typeof R).toBe("function");
  });

  it("maps not-yet-built types to the NotImplementedRenderer placeholder", () => {
    expect(getBlockRenderer("singleChoice")).toBe(NotImplementedRenderer);
    expect(getBlockRenderer("video")).toBe(NotImplementedRenderer);
    expect(getBlockRenderer("pdf")).toBe(NotImplementedRenderer);
    expect(getBlockRenderer("fillBlank")).toBe(NotImplementedRenderer);
    expect(getBlockRenderer("interactiveHtml")).toBe(NotImplementedRenderer);
  });

  it("throws for an unregistered block type (fail before playback)", () => {
    expect(() => getBlockRenderer("carousel")).toThrow();
  });

  it("registry is total over all seven block types", () => {
    expect(Object.keys(blockRenderers).sort()).toEqual(
      ["fillBlank", "images", "interactiveHtml", "pdf", "singleChoice", "text", "video"].sort(),
    );
  });

  it("NotImplementedRenderer renders a labelled data-block-type box", () => {
    const R = getBlockRenderer("singleChoice");
    const { container } = render(
      <R
        block={{ id: "q1", type: "singleChoice", prompt: "x", options: [], assessment: { mode: "survey" }, completion: { rule: "submit-any" } } as any}
        assetResolver={assetResolver}
        state={{ visible: true, enabled: true, completed: false }}
        visible
        enabled
        emit={emit}
      />,
    );
    const box = container.querySelector('[data-not-implemented="true"]');
    expect(box).not.toBeNull();
    expect(box).toHaveAttribute("data-block-type", "singleChoice");
  });
});

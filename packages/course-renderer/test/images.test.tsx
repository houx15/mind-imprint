import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ImagesRenderer } from "../src/blocks/ImagesRenderer";
import type { ImagesBlock } from "@mind-imprint/course-contract";
import type { AssetResolver, SliceEmitter } from "@mind-imprint/course-runtime";

const assetResolver: AssetResolver = { resolve: (p) => `/resolved/${p}` };

function renderImages(block: ImagesBlock, opts: { emit?: SliceEmitter; focusedItemId?: string } = {}) {
  const emit = opts.emit ?? (() => {});
  return render(
    <ImagesRenderer
      block={block}
      assetResolver={assetResolver}
      state={{ visible: true, enabled: true, completed: false }}
      visible
      enabled
      focusedItemId={opts.focusedItemId}
      emit={emit}
    />,
  );
}

const block = (presentation: ImagesBlock["presentation"], n: number): ImagesBlock => ({
  id: "pics",
  type: "images",
  presentation,
  items: Array.from({ length: n }, (_, i) => ({ id: `item-${i + 1}`, source: `img/${i + 1}.png`, alt: `alt ${i + 1}` })),
});

describe("ImagesRenderer", () => {
  it("single renders one <img> with resolved src + alt", () => {
    const { container } = renderImages(block("single", 3));
    const imgs = container.querySelectorAll("img");
    expect(imgs).toHaveLength(1);
    expect(imgs[0]).toHaveAttribute("src", "/resolved/img/1.png");
    expect(imgs[0]).toHaveAttribute("alt", "alt 1");
  });

  it("side-by-side renders two images", () => {
    const { container } = renderImages(block("side-by-side", 2));
    expect(container.querySelectorAll("img")).toHaveLength(2);
  });

  it("gallery shows one item and firing next emits image.selected with the new item id", async () => {
    const user = userEvent.setup();
    const events: Array<{ sourceId: string; type: string; payload: unknown }> = [];
    const emit: SliceEmitter = (sourceId, type, payload) => events.push({ sourceId, type: String(type), payload });
    renderImages(block("gallery", 3), { emit });

    // one image visible at a time
    expect(screen.getAllByRole("img")).toHaveLength(1);

    await user.click(screen.getByRole("button", { name: "下一张" }));
    expect(events).toHaveLength(1);
    expect(events[0]).toMatchObject({ sourceId: "pics", type: "image.selected", payload: { itemId: "item-2" } });
  });

  it("focusedItemId marks the matching item", () => {
    const { container } = renderImages(block("side-by-side", 2), { focusedItemId: "item-2" });
    const focused = container.querySelectorAll('[data-focused="true"]');
    expect(focused).toHaveLength(1);
    expect(focused[0]).toHaveAttribute("data-item-id", "item-2");
  });
});

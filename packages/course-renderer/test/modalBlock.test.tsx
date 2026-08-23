import { act, fireEvent, render, screen } from "@testing-library/react";
import type { SliceDefinition } from "@mind-imprint/course-contract";
import { RuntimeEventBus, InMemorySessionAdapter, type CourseRuntimeAdapters } from "@mind-imprint/course-runtime";
import { SlicePlayer } from "../src/slice/SlicePlayer";
import { AudioEngineProvider } from "../src/narration/audioEngine";
import { FakeAudioEngine } from "./support/fakeAudioEngine";

const PART_ID = "modal-part";
const clock = () => "2026-08-23T00:00:00.000Z";

function makeIdFactory() {
  let n = 0;
  return () => `ev-${++n}`;
}

function fallbackGenerator() {
  return { generate: async () => ({ text: "", generatedAt: clock(), usedSignalTypes: [], fallbackUsed: true }) };
}

/**
 * A PPT-sized figure that needs the whole slot, with its question authored
 * `presentation: "modal"` so it arrives over the figure instead of shrinking it.
 */
const modalSlice: SliceDefinition = {
  id: "modal-slice",
  title: "看图作答",
  objectiveIds: ["obj-one"],
  estimatedSeconds: 90,
  blocks: [
    {
      id: "figure",
      type: "images",
      presentation: "single",
      items: [{ id: "fig-1", source: "images/slide-14.png", alt: "两组比较" }],
    },
    {
      id: "question",
      type: "singleChoice",
      prompt: "这两组能直接比较吗？",
      options: [
        { id: "yes", label: "能" },
        { id: "not-yet", label: "还不能" },
      ],
      assessment: { mode: "survey" },
      completion: { rule: "submit-any" },
      presentation: "modal",
    },
  ],
  layout: { preset: "full", slots: [{ id: "main", blockIds: ["figure", "question"] }] },
  narrations: [],
  workflow: {
    version: "1.0",
    initialStepId: "ask",
    initialState: { visibleBlockIds: ["figure", "question"], enabledBlockIds: ["figure", "question"] },
    steps: [
      {
        id: "ask",
        enterActions: [{ type: "enable", targetId: "question" }],
        transitions: [{ on: { type: "block.completed", sourceId: "question" }, to: "done" }],
      },
      { id: "done", enterActions: [{ type: "clearFocus" }, { type: "completeSlice" }], transitions: [] },
    ],
  },
  navigation: { revisit: "restore-completed-state", autoNext: false, previous: "allowed", manualNext: "after-completion" },
};

async function setup(slice: SliceDefinition) {
  const sessionAdapter = new InMemorySessionAdapter({ idFactory: makeIdFactory(), clock });
  const session = await sessionAdapter.create({ courseId: "modal-course", studentId: "student-1" });
  const adapters: CourseRuntimeAdapters = {
    assetResolver: { resolve: (p) => `/resolved/${p}` },
    sessionAdapter,
    openingGenerator: fallbackGenerator(),
    closingGenerator: fallbackGenerator(),
  };
  const bus = new RuntimeEventBus({ courseId: "modal-course", sessionId: session.id, idFactory: makeIdFactory(), clock });
  const engine = new FakeAudioEngine();
  const onSliceComplete = vi.fn();
  let container!: HTMLElement;
  await act(async () => {
    const r = render(
      <AudioEngineProvider value={engine}>
        <SlicePlayer
          slice={slice}
          partId={PART_ID}
          sessionId={session.id}
          adapters={adapters}
          bus={bus}
          onSliceComplete={onSliceComplete}
          onNavigateNext={vi.fn()}
        />
      </AudioEngineProvider>,
    );
    container = r.container;
  });
  return { container, onSliceComplete };
}

const dialog = () => document.querySelector('[role="dialog"]');
const launcher = (c: HTMLElement) => c.querySelector("[data-modal-launcher]") as HTMLButtonElement;

describe("presentation: modal", () => {
  it("opens over the slice on entry and keeps the figure's slot free", async () => {
    const { container } = await setup(modalSlice);

    // The question is in a dialog, not competing with the figure for slot height.
    expect(dialog()).not.toBeNull();
    expect(dialog()!.textContent).toContain("这两组能直接比较吗？");
    // The figure still renders in the slot.
    expect(container.querySelector('[data-block-id="figure"]')).not.toBeNull();
    // What the slot holds for the question is only the compact launcher.
    const host = container.querySelector('[data-block-id="question"][data-modal-host]');
    expect(host).not.toBeNull();
    expect(host!.querySelector('[data-block-type="singleChoice"]')).toBeNull();
  });

  it("can be dismissed to study the figure and reopened from the launcher", async () => {
    const { container } = await setup(modalSlice);

    fireEvent.click(screen.getByRole("button", { name: "关闭" }));
    expect(dialog()).toBeNull();

    fireEvent.click(launcher(container));
    expect(dialog()).not.toBeNull();
  });

  it("closes once the question completes, and cannot be answered twice", async () => {
    const { container, onSliceComplete } = await setup(modalSlice);

    fireEvent.click(screen.getByRole("radio", { name: "还不能" }));
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "提交" }));
    });

    expect(onSliceComplete).toHaveBeenCalledTimes(1);
    // The dialog gets out of the way so the figure is unobstructed again.
    expect(dialog()).toBeNull();
    expect(launcher(container)).toHaveAttribute("data-completed", "true");

    // Reopening remounts the renderer; it must come back INERT so a second
    // submission can never drive the workflow past where the author authored.
    fireEvent.click(launcher(container));
    expect(dialog()).not.toBeNull();
    expect(screen.getByRole("radio", { name: "还不能" })).toBeDisabled();
    expect(onSliceComplete).toHaveBeenCalledTimes(1);
  });

  it("leaves an inline-presented question in its slot", async () => {
    // NOTE `presentation` is per block TYPE: on `images` it selects the item
    // layout (single / side-by-side / gallery); on an assessment block it is
    // this inline-vs-modal axis. Rebuild the question explicitly rather than
    // mapping over the block union, so neither meaning leaks onto the figure.
    const [figure, question] = modalSlice.blocks;
    const inline: SliceDefinition = {
      ...modalSlice,
      blocks: [figure!, { ...(question as Extract<typeof question, { type: "singleChoice" }>), presentation: "inline" }],
    };
    const { container } = await setup(inline);
    expect(dialog()).toBeNull();
    expect(container.querySelector('[data-block-type="singleChoice"]')).not.toBeNull();
  });
});

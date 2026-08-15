import { render } from "@testing-library/react";
import type { BlockSessionState } from "@mind-imprint/course-contract";
import type { SliceEmitter } from "@mind-imprint/course-runtime";
import {
  HtmlInteractionRenderer,
  createHtmlMessageHandler,
} from "../../src/blocks/html/HtmlInteractionRenderer";
import { PROTOCOL_NAME, PROTOCOL_VERSION } from "../../src/blocks/html/protocol";
import type { InteractiveHtmlBlock } from "../../src/blocks/types";

const assetResolver = { resolve: (p: string) => `/resolved/${p}` };
const baseState: BlockSessionState = { visible: true, enabled: true, completed: false };

const block: InteractiveHtmlBlock = {
  id: "h1",
  type: "interactiveHtml",
  source: "assets/interactions/sort.html",
  protocolVersion: "1.0",
  aspectRatio: "4:3",
  completion: { rule: "interaction-complete" },
};

interface Recorded {
  sourceId: string;
  type: string;
  payload: unknown;
}

function recorder() {
  const events: Recorded[] = [];
  const emit: SliceEmitter = (sourceId, type, payload) => events.push({ sourceId, type: String(type), payload });
  return { events, emit };
}

// A stub Window plays the role of `iframe.contentWindow` for the source check.
const frameWindow = {} as unknown as Window;

function frameMsg(overrides: Record<string, unknown> = {}) {
  return {
    protocol: PROTOCOL_NAME,
    version: PROTOCOL_VERSION,
    sessionToken: "fixed-tok",
    type: "ready",
    ...overrides,
  };
}

function makeHandler(over: Partial<Parameters<typeof createHtmlMessageHandler>[0]> = {}) {
  const { events, emit } = recorder();
  const rejected: string[] = [];
  const handler = createHtmlMessageHandler({
    getExpectedSource: () => frameWindow,
    sessionToken: "fixed-tok",
    block,
    emit,
    onRejected: (reason) => rejected.push(reason),
    ...over,
  });
  return { handler, events, rejected };
}

describe("HtmlInteractionRenderer (component)", () => {
  it("renders a sandboxed iframe (allow-scripts only, NO allow-same-origin) in an aspect-ratio wrapper", () => {
    const { emit } = recorder();
    const { container } = render(
      <HtmlInteractionRenderer
        block={block}
        assetResolver={assetResolver}
        state={baseState}
        visible
        enabled
        emit={emit}
        tokenFactory={() => "fixed-tok"}
      />,
    );
    const iframe = container.querySelector("iframe");
    expect(iframe).not.toBeNull();
    expect(iframe!.getAttribute("sandbox")).toBe("allow-scripts");
    expect(iframe!.getAttribute("sandbox")).not.toContain("allow-same-origin");
    expect(iframe!.getAttribute("src")).toBe("/resolved/assets/interactions/sort.html");

    const wrapper = container.querySelector('[data-block-type="interactiveHtml"]');
    expect(wrapper).not.toBeNull();
    expect(wrapper!.getAttribute("data-aspect-ratio")).toBe("4:3");
  });

  it("marks the block non-interactive when enabled=false (advisory; sandbox already isolates)", () => {
    const { emit } = recorder();
    const { container } = render(
      <HtmlInteractionRenderer
        block={block}
        assetResolver={assetResolver}
        state={{ ...baseState, enabled: false }}
        visible
        enabled={false}
        emit={emit}
        tokenFactory={() => "fixed-tok"}
      />,
    );
    const wrapper = container.querySelector('[data-block-type="interactiveHtml"]');
    expect(wrapper!.getAttribute("aria-disabled")).toBe("true");
  });
});

describe("createHtmlMessageHandler (message boundary)", () => {
  it("emits interaction.ready for a valid ready message from the frame source", () => {
    const { handler, events, rejected } = makeHandler();
    handler(frameMsg({ type: "ready", payload: { step: 1 } }), frameWindow);
    expect(events).toEqual([{ sourceId: "h1", type: "interaction.ready", payload: { step: 1 } }]);
    expect(rejected).toEqual([]);
  });

  it("emits interaction.completed then block.completed for a valid completed message", () => {
    const { handler, events } = makeHandler();
    handler(frameMsg({ type: "completed", payload: { score: 3 } }), frameWindow);
    expect(events.map((e) => e.type)).toEqual(["interaction.completed", "block.completed"]);
    expect(events[0]?.payload).toEqual({ score: 3 });
  });

  it("emits block.completed only once even if completed arrives twice", () => {
    const { handler, events } = makeHandler();
    handler(frameMsg({ type: "completed" }), frameWindow);
    handler(frameMsg({ type: "completed" }), frameWindow);
    expect(events.filter((e) => e.type === "block.completed")).toHaveLength(1);
  });

  it("does NOT emit block.completed when completion rule is absent", () => {
    const noRule: InteractiveHtmlBlock = { ...block, completion: undefined };
    const { handler, events } = makeHandler({ block: noRule });
    handler(frameMsg({ type: "completed" }), frameWindow);
    expect(events.map((e) => e.type)).toEqual(["interaction.completed"]);
  });

  it("drops a wrong-token message: no emit, onRejected('token')", () => {
    const { handler, events, rejected } = makeHandler();
    handler(frameMsg({ type: "completed", sessionToken: "stale" }), frameWindow);
    expect(events).toEqual([]);
    expect(rejected).toEqual(["token"]);
  });

  it("drops a message whose source is not the iframe window: no emit", () => {
    const { handler, events, rejected } = makeHandler();
    const otherWindow = {} as unknown as Window;
    handler(frameMsg({ type: "completed" }), otherWindow);
    expect(events).toEqual([]);
    expect(rejected).toEqual(["source"]);
  });

  it("calls onCompleted after a valid completion", () => {
    let done = false;
    const { handler } = makeHandler({ onCompleted: () => (done = true) });
    handler(frameMsg({ type: "completed" }), frameWindow);
    expect(done).toBe(true);
  });
});

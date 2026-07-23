import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, act } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AskPanel } from "./AskPanel";

// Fakes for the voice stack (AsrStream + MicCapture), matching
// CoachRail.test.tsx's mock shape exactly — same two modules, same fake
// surface (onPartial/onFinal/onError/sendPCM/stop, start()/stop()).
const { FakeAsrStream, FakeMicCapture } = vi.hoisted(() => {
  class FakeAsrStream {
    static instances: FakeAsrStream[] = [];
    partialCb: ((t: string) => void) | null = null;
    finalCb: ((t: string) => void) | null = null;
    errorCb: ((m: string) => void) | null = null;
    stopped = false;
    constructor() {
      FakeAsrStream.instances.push(this);
    }
    onPartial(cb: (t: string) => void) {
      this.partialCb = cb;
    }
    onFinal(cb: (t: string) => void) {
      this.finalCb = cb;
    }
    onError(cb: (m: string) => void) {
      this.errorCb = cb;
    }
    sendPCM() {}
    stop() {
      this.stopped = true;
    }
  }
  class FakeMicCapture {
    static instances: FakeMicCapture[] = [];
    static startBehavior: "resolve" | "reject" = "resolve";
    static rejectMessage = "麦克风权限被拒绝";
    stopped = false;
    constructor() {
      FakeMicCapture.instances.push(this);
    }
    async start(_onPcm: (pcm: Int16Array) => void) {
      if (FakeMicCapture.startBehavior === "reject") {
        throw new Error(FakeMicCapture.rejectMessage);
      }
    }
    stop() {
      this.stopped = true;
    }
  }
  return { FakeAsrStream, FakeMicCapture };
});

vi.mock("../../api/voice", () => ({ AsrStream: FakeAsrStream }));
vi.mock("../../audio/capture", () => ({ MicCapture: FakeMicCapture }));

const props = {
  expanded: true,
  onToggle: vi.fn(),
  branchColor: "#2A3B7A",
  context: "引导",
  chips: ["这张卡要我做什么？", "我该先查哪一步？"],
  messages: [],
  pending: false,
  onSend: vi.fn(),
};

describe("AskPanel", () => {
  it("renders the binding disclosure copy verbatim", () => {
    render(<AskPanel {...props} />);
    expect(screen.getByText("问印记")).toBeInTheDocument();
    expect(screen.getByText("随时打断我，问任何问题")).toBeInTheDocument();
    expect(screen.getByText("正在看：引导")).toBeInTheDocument();
    expect(screen.getByText("你可能想问")).toBeInTheDocument();
    expect(screen.getByPlaceholderText("输入你的问题……")).toBeInTheDocument();
    expect(screen.getByText("按住说话，问老师")).toBeInTheDocument();
  });

  it("sends a chip as a message", async () => {
    const onSend = vi.fn();
    render(<AskPanel {...props} onSend={onSend} />);
    await userEvent.click(screen.getByText("这张卡要我做什么？"));
    expect(onSend).toHaveBeenCalledWith("这张卡要我做什么？");
  });

  it("sends the composer's text", async () => {
    const onSend = vi.fn();
    render(<AskPanel {...props} onSend={onSend} />);
    await userEvent.type(screen.getByPlaceholderText("输入你的问题……"), "这算证据吗？");
    await userEvent.click(screen.getByLabelText("发送"));
    expect(onSend).toHaveBeenCalledWith("这算证据吗？");
  });

  it("collapses to the vertical rail", () => {
    render(<AskPanel {...props} expanded={false} />);
    expect(screen.queryByPlaceholderText("输入你的问题……")).not.toBeInTheDocument();
    expect(screen.getByText("问印记")).toBeInTheDocument();
  });

  // Minor 3 (whole-branch): a message may legitimately be offer-only — a
  // rehydrated open card offer (CoursePlayer.tsx) or a live `card` frame with
  // no reply body — and must not render an empty bordered bubble above the
  // card preview. Mirrors the server's own guard against an empty frame
  // (course_session.go's Text-on-non-empty-Reply).
  it("renders no empty bubble for an offer-only message", () => {
    render(
      <AskPanel
        {...props}
        messages={[
          {
            id: "offer-1",
            role: "assistant",
            text: "",
            offer: { cardInstanceId: "ci1", cardId: "craap", materialId: "m1" },
            offerPhase: "offered",
          },
        ]}
      />,
    );
    expect(screen.queryByTestId("ask-bubble")).not.toBeInTheDocument();
    // The card preview itself still renders.
    expect(screen.getByText("信源辨识卡 CRAAP / CRRAAB")).toBeInTheDocument();
    expect(screen.getByText("接受")).toBeInTheDocument();
  });

  it("still renders the bubble for a message with text", () => {
    render(
      <AskPanel
        {...props}
        messages={[{ id: "m1", role: "assistant", text: "你觉得这句话里，哪一部分是证据？" }]}
      />,
    );
    expect(screen.getByTestId("ask-bubble")).toBeInTheDocument();
    expect(screen.getByText("你觉得这句话里，哪一部分是证据？")).toBeInTheDocument();
  });
});

describe("AskPanel push-to-talk voice (按住说话，问老师)", () => {
  beforeEach(() => {
    FakeAsrStream.instances = [];
    FakeMicCapture.instances = [];
    FakeMicCapture.startBehavior = "resolve";
  });

  it("holds 按住说话 to transcribe into the input, and does not auto-send on release", async () => {
    const onSend = vi.fn();
    render(<AskPanel {...props} onSend={onSend} />);
    const holdBtn = screen.getByText("按住说话，问老师").closest("button")!;

    fireEvent.mouseDown(holdBtn);

    // Capture actually started: a MicCapture + AsrStream were instantiated,
    // not just a handler firing into the void.
    expect(FakeMicCapture.instances).toHaveLength(1);
    expect(FakeAsrStream.instances).toHaveLength(1);

    // A transcript lands in the ask input — visible and editable — never
    // auto-sent (克制/铁律 2: the student confirms).
    const asr = FakeAsrStream.instances[0]!;
    act(() => {
      asr.finalCb?.("这条我不太信");
    });
    const input = screen.getByPlaceholderText("输入你的问题……") as HTMLInputElement;
    expect(input.value).toBe("这条我不太信");

    fireEvent.mouseUp(holdBtn);

    expect(FakeMicCapture.instances[0]!.stopped).toBe(true);
    expect(FakeAsrStream.instances[0]!.stopped).toBe(true);
    expect(onSend).not.toHaveBeenCalled();
  });

  it("releasing off the button (mouse leave) also stops the capture", async () => {
    render(<AskPanel {...props} />);
    const holdBtn = screen.getByText("按住说话，问老师").closest("button")!;

    fireEvent.mouseDown(holdBtn);
    expect(FakeMicCapture.instances).toHaveLength(1);

    fireEvent.mouseLeave(holdBtn);
    expect(FakeMicCapture.instances[0]!.stopped).toBe(true);
    expect(FakeAsrStream.instances[0]!.stopped).toBe(true);
  });

  it("surfaces a mic/ASR error inline instead of a modal", async () => {
    FakeMicCapture.startBehavior = "reject";
    render(<AskPanel {...props} />);
    const holdBtn = screen.getByText("按住说话，问老师").closest("button")!;

    fireEvent.mouseDown(holdBtn);

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent(FakeMicCapture.rejectMessage);
  });
});

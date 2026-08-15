import { describe, expect, it, vi, beforeEach } from "vitest";
import type { OpeningSceneInput, ClosingSceneInput } from "@mind-imprint/course-runtime";
import type { RuntimeSceneResult } from "@mind-imprint/course-contract";

import { apiScene } from "@/api/courseScene";
import { makeApiSceneGenerator } from "@/course/apiSceneGenerator";

vi.mock("@/api/courseScene", () => ({ apiScene: vi.fn() }));
const apiSceneMock = vi.mocked(apiScene);

const openingInput: OpeningSceneInput = {
  which: "opening",
  title: "一条网络信息，该不该信",
  estimatedMinutes: 12,
  objectives: ["学会用 CRAAP 判断信源"],
  learningPreview: ["给一条说法做溯源体检"],
  allowedSignals: ["last_course", "not_permitted_but_present"],
  signalValues: {
    last_course: "你上次完成了《论证的骨架》",
    empty: "",
    unpermitted: "should never be sent",
  },
  fallback: { text: "同学你好，我们开始今天的课。", audioUrl: "https://cdn/opening-fallback.mp3" },
};

const closingInput: ClosingSceneInput = {
  which: "closing",
  preparedSummary: "你走完了 CRAAP 的五个维度。",
  takeaways: ["信源辨识的五个维度"],
  transferApplications: ["下次读新闻先查作者与时间"],
  allowedSignals: [],
  sessionEvidence: {},
  fallback: { text: "这节课就到这里。" },
};

beforeEach(() => {
  apiSceneMock.mockReset();
});

describe("makeApiSceneGenerator", () => {
  it("returns the server RuntimeSceneResult on success", async () => {
    const serverResult: RuntimeSceneResult = {
      text: "同学你好，这节课我们一起做信源辨识。",
      audioUrl: "https://cdn/opening.mp3",
      generatedAt: "2026-08-16T00:00:00Z",
      usedSignalTypes: ["last_course"],
      fallbackUsed: false,
    };
    apiSceneMock.mockResolvedValue({ result: serverResult });

    const gen = makeApiSceneGenerator("a-mid");
    const got = await gen.generate(openingInput);
    expect(got).toEqual(serverResult);
  });

  it("sends only permitted, non-empty signal evidence in the request body", async () => {
    apiSceneMock.mockResolvedValue({
      result: { text: "ok", generatedAt: "t", usedSignalTypes: [], fallbackUsed: false },
    });

    await makeApiSceneGenerator("a-mid").generate(openingInput);

    expect(apiSceneMock).toHaveBeenCalledTimes(1);
    const [slug, body] = apiSceneMock.mock.calls[0]!;
    expect(slug).toBe("a-mid");
    expect(body.which).toBe("opening");
    expect(body.facts.title).toBe("一条网络信息，该不该信");
    // only last_course carries evidence; empty and non-allowed keys are dropped
    expect(body.signalEvidence).toEqual({ last_course: "你上次完成了《论证的骨架》" });
    expect(body.fallback.text).toBe("同学你好，我们开始今天的课。");
  });

  it("maps closing input into preparedSummary + takeaways + transfer", async () => {
    apiSceneMock.mockResolvedValue({
      result: { text: "ok", generatedAt: "t", usedSignalTypes: [], fallbackUsed: false },
    });

    await makeApiSceneGenerator("a-mid").generate(closingInput);

    const [, body] = apiSceneMock.mock.calls[0]!;
    expect(body.which).toBe("closing");
    expect(body.facts.preparedSummary).toBe("你走完了 CRAAP 的五个维度。");
    expect(body.facts.takeaways).toEqual(["信源辨识的五个维度"]);
    expect(body.signalEvidence).toEqual({});
  });

  it("returns the authored fallback (fallbackUsed=true) when the client rejects", async () => {
    apiSceneMock.mockImplementationOnce(async () => { throw new Error("network down"); });

    const got = await makeApiSceneGenerator("a-mid").generate(openingInput);
    expect(got.text).toBe("同学你好，我们开始今天的课。");
    expect(got.audioUrl).toBe("https://cdn/opening-fallback.mp3");
    expect(got.fallbackUsed).toBe(true);
    expect(got.usedSignalTypes).toEqual([]);
    expect(got.generatedAt).not.toBe("");
  });

  it("omits audioUrl in the fallback when none was authored", async () => {
    apiSceneMock.mockImplementationOnce(async () => { throw new Error("network down"); });

    const got = await makeApiSceneGenerator("a-mid").generate(closingInput);
    expect(got.text).toBe("这节课就到这里。");
    expect(got.audioUrl).toBeUndefined();
    expect(got.fallbackUsed).toBe(true);
  });
});

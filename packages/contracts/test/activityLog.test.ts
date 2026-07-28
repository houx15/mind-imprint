import { describe, it, expect } from "vitest";
import { LogEntry, LogSource } from "../src/activityLog";

describe("LogEntry", () => {
  it("parses a valid entry", () => {
    const ok = LogEntry.parse({ id: "l1", date: "07-29", text: "打开了 NASA 报告", source: "auto" });
    expect(ok.source).toBe("auto");
    expect(LogSource.parse("me")).toBe("me");
  });
  it("rejects an unknown source", () => {
    expect(() => LogEntry.parse({ id: "l1", date: "07-29", text: "x", source: "system" })).toThrow();
  });
});

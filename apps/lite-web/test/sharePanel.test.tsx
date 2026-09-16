import { render, screen, cleanup, waitFor, fireEvent } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { SharePanel } from "@lite/reports/SharePanel";

/**
 * SharePanel — the opt-in that lets a student let someone else see her
 * finished report: a link, a QR code, and 停止分享 to take it back.
 * task-10-brief.md's three behaviours, plus the ruling that overrides the
 * brief: the client builds the URL itself from `window.location.origin` and
 * the server-minted `token`, and never trusts the server's own `url` field
 * (which is Origin-header-derived and can point somewhere that 404s behind a
 * proxy that strips that header).
 */

vi.mock("qrcode", () => ({
  default: { toDataURL: vi.fn(async (text: string) => `data:image/png;base64,QR(${text})`) },
}));

const shareReport = vi.fn();
const unshareReport = vi.fn();

vi.mock("@lite/api/reports", () => ({
  shareReport: (...args: unknown[]) => shareReport(...args),
  unshareReport: (...args: unknown[]) => unshareReport(...args),
}));

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

beforeEach(() => {
  vi.stubGlobal("location", { origin: "https://mind-lite.example" });
});

describe("SharePanel", () => {
  it("shows no link and no QR until she turns sharing on", () => {
    render(<SharePanel kind="reading" atomId="atom-1" />);

    expect(screen.getByRole("button", { name: "生成分享链接" })).toBeTruthy();
    expect(screen.queryByRole("img")).toBeNull();
    expect(screen.queryByDisplayValue(/https?:\/\//)).toBeNull();
    expect(screen.queryByText(/https?:\/\//)).toBeNull();
    expect(screen.queryByRole("button", { name: "停止分享" })).toBeNull();
  });

  it("tells her plainly what sharing means before she opts in", () => {
    render(<SharePanel kind="reading" atomId="atom-1" />);

    expect(screen.getByText(/不用登录/)).toBeTruthy();
    expect(screen.getByText(/停止分享/)).toBeTruthy();
  });

  it("shows the link and a QR image once shared, built from the browser's own origin — never the server's url", async () => {
    shareReport.mockResolvedValue({ token: "abc123", url: "https://wrong-host.example/s/abc123" });
    render(<SharePanel kind="reading" atomId="atom-1" />);

    fireEvent.click(screen.getByRole("button", { name: "生成分享链接" }));

    const link = await screen.findByDisplayValue("https://mind-lite.example/s/abc123");
    expect(link).toBeTruthy();
    expect(screen.queryByDisplayValue(/wrong-host/)).toBeNull();
    expect(screen.queryByText(/wrong-host/)).toBeNull();

    const img = screen.getByRole("img", { name: /二维码/ }) as HTMLImageElement;
    expect(img.src).toBe("data:image/png;base64,QR(https://mind-lite.example/s/abc123)");

    // 第三个参数是她勾的那一位。默认不勾 —— 「不选就什么都没多公开」
    // 这件事在这里是能断的。
    expect(shareReport).toHaveBeenCalledWith("reading", "atom-1", { includeTranscript: false });
  });

  it("停止分享 removes the link and the QR", async () => {
    shareReport.mockResolvedValue({ token: "abc123", url: "https://wrong-host.example/s/abc123" });
    unshareReport.mockResolvedValue(undefined);
    render(<SharePanel kind="reading" atomId="atom-1" />);

    fireEvent.click(screen.getByRole("button", { name: "生成分享链接" }));
    await screen.findByDisplayValue("https://mind-lite.example/s/abc123");

    fireEvent.click(screen.getByRole("button", { name: "停止分享" }));

    await waitFor(() => expect(screen.queryByDisplayValue(/mind-lite/)).toBeNull());
    expect(screen.queryByRole("img")).toBeNull();
    expect(screen.getByRole("button", { name: "生成分享链接" })).toBeTruthy();
    expect(unshareReport).toHaveBeenCalledWith("reading", "atom-1");
  });

  it("F2: mounting on an already-shared report shows the link, the QR, and 停止分享 — not the off state", async () => {
    render(<SharePanel kind="reading" atomId="atom-1" initialShareToken="already-shared-token" />);

    // Off-state affordances must be absent immediately — no flash of "off"
    // before the fetch, because there is no fetch: the token is a prop.
    expect(screen.queryByRole("button", { name: "生成分享链接" })).toBeNull();

    const link = screen.getByDisplayValue("https://mind-lite.example/s/already-shared-token");
    expect(link).toBeTruthy();
    expect(screen.getByRole("button", { name: "停止分享" })).toBeTruthy();

    const img = (await screen.findByRole("img", { name: /二维码/ })) as HTMLImageElement;
    expect(img.src).toBe("data:image/png;base64,QR(https://mind-lite.example/s/already-shared-token)");

    // Nothing was minted — the token came from the server's earlier report
    // fetch, not from a fresh POST.
    expect(shareReport).not.toHaveBeenCalled();
  });

  it("maps kind='writing' onto the writing atom id when sharing/unsharing", async () => {
    shareReport.mockResolvedValue({ token: "tok-w", url: "https://irrelevant.example/s/tok-w" });
    unshareReport.mockResolvedValue(undefined);
    render(<SharePanel kind="writing" atomId="atom-9" />);

    fireEvent.click(screen.getByRole("button", { name: "生成分享链接" }));
    await screen.findByDisplayValue("https://mind-lite.example/s/tok-w");
    expect(shareReport).toHaveBeenCalledWith("writing", "atom-9", { includeTranscript: false });

    fireEvent.click(screen.getByRole("button", { name: "停止分享" }));
    await waitFor(() => expect(unshareReport).toHaveBeenCalledWith("writing", "atom-9"));
  });
});

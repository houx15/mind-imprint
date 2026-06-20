import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SettingsPanel } from "./SettingsPanel";
import { loadConfig } from "../llm/config";

beforeEach(() => localStorage.clear());
afterEach(() => { vi.unstubAllGlobals(); });

async function fillCore() {
  await userEvent.clear(screen.getByLabelText("Base URL"));
  await userEvent.type(screen.getByLabelText("Base URL"), "https://api.openai.com/v1");
  await userEvent.clear(screen.getByLabelText("Model"));
  await userEvent.type(screen.getByLabelText("Model"), "gpt-4o-mini");
  await userEvent.clear(screen.getByLabelText("API Key"));
  await userEvent.type(screen.getByLabelText("API Key"), "sk-demo");
}

describe("SettingsPanel", () => {
  it("saves entered config to storage", async () => {
    render(<SettingsPanel />);
    await fillCore();
    await userEvent.click(screen.getByRole("button", { name: "保存" }));
    expect(loadConfig({})).toMatchObject({ format: "openai", baseUrl: "https://api.openai.com/v1", model: "gpt-4o-mini", apiKey: "sk-demo" });
  });
  it("test connection shows the model reply on success", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: true, status: 200, json: () => Promise.resolve({ choices: [{ message: { content: "OK!" } }] }) }));
    render(<SettingsPanel />);
    await fillCore();
    await userEvent.click(screen.getByRole("button", { name: "测试连接" }));
    expect(await screen.findByText(/OK!/)).toBeInTheDocument();
  });
  it("test connection shows the error message (❌) without leaking the key", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: false, status: 401, json: () => Promise.resolve({ error: { message: "invalid api key" } }) }));
    render(<SettingsPanel />);
    await fillCore();
    await userEvent.click(screen.getByRole("button", { name: "测试连接" }));
    expect(await screen.findByText(/invalid api key/)).toBeInTheDocument();
    expect(screen.queryByText(/sk-demo/)).not.toBeInTheDocument();
  });
});

import { describe, it, expect, beforeEach } from "vitest";
import { loadConfig, saveConfig, isConfigured } from "./config";
import type { LlmConfig } from "./types";

beforeEach(() => localStorage.clear());

describe("config", () => {
  it("saveConfig then loadConfig roundtrips via localStorage", () => {
    const cfg: LlmConfig = { format: "openai", baseUrl: "u", model: "m", apiKey: "k" };
    saveConfig(cfg);
    expect(loadConfig({})).toEqual(cfg);
  });
  it("falls back to injected env when storage is empty", () => {
    const cfg = loadConfig({
      VITE_LLM_FORMAT: "anthropic", VITE_LLM_BASE_URL: "b", VITE_LLM_MODEL: "m", VITE_LLM_API_KEY: "k",
    });
    expect(cfg).toMatchObject({ format: "anthropic", baseUrl: "b", model: "m", apiKey: "k" });
  });
  it("isConfigured requires all four core fields", () => {
    expect(isConfigured({ format: "openai", baseUrl: "u", model: "m", apiKey: "k" })).toBe(true);
    expect(isConfigured({ format: "openai", baseUrl: "u", model: "m" })).toBe(false);
  });
  it("falls back to env when stored JSON is corrupt (does not throw)", () => {
    localStorage.setItem("mk.llmConfig", "NOT_JSON{");
    const cfg = loadConfig({ VITE_LLM_FORMAT: "openai", VITE_LLM_BASE_URL: "b", VITE_LLM_MODEL: "m", VITE_LLM_API_KEY: "k" });
    expect(cfg).toMatchObject({ format: "openai", baseUrl: "b", model: "m", apiKey: "k" });
  });
});

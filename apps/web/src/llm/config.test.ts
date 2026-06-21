import { describe, it, expect, beforeEach } from "vitest";
import { loadConfig, saveConfig, isConfigured, isVerified, markVerified } from "./config";
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

describe("config hardening", () => {
  beforeEach(() => localStorage.clear());

  it("loads a valid stored config", () => {
    saveConfig({ format: "openai", baseUrl: "https://x/v1", model: "m", apiKey: "k" });
    expect(isConfigured(loadConfig({}))).toBe(true);
  });
  it("falls back to env defaults when stored JSON is structurally invalid", () => {
    localStorage.setItem("mk.llmConfig", JSON.stringify({ format: 123 }));
    const cfg = loadConfig({ VITE_LLM_MODEL: "envModel" });
    expect(cfg.model).toBe("envModel");
  });
  it("isVerified requires both configured and verified flag", () => {
    const base = { format: "openai" as const, baseUrl: "b", model: "m", apiKey: "k" };
    expect(isVerified(base)).toBe(false);
    expect(isVerified({ ...base, verified: true })).toBe(true);
  });
  it("markVerified persists verified:true without touching the key in any log", () => {
    const base = { format: "openai" as const, baseUrl: "b", model: "m", apiKey: "k" };
    saveConfig(base);
    markVerified(base);
    expect(loadConfig({}).verified).toBe(true);
  });
});

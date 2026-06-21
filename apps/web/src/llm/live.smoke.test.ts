import { describe, it, expect } from "vitest";
import { loadConfig, isConfigured } from "./config";
import { chat } from "./client";

const cfg = loadConfig();
const runnable = isConfigured(cfg);

// Skips automatically unless apps/web/.env.local provides a full VITE_LLM_* config.
describe.skipIf(!runnable)("LLM live smoke (needs apps/web/.env.local)", () => {
  it("returns a non-empty reply from the configured provider", async () => {
    // maxTokens must clear a reasoning model's hidden reasoning budget before
    // it emits any visible token, or content comes back empty (finish=length).
    const r = await chat(cfg, { messages: [{ role: "user", content: "Reply with the single word: OK" }], maxTokens: 256 });
    expect(r.text.trim().length).toBeGreaterThan(0);
  });
});

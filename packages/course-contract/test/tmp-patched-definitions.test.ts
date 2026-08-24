// TEMPORARY (2026-08-23 gate-fix batch): validates every rewritten course
// definition against the real contract before it is PUT to production.
// Delete once the batch has shipped.
import { describe, expect, it } from "vitest";
import { readdirSync, readFileSync } from "node:fs";
import { resolve } from "node:path";
import { validateCourseDefinition } from "../src/validate";

const DIR = resolve(__dirname, "../../../deploy/course-layout-gate-fixes-2026-08-23/definitions");

const files = readdirSync(DIR)
  .filter((f) => f.endsWith(".json") && !f.endsWith(".before.json"))
  .sort();

describe("patched course definitions", () => {
  it("has definitions to check", () => {
    expect(files.length).toBeGreaterThan(0);
  });

  for (const file of files) {
    it(`${file} validates`, () => {
      const doc = JSON.parse(readFileSync(resolve(DIR, file), "utf8"));
      const result = validateCourseDefinition(doc);
      if (!result.ok) {
        throw new Error(
          `${file}\n` + result.issues.map((i) => `  [${i.layer}] ${i.path}: ${i.message}`).join("\n"),
        );
      }
      expect(result.ok).toBe(true);
    });
  }

  // The originals are the control: anything already failing validation before
  // the rewrite must not be blamed on the rewrite.
  for (const file of files) {
    const before = file.replace(/\.json$/, ".before.json");
    it(`${before} (control)`, () => {
      const doc = JSON.parse(readFileSync(resolve(DIR, before), "utf8"));
      const result = validateCourseDefinition(doc);
      if (!result.ok) {
        console.log(
          `PRE-EXISTING issues in ${before}:\n` +
            result.issues.map((i) => `  [${i.layer}] ${i.path}: ${i.message}`).join("\n"),
        );
      }
    });
  }
});

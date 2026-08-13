import { describe, it, expect } from "vitest";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { EvaluationReport } from "../src/evaluationReport";

// Cross-language golden check: `apps/api/internal/evalreport/golden_test.go`
// marshals the REAL Go `Placeholder(...)` output and commits it here. This
// is the only automated check that the actual Go placeholder JSON — not a
// hand-written TS fixture — satisfies the strict Zod `EvaluationReport`
// schema; Go's own `Validate` only guards the outer envelope (id/status
// shape), so nothing else exercised this boundary.
const goldenPath = fileURLToPath(
  new URL("../../../apps/api/internal/evalreport/testdata/placeholder_golden.json", import.meta.url),
);

describe("EvaluationReport golden (Go placeholder -> Zod)", () => {
  it("parses the real Go Placeholder(...) output without throwing", () => {
    const golden = JSON.parse(readFileSync(goldenPath, "utf-8"));
    expect(() => EvaluationReport.parse(golden)).not.toThrow();
  });
});

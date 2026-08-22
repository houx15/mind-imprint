import { describe, it, expect } from "vitest";
import { readFileSync } from "node:fs";
import path from "node:path";
import { EvaluationReport } from "@mind-imprint/contracts";
import { demoEvaluationReport } from "@/tour/fixtures/demoEvaluationReport";

// The guided-tour demo evaluation report (P2 Task 5). Two guarantees:
//   1. the fixture satisfies the strict EvaluationReport contract, and
//   2. the row seeded into migration 0082 is byte-identical to it — so the
//      report the tour walks in prod is exactly the one validated here.

describe("demo evaluation report", () => {
  it("satisfies the strict EvaluationReport contract", () => {
    expect(() => EvaluationReport.parse(demoEvaluationReport)).not.toThrow();
  });

  it("is seeded byte-identically into migration 0082", () => {
    // vitest cwd = apps/web; the migration lives under apps/api.
    const migrationPath = path.resolve(
      process.cwd(),
      "../api/internal/store/migrations/0082_seed_demo_project_finished.sql",
    );
    const sql = readFileSync(migrationPath, "utf8");

    // Extract the report jsonb literal from the evaluation_report INSERT. The
    // JSON is authored WITHOUT any ASCII apostrophe, so the SQL single-quoted
    // literal needs no un-escaping: it runs from the first `'{` after the
    // INSERT to the closing `}'::jsonb`.
    const insertIdx = sql.indexOf("INSERT INTO evaluation_report");
    expect(insertIdx).toBeGreaterThanOrEqual(0);
    const region = sql.slice(insertIdx);
    const match = region.match(/'(\{[\s\S]*\})'::jsonb/);
    const migrationJson = match?.[1];
    if (!migrationJson) throw new Error("could not find the report jsonb literal in migration 0082");

    // Sanity: no apostrophe escaping happened (would corrupt JSON.parse).
    expect(migrationJson.includes("''")).toBe(false);

    // The seeded JSON must itself satisfy the strict contract.
    const parsed = JSON.parse(migrationJson);
    expect(() => EvaluationReport.parse(parsed)).not.toThrow();

    // And it must be exactly the fixture serialized — this is the drift guard.
    expect(migrationJson).toBe(JSON.stringify(demoEvaluationReport));
  });
});

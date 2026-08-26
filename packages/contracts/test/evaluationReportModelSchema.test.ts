import { readFileSync, writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import path from "node:path";
import { describe, expect, it } from "vitest";
import type { ZodTypeAny } from "zod";
import { zodToJsonSchema } from "zod-to-json-schema";
import {
  EvaluationReport,
  EvaluationReportModelOutput,
} from "../src/evaluationReport";

const here = path.dirname(fileURLToPath(import.meta.url));
const schemaPath = path.resolve(here, "../../../apps/api/internal/evalbench/prompts/schemas/evaluation_report_model_output_v1.schema.json");
const reportSchemaPath = path.resolve(here, "../../../apps/api/internal/evalbench/prompts/schemas/evaluation_report_v1.schema.json");
const examplePath = path.resolve(here, "../../../apps/api/internal/evalbench/prompts/schemas/evaluation_report_model_output_v1.example.json");
const schemaTargets = [
  ["EvaluationReport", EvaluationReport, reportSchemaPath],
  ["EvaluationReportModelOutput", EvaluationReportModelOutput, schemaPath],
] as const;

function generatedSchema(name: string, contract: ZodTypeAny) {
  return JSON.stringify(zodToJsonSchema(contract, {
    name,
    target: "jsonSchema7",
    $refStrategy: "root",
  }), null, 2) + "\n";
}

describe("EvaluationReport JSON Schemas", () => {
  it("matches every checked-in schema used by Go", () => {
    for (const [name, contract, target] of schemaTargets) {
      const generated = generatedSchema(name, contract);
      if (process.env.UPDATE_EVALREPORT_MODEL_SCHEMA === "1") {
        writeFileSync(target, generated);
      }
      expect(readFileSync(target, "utf8")).toBe(generated);
    }
  });

  it("accepts the complete example embedded by evalbench", () => {
    const example = JSON.parse(readFileSync(examplePath, "utf8"));
    expect(EvaluationReportModelOutput.safeParse(example).success).toBe(true);
  });

  it("rejects missing, null, and unknown model-output fields", () => {
    const example = JSON.parse(readFileSync(examplePath, "utf8"));
    const missing = JSON.parse(JSON.stringify(example));
    delete missing.abstract.overview;
    const nullable = JSON.parse(JSON.stringify(example));
    nullable.abstract.suggestionSentences = null;
    const unknown = JSON.parse(JSON.stringify(example));
    unknown.unexpected = true;

    for (const invalid of [missing, nullable, unknown]) {
      expect(EvaluationReportModelOutput.safeParse(invalid).success).toBe(false);
    }
  });
});

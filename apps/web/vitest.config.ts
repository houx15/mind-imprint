import path from "node:path";
import { fileURLToPath } from "node:url";
import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";

const __dirname = fileURLToPath(new URL(".", import.meta.url));

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "src"),
    },
  },
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: ["./src/test/setup.ts"],
    include: [
      "test/**/*.test.{ts,tsx}",
      // EvaluationReport + AssessmentView (2026-08-13 pipeline) intentionally
      // colocate their tests next to the components instead of the repo's
      // usual test/ mirror tree — see task briefs under
      // .superpowers/sdd/2026-08-13-evaluation-report-pipeline/.
      "src/shell/report/EvaluationReport/**/*.test.{ts,tsx}",
      "src/shell/report/print/**/*.test.{ts,tsx}",
      "src/shell/assessment/**/*.test.{ts,tsx}",
      // selectHomeCourses (Task 6, 2026-08-19 course-catalog-taxonomy-and-intro)
      // colocates its test next to the helper, same convention as above.
      "src/shell/home/**/*.test.{ts,tsx}",
    ],
  },
});

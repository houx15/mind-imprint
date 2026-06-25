// Dev-only: regenerates the Go parity fixtures from the canonical TS sources.
//
// Provenance: these fixtures are the *authoritative* TS output. The Go ports in
// apps/api/internal/agent (BuildSystemPrompt, SerializeCardForRefeed) are tested
// for byte/semantic equality against them. Regenerate after any change to the
// TS prompt template, the card registry, or refeed logic:
//
//   pnpm --filter @mind-imprint/web exec tsx scripts/gen-go-fixtures.ts
//
// The Go catalog is sorted by id (cards.Catalog()); we sort the TS catalog by id
// here too so the category grouping order matches the Go builder exactly.
import { writeFileSync, mkdirSync } from "node:fs";
import { resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { buildSystemPrompt } from "../src/agent/prompt";
import { buildEvalPrompt } from "../src/agent/evalPrompt";
import { assembleEvalInput } from "../src/agent/evalInput";
import { serializeCardForRefeed, CARD_REGISTRY, deriveCatalog, FULL_RUBRIC } from "@mind-imprint/contracts";

const here = dirname(fileURLToPath(import.meta.url));
const outDir = resolve(here, "../../api/internal/agent/testdata");
mkdirSync(outDir, { recursive: true });

// Build the catalog from the registry, sorted by id to match Go's id-sorted
// cards.Catalog().
const catalog = deriveCatalog(CARD_REGISTRY).sort((a, b) =>
  a.id < b.id ? -1 : a.id > b.id ? 1 : 0,
);

// 1) System prompt fixture (no trailing newline; Go reads the file verbatim).
writeFileSync(resolve(outDir, "system_prompt_full.txt"), buildSystemPrompt(catalog));

// 2) Refeed fixtures from sift_craap.
const sift = CARD_REGISTRY["sift_craap"]!;

const base = {
  id: "ci_1",
  card_id: "sift_craap",
  task_id: "t_1",
  parent_node_id: null,
  event_trace: [] as unknown[],
  rubric_tags: [] as string[],
  created_at: "2026-06-21T10:00:00.000Z",
  completed_at: null,
};

const completed = serializeCardForRefeed(sift, {
  ...base,
  status: "completed",
  field_values: {
    sift: {
      stop: "证明中国让地球更可持续",
      sources: [
        { name: "NASA", type: "官方", verdict: "可信" },
        { name: "Nature Sustainability", type: "学者/机构", verdict: "可信" },
      ],
      better: "原始研究来自 NASA / Nature Sustainability",
      trace: "https://www.nature.com/...",
    },
  },
} as never);

const skipped = serializeCardForRefeed(sift, {
  ...base,
  status: "skipped",
  field_values: {},
} as never);

writeFileSync(
  resolve(outDir, "refeed_sift_completed.json"),
  JSON.stringify(completed, null, 2) + "\n",
);
writeFileSync(
  resolve(outDir, "refeed_sift_skipped.json"),
  JSON.stringify(skipped, null, 2) + "\n",
);

// 3) Eval prompt fixture (verbatim flagship evaluator prompt).
writeFileSync(resolve(outDir, "eval_prompt_full.txt"), buildEvalPrompt(FULL_RUBRIC));

// 4) Eval input fixture from a small fixed transcript + one completed card.
const evalMessages = [
  { id: "m1", task_id: "t_1", role: "user", content: "我想引用这篇公众号文章", created_at: base.created_at },
  { id: "m2", task_id: "t_1", role: "assistant", content: "先一起核查来源吧", tool_call: { id: "tc1", name: "summon_card", args: { card_id: "sift_craap", reason: "r", nudge_text: "n" }, card_instance_id: "ci_1" }, created_at: base.created_at },
] as never;
const evalCards = [{ ...base, status: "completed", field_values: { sift: { stop: "证明中国让地球更可持续" } } }] as never;
const evalInput = assembleEvalInput({ messages: evalMessages, cards: evalCards, registry: CARD_REGISTRY });
writeFileSync(resolve(outDir, "eval_input.txt"), evalInput);

// eslint-disable-next-line no-console
console.log("wrote 5 fixtures to", outDir);

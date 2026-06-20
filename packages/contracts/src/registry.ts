import { CardSpec } from "./cardSpec";
import siftCraap from "../cards/sift_craap.json";
import concession from "../cards/concession.json";

const DEFAULT_RAW: Record<string, unknown> = { sift_craap: siftCraap, concession };

export type CatalogEntry = { id: string; category: string; name: string; trigger_condition: string };
export type Catalog = CatalogEntry[];

export function loadRegistry(raw: Record<string, unknown> = DEFAULT_RAW): Record<string, CardSpec> {
  const out: Record<string, CardSpec> = {};
  for (const [key, data] of Object.entries(raw)) {
    const parsed = CardSpec.safeParse(data);
    if (!parsed.success) {
      const detail = parsed.error.issues.map((i) => `${i.path.join(".")}: ${i.message}`).join("; ");
      throw new Error(`Invalid card "${key}": ${detail}`);
    }
    if (parsed.data.id !== key) {
      throw new Error(`Card key "${key}" does not match card.id "${parsed.data.id}"`);
    }
    out[key] = parsed.data;
  }
  return out;
}

export function deriveCatalog(registry: Record<string, CardSpec>): Catalog {
  return Object.values(registry).map((c) => ({
    id: c.id, category: c.category, name: c.name, trigger_condition: c.trigger_condition,
  }));
}

export const CARD_REGISTRY = loadRegistry();

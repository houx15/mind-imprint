import { z } from "zod";
import {
  WorkspaceProjection,
  Proposal,
  PlanItem,
  PlanTag,
  PlanColumn,
  LogEntry,
  Collection,
  Reference,
  MaterialSource,
  OutlineNode,
  ReflectionDoc,
  Mirror,
} from "@mind-imprint/contracts";
import { apiFetch, ApiError } from "../../api/client";

// The lean GET /projects/{id} projection the four-room shell needs — identity
// + the four proposal dims. Each room fetches its own richer data (slices 2–5).
// Parses through the contract so any wire drift fails loud here, not deep in a
// block.
export async function getWorkspace(id: string): Promise<WorkspaceProjection> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${id}`);
  return WorkspaceProjection.parse(raw);
}

// ---- Project Management room (slice 2) -----------------------------------
// Every fn here parses its payload through the shared contract so wire drift
// surfaces at the boundary, mirroring getWorkspace above.

// PUT /proposal — upsert the four kick-off dimensions; returns the stored row.
export async function putProposal(id: string, proposal: Proposal): Promise<Proposal> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${id}/proposal`, {
    method: "PUT",
    body: JSON.stringify(proposal),
  });
  return Proposal.parse((raw as { proposal: unknown }).proposal);
}

// GET /plan — the whole board (ordered by stage, position, start).
export async function getPlan(id: string): Promise<PlanItem[]> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${id}/plan`);
  return z.array(PlanItem).parse((raw as { items: unknown }).items);
}

// The shape the create endpoint accepts — position/id are server-assigned.
export type NewPlanItem = {
  title: string;
  tag: PlanTag;
  column: PlanColumn;
  stage: string;
  refMaterialId?: string;
  start: number;
  days: number;
};

// POST /plan/items — append a task; returns the created row.
export async function createPlanItem(id: string, body: NewPlanItem): Promise<PlanItem> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${id}/plan/items`, {
    method: "POST",
    body: JSON.stringify(body),
  });
  return PlanItem.parse((raw as { item: unknown }).item);
}

// PATCH /plan/items/{iid} — partial edit (move column, reschedule, resize, …).
export type PlanItemPatch = Partial<Omit<PlanItem, "id">>;
export async function patchPlanItem(id: string, iid: string, patch: PlanItemPatch): Promise<PlanItem> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${id}/plan/items/${iid}`, {
    method: "PATCH",
    body: JSON.stringify(patch),
  });
  return PlanItem.parse((raw as { item: unknown }).item);
}

// DELETE /plan/items/{iid} — 204, no body.
export async function deletePlanItem(id: string, iid: string): Promise<void> {
  await apiFetch<void>(`/api/v1/projects/${id}/plan/items/${iid}`, { method: "DELETE" });
}

// GET /log — the activity log, newest-last, date = "MM-DD".
export async function getLog(id: string): Promise<LogEntry[]> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${id}/log`);
  return z.array(LogEntry).parse((raw as { entries: unknown }).entries);
}

// POST /log — the student's own note (source="me", entry_date=today).
export async function addLog(id: string, text: string): Promise<LogEntry> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${id}/log`, {
    method: "POST",
    body: JSON.stringify({ text }),
  });
  return LogEntry.parse((raw as { entry: unknown }).entry);
}

// POST /coach — one restrained coaching turn (JSON, not SSE). The only spend
// endpoint of the room; returns the AI reply as a plain string.
export type CoachScope = "forming" | "find_sources" | "writing" | "proposal_review";
export async function coach(id: string, scope: CoachScope, userInput: string): Promise<string> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${id}/coach`, {
    method: "POST",
    body: JSON.stringify({ scope, user_input: userInput }),
  });
  return z.object({ reply: z.string() }).parse(raw).reply;
}

// POST /plan/generate — 印记 turns the kickoff into a first project plan. Spend
// endpoint; returns the freshly created (persisted) plan items. Throws ApiError
// with code "proposal_empty" when there's nothing to generate from yet.
export async function generatePlan(id: string): Promise<PlanItem[]> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${id}/plan/generate`, { method: "POST" });
  return z.object({ items: z.array(PlanItem) }).parse(raw).items;
}

// ---- Read room / Library (slice 3) ---------------------------------------
// The Zotero-shaped library: collections + references, all persisted. Every fn
// parses through the shared contract so wire drift surfaces at the boundary.

// GET /library — the whole library for the project. references carry projected
// notes[] (from submitted reading cards) + materialId.
export async function getLibrary(id: string): Promise<{ collections: Collection[]; references: Reference[] }> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${id}/library`);
  const obj = raw as { collections: unknown; references: unknown };
  return {
    collections: z.array(Collection).parse(obj.collections),
    references: z.array(Reference).parse(obj.references),
  };
}

// POST /collections — create a folder (parentId nests it under another).
export type NewCollection = { name: string; parentId?: string | null };
export async function createCollection(id: string, body: NewCollection): Promise<Collection> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${id}/collections`, {
    method: "POST",
    body: JSON.stringify(body),
  });
  return Collection.parse((raw as { collection: unknown }).collection);
}

// PATCH /collections/{cid} — rename / re-nest / reorder.
export type CollectionPatch = { name?: string; parentId?: string | null; position?: number };
export async function patchCollection(id: string, cid: string, patch: CollectionPatch): Promise<Collection> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${id}/collections/${cid}`, {
    method: "PATCH",
    body: JSON.stringify(patch),
  });
  return Collection.parse((raw as { collection: unknown }).collection);
}

// DELETE /collections/{cid} — 204; its references' collection_id → null via FK.
export async function deleteCollection(id: string, cid: string): Promise<void> {
  await apiFetch<void>(`/api/v1/projects/${id}/collections/${cid}`, { method: "DELETE" });
}

// POST /references — register a blank-metadata source (or a pending placeholder
// with search hints). Server assigns id / defaults.
export type NewReference = {
  title?: string;
  url?: string;
  classification?: string;
  collectionId?: string | null;
  pending?: boolean;
  searchHints?: string[];
};
export async function createReference(id: string, body: NewReference): Promise<Reference> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${id}/references`, {
    method: "POST",
    body: JSON.stringify(body),
  });
  return Reference.parse((raw as { reference: unknown }).reference);
}

// PATCH /references/{rid} — partial edit of any editable metadata field. notes/
// materialId are server-projected and never patched from the client.
export type ReferencePatch = Partial<Omit<Reference, "id" | "notes" | "materialId">>;
export async function patchReference(id: string, rid: string, patch: ReferencePatch): Promise<Reference> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${id}/references/${rid}`, {
    method: "PATCH",
    body: JSON.stringify(patch),
  });
  return Reference.parse((raw as { reference: unknown }).reference);
}

// DELETE /references/{rid} — 204, no body.
export async function deleteReference(id: string, rid: string): Promise<void> {
  await apiFetch<void>(`/api/v1/projects/${id}/references/${rid}`, { method: "DELETE" });
}

// ---- Write room (slice 4) ------------------------------------------------
// Outline + draft, both persisted. The draft's autosave reuses putBuffer from
// api/writing (not re-implemented here). Outline is a whole-set replace: PUT
// sends the flat {text,depth} array in order and the server re-assigns
// position/ids, so the client re-reads the fresh nodes it returns.

// GET /outline — the outline bullets, ordered by position.
export async function getOutline(id: string): Promise<OutlineNode[]> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${id}/outline`);
  return z.array(OutlineNode).parse((raw as { nodes: unknown }).nodes);
}

// The shape PUT /outline accepts — id optional (server mints/keeps), position
// is implied by array index.
export type OutlineNodeInput = { id?: string; text: string; depth: number };

// PUT /outline — replace the whole set; returns the fresh nodes (with ids +
// position assigned by the server).
export async function putOutline(id: string, nodes: OutlineNodeInput[]): Promise<OutlineNode[]> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${id}/outline`, {
    method: "PUT",
    body: JSON.stringify({ nodes }),
  });
  return z.array(OutlineNode).parse((raw as { nodes: unknown }).nodes);
}

// GET /draft — the current edit_buffer content ("" when there's no row yet).
// Autosave goes through putBuffer (api/writing), not a fn here.
export async function getDraft(id: string): Promise<string> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${id}/draft`);
  return z.object({ content: z.string() }).parse(raw).content;
}

// Thrown when enter-reading gets a 422 — the source has no readable content
// (no url, no material). The caller shows a gentle inline nudge, not a crash.
export class NoReadableContentError extends Error {
  constructor(message: string, public readonly code: string = "no_content") {
    super(message);
    this.name = "NoReadableContentError";
  }
}

// ---- Review room (slice 5) -----------------------------------------------
// The student's own 5-dimension reflection + the AI-composed "你的思维印记"
// mirror. Both persisted. Finish reuses the project-terminal client in
// api/projects (not re-added here). Every fn parses through the shared contract.

// GET /reflection-doc — the stored answers + done flag ("" answers when no row).
export async function getReflection(id: string): Promise<ReflectionDoc> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${id}/reflection-doc`);
  return ReflectionDoc.parse(raw);
}

// PUT /reflection-doc — upsert the answers (and optionally flip done). Returns
// the stored row.
export async function putReflection(
  id: string,
  body: { answers: string[]; done?: boolean },
): Promise<ReflectionDoc> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${id}/reflection-doc`, {
    method: "PUT",
    body: JSON.stringify(body),
  });
  return ReflectionDoc.parse(raw);
}

// GET /mirror — the stored mirror, or null when it hasn't been composed yet.
// Never triggers an LLM call (that's POST /mirror's job, first-open-wins).
export async function getMirror(id: string): Promise<Mirror | null> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${id}/mirror`);
  if (raw === null) return null;
  return Mirror.parse(raw);
}

// POST /mirror — first-open-wins: returns the stored mirror if present (no
// spend), else composes once (flagship) + stores. The one spend endpoint here.
export async function postMirror(id: string): Promise<Mirror> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${id}/mirror`, { method: "POST" });
  return Mirror.parse(raw);
}

// POST /references/{rid}/enter-reading — ensure a readable material (fetch from
// url on first entry) and return its full MaterialSource DTO. 422 → the typed
// NoReadableContentError so the library can nudge instead of navigating.
export async function enterReading(id: string, rid: string): Promise<MaterialSource> {
  try {
    const raw = await apiFetch<unknown>(`/api/v1/projects/${id}/references/${rid}/enter-reading`, {
      method: "POST",
    });
    return MaterialSource.parse(raw);
  } catch (e) {
    // 422 (no url / no material) OR any fetch failure → offer the paste box.
    if (e instanceof ApiError && (e.status === 422 || e.code === "fetch_failed")) {
      throw new NoReadableContentError(
        e.message || "取不到这个链接的正文，可以直接把正文粘进来。",
        e.code || "no_content",
      );
    }
    throw e;
  }
}

// POST /references/{rid}/paste-content — the fallback when a link can't be
// fetched: the student pastes the article body, which becomes the material's
// blocks. Returns the full MaterialSource so the caller can open the Reading
// Room immediately.
export async function pasteContent(id: string, rid: string, text: string, title?: string): Promise<MaterialSource> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${id}/references/${rid}/paste-content`, {
    method: "POST",
    body: JSON.stringify({ text, title }),
  });
  return MaterialSource.parse(raw);
}

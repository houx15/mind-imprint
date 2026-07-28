import { z } from "zod";
import {
  WorkspaceProjection,
  Proposal,
  PlanItem,
  PlanTag,
  PlanColumn,
  LogEntry,
} from "@mind-imprint/contracts";
import { apiFetch } from "../../api/client";

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
export type CoachScope = "forming" | "find_sources" | "writing";
export async function coach(id: string, scope: CoachScope, userInput: string): Promise<string> {
  const raw = await apiFetch<unknown>(`/api/v1/projects/${id}/coach`, {
    method: "POST",
    body: JSON.stringify({ scope, user_input: userInput }),
  });
  return z.object({ reply: z.string() }).parse(raw).reply;
}

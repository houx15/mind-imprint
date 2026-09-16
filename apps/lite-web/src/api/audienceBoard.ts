import { apiFetch } from "./client";
import type { Persona } from "./personas";

/** Student-authored assumptions about audiences; not verified interview evidence. */
export interface AudienceBoard {
  id: string;
  role: string;
  person: string;
  ageRange: string;
  hobbies?: string[];
  interests: string[];
  offerings: string[];
  keywordDrafts?: Partial<Record<"hobbies" | "interests" | "offerings", { text: string; editing: string | null }>>;
}
export type AudienceStep = "roles" | "person" | "age" | "interests" | "offerings" | "summary";
export interface AudienceDocument {
  boards: AudienceBoard[];
  archivedBoards?: AudienceBoard[];
  activeBoardId: string;
  step: AudienceStep;
  summary?: AudienceSummary;
  keywords?: Record<string, string[]>;
}
export interface AudienceDraft { document: AudienceDocument; revision: number }

export function toggleAudienceRole(doc: AudienceDocument, role: string, makeId: () => string): AudienceDocument {
  const found = doc.boards.find(board => board.role === role);
  const archived = doc.archivedBoards ?? [];
  if (!found && doc.boards.length >= 12) return doc;
  // Keep every saved board; refuse a deselection if the archive is full.
  if (found && archived.length >= 12) return doc;
  const restored = archived.find(board => board.role === role);
  const boards = found ? doc.boards.filter(board => board.id !== found.id) : [...doc.boards, restored ?? { id: makeId(), role, person: "", ageRange: "", interests: [], offerings: [] }];
  return { ...doc, boards, archivedBoards: found ? [...archived, found] : archived.filter(board => board.role !== role), summary: undefined, keywords: undefined, activeBoardId: boards.some(board => board.id === doc.activeBoardId) ? doc.activeBoardId : boards[0]?.id ?? "" };
}
export interface AudienceKeyword { text: string; sourceField: "interests" | "offerings"; sourceIndex: number }
export interface AudienceSummary { boards: { boardId: string; keywords: AudienceKeyword[] }[] }
const base = (projectId: string) => `/api/v1/pbl/projects/${projectId}/audience-board`;
export const getAudienceDraft = (projectId: string) => apiFetch<AudienceDraft>(base(projectId));
export const summarizeAudienceDraft = (projectId: string, revision: number) =>
  apiFetch<{ revision: number; summary: AudienceSummary }>(`${base(projectId)}/summary`, { method: "POST", body: JSON.stringify({ revision }) });
export const saveAudienceDraft = (projectId: string, draft: AudienceDraft) =>
  apiFetch<AudienceDraft>(base(projectId), { method: "PUT", body: JSON.stringify(draft) });
export const confirmAudienceDraft = (projectId: string, revision: number, keywords: Record<string, string[]>) =>
  apiFetch<{ revision: number; personas: Persona[] }>(`${base(projectId)}/confirm`, {
    method: "POST", body: JSON.stringify({ revision, keywords }),
  });

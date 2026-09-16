import { apiFetch } from "./client";
export interface HeroBrief { mode: "" | "image" | "code" | "mixed"; scene: string; action: string; prompt: string }
export interface HeroTrial { versionId: string; observation: string }
export interface HeroResponseDraft { redrawImage: boolean; feedback: string; observation: string; response: "revise" | "retain" | "" }
export interface CreativeDirection { responseDrafts?: Record<string, HeroResponseDraft>; pendingMotif?: string; includeProcess?: boolean; includeComparison?: boolean; trial?: HeroTrial; hero?: HeroBrief; stage: "feeling" | "motifs" | "hero"; feeling: string; motifs: string[]; suggestions: string[] }
export interface CreativeDraft { document: CreativeDirection; revision: number }
const base = (id: string) => `/api/v1/pbl/projects/${id}/creative-direction`;
export const getCreativeDirection = (id: string) => apiFetch<CreativeDraft>(base(id));
export const saveCreativeDirection = (id: string, draft: CreativeDraft) => apiFetch<CreativeDraft>(base(id), { method: "PUT", body: JSON.stringify(draft) });
export const suggestCreativeMotifs = (id: string, revision: number) => apiFetch<CreativeDraft>(`${base(id)}/motifs`, { method: "POST", body: JSON.stringify({ revision }) });

export const refineHeroPrompt = (id: string, revision: number) => apiFetch<CreativeDraft>(`${base(id)}/hero-prompt`, { method: "POST", body: JSON.stringify({ revision }) });

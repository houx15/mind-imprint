import { apiFetch } from "./client";
import type { ShowcaseConfig, ShowcaseWork } from "../site/showcaseTypes";

export interface ShowcaseState {
  draft: ShowcaseConfig;
  revision: number;
  published: boolean;
  hasUnpublishedChanges: boolean;
  url: string;
  availableWorks: ShowcaseWork[];
  hasLegacySite: boolean;
  heroImageUrl?: string;
  avatarUrl?: string;
}
export const getShowcase = () => apiFetch<ShowcaseState>("/api/v1/pbl/showcase");
export const saveShowcase = (draft: ShowcaseConfig, expectedRevision: number) =>
  apiFetch<ShowcaseState>("/api/v1/pbl/showcase", { method: "PUT", body: JSON.stringify({ draft, expectedRevision }) });
export const publishShowcase = (expectedRevision: number) =>
  apiFetch<ShowcaseState>("/api/v1/pbl/showcase/publish", { method: "POST", body: JSON.stringify({ expectedRevision }) });
export const revokeShowcase = () => apiFetch<ShowcaseState>("/api/v1/pbl/showcase/publish", { method: "DELETE" });

export const generateShowcaseImage = (prompt: string, purpose: "hero" | "avatar") =>
  apiFetch<{objectKey: string; url: string}>("/api/v1/pbl/showcase/images/generate", {
    method: "POST", body: JSON.stringify({prompt, purpose}),
  });

import type { ShowcaseInterestSnapshot } from "../site/ShowcaseInterestTree";
import { apiFetch } from "./client";
import type { ShowcaseConfig, ShowcaseWork, ShowcaseAboutMessage } from "../site/showcaseTypes";

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
  availableInterestTree?: ShowcaseInterestSnapshot;
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

export async function resolveShowcaseImage(objectKey: string): Promise<string> {
  const result = await apiFetch<{url:string}>("/api/v1/pbl/showcase/images/resolve", {method:"POST",body:JSON.stringify({objectKey})});
  return result.url;
}

export interface ShowcaseAboutProposal {
  name: string;
  bio: string;
  interests: string[];
  aboutLayout: "classic" | "orbit";
  reason: string;
}
export const chatShowcaseAbout = (messages: ShowcaseAboutMessage[], profile: Pick<ShowcaseConfig,"name"|"bio"|"interests"|"aboutLayout">) =>
  apiFetch<{reply:string;proposal?:ShowcaseAboutProposal}>("/api/v1/pbl/showcase/about/chat", {method:"POST",body:JSON.stringify({messages,profile})});

export async function uploadShowcaseImage(file: File): Promise<{objectKey:string;url:string}> {
  const body = new FormData(); body.append("file",file);
  return apiFetch("/api/v1/pbl/showcase/images/upload", {method:"POST",body});
}

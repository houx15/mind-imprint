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
  aboutChatAvailable?: boolean;
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

export type ShowcaseGuideStage = "design" | "hero" | "profile" | "works" | "components" | "finish";
export type ShowcaseGuideProposal = Partial<Pick<ShowcaseConfig,"style"|"layout"|"palette"|"font"|"heroTitle"|"tagline"|"heroImagePrompt"|"name"|"bio"|"interests"|"aboutLayout"|"interestTreeMode"|"portfolioLayout"|"writingStyle"|"readingStyle"|"homeWorkLimit">> & {reason:string};
export const chatShowcaseGuide = (stage: ShowcaseGuideStage, messages: ShowcaseAboutMessage[], context: ShowcaseConfig) => {
  const {name,bio,tagline,heroTitle,interests,style,layout,palette,font,aboutLayout,interestTreeMode,portfolioLayout,writingStyle,readingStyle,homeWorkLimit} = context;
  return apiFetch<{reply:string;proposal?:ShowcaseGuideProposal}>("/api/v1/pbl/showcase/guide/chat", {method:"POST",body:JSON.stringify({stage,messages:messages.slice(-20),context:{name,bio,tagline,heroTitle,interests,style,layout,palette,font,aboutLayout,interestTreeMode,portfolioLayout,writingStyle,readingStyle,homeWorkLimit}})});
};

export interface GeneratedShowcaseComponent {title:string;source:string;height:number;placement:"after-about"|"after-works";explanation:string}
export const generateShowcaseComponent = (prompt:string, format:"svg"|"html", style:ShowcaseConfig["style"], palette:ShowcaseConfig["palette"]) =>
  apiFetch<GeneratedShowcaseComponent>("/api/v1/pbl/showcase/components/generate",{method:"POST",body:JSON.stringify({prompt,format,style:style??"classic",palette})});

export async function uploadShowcaseImage(file: File): Promise<{objectKey:string;url:string}> {
  const body = new FormData(); body.append("file",file);
  return apiFetch("/api/v1/pbl/showcase/images/upload", {method:"POST",body});
}

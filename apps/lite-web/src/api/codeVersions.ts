import { API_BASE, apiFetch } from "./client";
export interface CodeVersion { comparison?: {feedback:string;observation:string}|null; mode?: "image" | "code" | "mixed"; publicationMissing?: string[]; id: string; brief_revision: number; created_at: string; parent_version_id?: string|null; feedback?: string }
const base=(id:string)=>`/api/v1/pbl/projects/${id}/code-versions`;
export const listCodeVersions=(id:string)=>apiFetch<CodeVersion[]>(base(id));
export const generateHeroCode=(id:string,revision:number,baseVersion?:string,feedback?:string,includeContent=false,redrawImage=false)=>apiFetch<CodeVersion>(base(id),{method:"POST",body:JSON.stringify({revision,baseVersion,feedback,includeContent,redrawImage})});
export const codePreviewURL=(id:string,version:string)=>`${API_BASE}${base(id)}/${version}/preview`;

import { apiFetch } from './client';
import type { Note, NoteKind } from './notes';
export interface ObservationRow { kind: NoteKind; body: string; imageKey: string; from?: string }
export interface ObservationDraft { document: ObservationRow[]; revision: number }
const base=(project:string,tool:string)=>`/api/v1/pbl/projects/${project}/tools/${tool}/observation-draft`;
export const getObservationDraft=(project:string,tool:string)=>apiFetch<ObservationDraft>(base(project,tool));
export const saveObservationDraft=(project:string,tool:string,draft:ObservationDraft)=>apiFetch<ObservationDraft>(base(project,tool),{method:'PUT',body:JSON.stringify(draft)});
export const submitObservationDraft=(project:string,tool:string,revision:number)=>apiFetch<{draft:ObservationDraft;notes:Note[]}>(`${base(project,tool)}/submit`,{method:'POST',body:JSON.stringify({revision})});

import { apiFetch } from "./client";

/**
 * api/personas.ts —— 主页项目第一关：这一页给谁看。
 *
 * 形状照着 apps/api/internal/api/pbl_personas.go 的 pblPersonaDTO 抄，不是猜的。
 */

export interface Persona {
  id: string;
  /** 一个短称呼。他是一类读者，不是一个具体的人。 */
  label: string;
  /** 他为什么会知道她、怎么点进这一页。 */
  whyKnows: string;
  /** 他想看到什么。 */
  wants: string;
  /** 这一页该给他什么感觉。第三关的配色从这里派生。 */
  feeling: string;
  keywords: string[];
  /** 生成的画像。空 = 还没画。 */
  portraitUrl: string;
  chosen: boolean;
}

const base = (id: string) => `/api/v1/pbl/projects/${id}/personas`;

export function listPersonas(projectId: string): Promise<Persona[]> {
  return apiFetch<Persona[]>(base(projectId));
}

/** 印记先动：从她真做过的事里推出两三个可能的读者。整批重来也走这里。 */
export function generatePersonas(projectId: string): Promise<Persona[]> {
  return apiFetch<Persona[]>(`${base(projectId)}/generate`, { method: "POST" });
}

/**
 * 给某一个候选画一张画像。
 *
 * 🚨 一张一个请求。实测一张 69 秒，三张连着画就是三分多钟的一个请求——她那边
 * 看到的会是一个转了三分钟然后超时的圈。
 */
export function drawPersonaPortrait(projectId: string, personaId: string): Promise<Persona> {
  return apiFetch<Persona>(`${base(projectId)}/${personaId}/portrait`, { method: "POST" });
}

/** 她留下的那一个，和她留下的那些关键词。同一次判断，同一个请求。 */
export function choosePersona(
  projectId: string,
  personaId: string,
  keywords: string[],
): Promise<Persona> {
  return apiFetch<Persona>(`${base(projectId)}/${personaId}/choose`, {
    method: "POST",
    body: JSON.stringify({ keywords }),
  });
}

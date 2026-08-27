import { apiFetch } from "./client";
import type { AccentId } from "../ui/accent";
import type { BackgroundId } from "../ui/background";

export interface MeUser {
  id: string;
  email: string;
  display_name: string;
  role: string;
  avatar_color: string;
  page_background: string;
  onboarded_at: string | null;
  // `edition` is which product the school bought ('pro' | 'lite'), and it is
  // what each frontend checks on boot to see whether this student is standing
  // in the right app — see shell/edition/editionRouting.ts.
  //
  // Optional on purpose. The API always sends it now, but a client can be
  // talking to an older one mid-deploy, and the routing rule treats an
  // unrecognised edition as "stay put" rather than ejecting a student from an
  // app that was working for them.
  school: { id: string; name: string; edition?: string };
  classes: { id: string; name: string; role_in_class: string }[];
}

export async function signup(input: {
  email: string;
  password: string;
  display_name: string;
  join_code: string;
}): Promise<void> {
  await apiFetch<Record<string, never>>("/api/v1/auth/signup", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function verifyEmail(token: string): Promise<MeUser> {
  const r = await apiFetch<{ user: MeUser }>("/api/v1/auth/verify-email", {
    method: "POST",
    body: JSON.stringify({ token }),
  });
  return r.user;
}

export async function signin(input: { email: string; password: string }): Promise<MeUser> {
  const r = await apiFetch<{ user: MeUser }>("/api/v1/auth/signin", {
    method: "POST",
    body: JSON.stringify(input),
  });
  return r.user;
}

export async function signout(): Promise<void> {
  await apiFetch<void>("/api/v1/auth/signout", { method: "POST" });
}

export async function getMe(): Promise<MeUser> {
  const r = await apiFetch<{ user: MeUser }>("/api/v1/auth/me");
  return r.user;
}

// Persist the student's chosen accent preset. Pure preference write.
export async function setAccent(accent: AccentId): Promise<void> {
  await apiFetch<Record<string, never>>("/api/v1/users/me/accent", {
    method: "PUT",
    body: JSON.stringify({ accent }),
  });
}

// Persist the student's chosen page background colorway. Pure preference write.
export async function setBackground(background: BackgroundId): Promise<void> {
  await apiFetch<{ background: string }>("/api/v1/users/me/background", {
    method: "PUT",
    body: JSON.stringify({ background }),
  });
}

// Stamp that the student finished/dismissed onboarding. No body; pure flag write.
export async function putOnboarding(): Promise<void> {
  await apiFetch<{ ok: boolean }>("/api/v1/users/me/onboarding", { method: "PUT" });
}

// Save free-text feedback from the nav rail's 反馈 button. Fire-and-forget from
// the caller's perspective — the id isn't surfaced to the student.
export async function submitFeedback(text: string): Promise<void> {
  await apiFetch<{ id: string }>("/api/v1/feedback", {
    method: "POST",
    body: JSON.stringify({ text }),
  });
}

import { apiFetch } from "./client";
import type { AccentId } from "../ui/accent";

export interface MeUser {
  id: string;
  email: string;
  display_name: string;
  role: string;
  avatar_color: string;
  school: { id: string; name: string };
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

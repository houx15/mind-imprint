import { apiFetch } from "./client";
import type { MeUser } from "@/api";

/**
 * api/auth — lite's calls to the SHARED auth endpoints.
 *
 * Auth is not a lite feature or a pro feature; it is one surface both
 * editions sit on. `/auth/signin`, `/auth/signup`, `/auth/signout` and
 * `/auth/me` are ungated on the server (only the edition-specific route
 * groups are wrapped in `requireEdition`), and the session cookie the API
 * sets is same-site for every `*.uni-robot.cn` frontend — so signing in here
 * and signing in on the pro host are literally the same act.
 *
 * These are thin wrappers over LITE's own `apiFetch` rather than imports of
 * pro's `api.signin`, for the same reason lite has its own client at all:
 * pro's api barrel carries the whole project/studio surface lite has no use
 * for. `MeUser` is imported as a TYPE only (erased at build time), so the two
 * apps cannot drift on the payload's shape.
 */
export type { MeUser };

export async function signin(input: { email: string; password: string }): Promise<MeUser> {
  const r = await apiFetch<{ user: MeUser }>("/api/v1/auth/signin", {
    method: "POST",
    body: JSON.stringify(input),
  });
  return r.user;
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

export async function signout(): Promise<void> {
  await apiFetch<void>("/api/v1/auth/signout", { method: "POST" });
}

export async function getMe(): Promise<MeUser> {
  const r = await apiFetch<{ user: MeUser }>("/api/v1/auth/me");
  return r.user;
}

/** Persist the student's accent choice. Same endpoint pro's settings uses. */
export async function setAccent(accent: string): Promise<void> {
  await apiFetch<Record<string, never>>("/api/v1/users/me/accent", {
    method: "PUT",
    body: JSON.stringify({ accent }),
  });
}

/** Persist the student's page-background choice. */
export async function setBackground(background: string): Promise<void> {
  await apiFetch<{ background: string }>("/api/v1/users/me/background", {
    method: "PUT",
    body: JSON.stringify({ background }),
  });
}

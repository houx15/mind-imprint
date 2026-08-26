// api/client.ts — the lite shell's fetch wrapper. Mirrors
// apps/web/src/api/client.ts's shape exactly: same envelope, same ApiError
// shape, same credentials:"include" (session cookie auth is shared
// infrastructure, Tasks 1-8). Lite gets its OWN copy rather than importing
// the pro one because the pro client (apps/web/src/api) carries the whole
// project/studio surface lite doesn't have — this file is deliberately just
// the fetch primitive.
//
// Error envelope confirmed against apps/api/internal/httpx/errors.go
// (WriteError): `{"error": {"code": string, "message": string, "details"?:
// any}}` — same shape apps/web's client already assumes.

export const API_BASE = (import.meta.env.VITE_API_BASE_URL ?? "").replace(/\/+$/, "");

export class ApiError extends Error {
  constructor(
    public readonly code: string,
    message: string,
    public readonly status: number,
    public readonly details?: unknown,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

export async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    ...init,
    credentials: "include",
    headers: { "Content-Type": "application/json", ...(init?.headers ?? {}) },
  });
  if (!res.ok) {
    let code = "internal_error";
    let message = `HTTP ${res.status}`;
    let details: unknown;
    try {
      const body = await res.json();
      if (body?.error) {
        code = body.error.code ?? code;
        message = body.error.message ?? message;
        details = body.error.details;
      }
    } catch {
      /* non-JSON error body */
    }
    throw new ApiError(code, message, res.status, details);
  }
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

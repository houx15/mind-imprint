export const API_BASE = (import.meta.env.VITE_API_BASE_URL ?? "").replace(/\/+$/, "");

export class ApiError extends Error {
  constructor(public readonly code: string, message: string, public readonly status: number, public readonly details?: unknown) {
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
      if (body?.error) { code = body.error.code ?? code; message = body.error.message ?? message; details = body.error.details; }
    } catch { /* non-JSON error body */ }
    throw new ApiError(code, message, res.status, details);
  }
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

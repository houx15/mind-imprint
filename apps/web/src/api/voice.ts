import { API_BASE, ApiError } from "./client";

export async function synthesize(text: string, opts?: { speed?: number }): Promise<string> {
  const res = await fetch(`${API_BASE}/api/v1/voice/tts`, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json", Accept: "audio/mpeg" },
    body: JSON.stringify({ text, speed: opts?.speed ?? 1 }),
  });
  if (!res.ok) throw new ApiError("voice_error", `HTTP ${res.status}`, res.status);
  const blob = await res.blob();
  return URL.createObjectURL(blob);
}

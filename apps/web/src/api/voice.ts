import { API_BASE, ApiError } from "./client";

// Per the WebSocket spec, readyState 1 == OPEN. Using the numeric literal
// (instead of `WebSocket.OPEN`/`ws.OPEN`) keeps this working against fakes
// used in unit tests that stub `globalThis.WebSocket`.
const WS_OPEN = 1;

function asrWsUrl(): string {
  let base: string;
  if (API_BASE) {
    base = API_BASE.replace(/^http/, "ws");
  } else {
    base = location.origin.replace(/^http/, "ws");
  }
  return `${base}/api/v1/voice/asr`;
}

type AsrServerMessage =
  | { type: "partial"; text: string }
  | { type: "final"; text: string }
  | { type: "error"; message: string };

export class AsrStream {
  private ws: WebSocket;
  private partialCb: ((text: string) => void) | null = null;
  private finalCb: ((text: string) => void) | null = null;
  private errorCb: ((message: string) => void) | null = null;

  constructor() {
    this.ws = new WebSocket(asrWsUrl());
    this.ws.onmessage = (ev: MessageEvent) => {
      let msg: AsrServerMessage;
      try {
        msg = JSON.parse(ev.data as string);
      } catch {
        return;
      }
      switch (msg.type) {
        case "partial":
          this.partialCb?.(msg.text);
          break;
        case "final":
          this.finalCb?.(msg.text);
          break;
        case "error":
          this.errorCb?.(msg.message);
          break;
      }
    };
  }

  onPartial(cb: (text: string) => void): void {
    this.partialCb = cb;
  }

  onFinal(cb: (text: string) => void): void {
    this.finalCb = cb;
  }

  onError(cb: (message: string) => void): void {
    this.errorCb = cb;
  }

  sendPCM(pcm: Int16Array): void {
    if (this.ws.readyState !== WS_OPEN) return;
    this.ws.send(pcm.buffer);
  }

  stop(): void {
    if (this.ws.readyState === WS_OPEN) {
      this.ws.send(JSON.stringify({ type: "stop" }));
    }
    this.ws.close();
  }
}

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

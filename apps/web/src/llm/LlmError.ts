import type { LlmFormat } from "./types";

export interface LlmErrorOptions { status?: number; provider?: LlmFormat; body?: unknown }

export class LlmError extends Error {
  status?: number;
  provider?: LlmFormat;
  body?: unknown;
  constructor(message: string, opts: LlmErrorOptions = {}) {
    super(message);
    this.name = "LlmError";
    this.status = opts.status;
    this.provider = opts.provider;
    this.body = opts.body;
  }
}

import { z } from "zod";

export const ChatThread = z.object({ id: z.string(), title: z.string(), createdAt: z.string() });
export type ChatThread = z.infer<typeof ChatThread>;

export const ChatMessage = z.object({
  id: z.string(),
  role: z.enum(["user", "assistant", "system"]),
  content: z.string(),
  modality: z.enum(["text", "voice", "file", "image"]),
  createdAt: z.string(),
});
export type ChatMessage = z.infer<typeof ChatMessage>;

export const ChatCardOffer = z.object({ cardInstanceId: z.string(), cardId: z.string(), materialId: z.string() });
export type ChatCardOffer = z.infer<typeof ChatCardOffer>;

import { useSyncExternalStore } from "react";
import type { Conversation, ConvState } from "./createConversation";

export function useConversation(conv: Conversation): ConvState {
  return useSyncExternalStore(conv.subscribe, conv.getSnapshot);
}

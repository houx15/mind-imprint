import { useSyncExternalStore } from "react";
import type { Store } from "./createStore";
import type { StoreState } from "./schema";

export function useStore(store: Store): StoreState {
  return useSyncExternalStore(store.subscribe, store.getSnapshot);
}

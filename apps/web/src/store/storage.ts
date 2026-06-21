export const STORE_KEY = "mk.store";

export interface RawStorage {
  getItem(key: string): string | null;
  setItem(key: string, value: string): void;
}

export function makeMemoryStorage(): RawStorage {
  const map = new Map<string, string>();
  return {
    getItem: (key) => (map.has(key) ? map.get(key)! : null),
    setItem: (key, value) => {
      map.set(key, value);
    },
  };
}

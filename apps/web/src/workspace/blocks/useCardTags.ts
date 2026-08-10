import { useCallback, useEffect, useState } from "react";
import { getCardTags, putCardTag, type CardTag } from "../../api/cardTags";

// useCardTags — §4 gap G8 · loads + mutates the per-guided-part status tags.
// Optimistic: the local map updates immediately, then persists.
export function useCardTags(projectId: string) {
  const [tags, setTags] = useState<Record<string, string>>({});

  useEffect(() => {
    let cancelled = false;
    void getCardTags(projectId).then((t) => { if (!cancelled) setTags(t); }).catch(() => {});
    return () => { cancelled = true; };
  }, [projectId]);

  const setTag = useCallback(
    (key: string, status: CardTag) => {
      setTags((prev) => {
        const next = { ...prev };
        if (status === "") delete next[key];
        else next[key] = status;
        return next;
      });
      void putCardTag(projectId, key, status).catch(() => {});
    },
    [projectId],
  );

  return { tags, setTag };
}

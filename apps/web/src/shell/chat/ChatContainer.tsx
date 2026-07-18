import { useEffect, useRef, useState } from "react";
import type { CardInstance, ChatCardOffer, ChatThread } from "@mind-imprint/contracts";
import { api } from "../../api";
import { ChatSurface, type ChatEntry } from "./ChatSurface";
import { ChatReport } from "./ChatReport";

let localIdSeq = 0;
function nextLocalId(prefix: string) {
  localIdSeq += 1;
  return `${prefix}-${Date.now()}-${localIdSeq}`;
}

// Data layer for the free-chat surface (Slice 11 T7): loads the thread list,
// owns the active thread + its message log + the in-flight streaming state,
// and hands plain callbacks down to the presentational ChatSurface. No
// layout/inline-style/copy decisions live here — those are ChatSurface's,
// bound to docs/design/思维印记_工作区.dc.html lines 527-666.
export function ChatContainer() {
  const [threads, setThreads] = useState<ChatThread[]>([]);
  const [activeThreadId, setActiveThreadId] = useState<string | null>(null);
  const [entries, setEntries] = useState<ChatEntry[]>([]);
  const [draft, setDraft] = useState("");
  const [sending, setSending] = useState(false);
  const [reportOpen, setReportOpen] = useState(false);
  const entriesRef = useRef<ChatEntry[]>([]);
  entriesRef.current = entries;

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      const list = await api.listThreads();
      if (cancelled) return;
      setThreads(list);
      if (list.length > 0) setActiveThreadId(list[0]!.id);
    })();
    return () => { cancelled = true; };
  }, []);

  useEffect(() => {
    setReportOpen(false);
    if (!activeThreadId) { setEntries([]); return; }
    let cancelled = false;
    void (async () => {
      const msgs = await api.getMessages(activeThreadId);
      if (cancelled) return;
      setEntries(
        msgs
          .filter((m): m is typeof m & { role: "user" | "assistant" } => m.role === "user" || m.role === "assistant")
          .map((m) => ({ id: m.id, role: m.role, text: m.content })),
      );
    })();
    return () => { cancelled = true; };
  }, [activeThreadId]);

  async function handleNewConversation() {
    const thread = await api.createThread();
    setThreads((prev) => [thread, ...prev]);
    setActiveThreadId(thread.id);
    setEntries([]);
    setReportOpen(false);
  }

  function handleSelectThread(id: string) {
    setActiveThreadId(id);
    setReportOpen(false);
  }

  async function handleSend() {
    const text = draft.trim();
    if (!text || sending) return;

    let threadId = activeThreadId;
    if (!threadId) {
      const thread = await api.createThread();
      setThreads((prev) => [thread, ...prev]);
      threadId = thread.id;
      setActiveThreadId(threadId);
    }

    setDraft("");
    setSending(true);

    const userEntryId = nextLocalId("user");
    const assistantEntryId = nextLocalId("assistant");
    setEntries((prev) => [...prev, { id: userEntryId, role: "user", text }, { id: assistantEntryId, role: "assistant", text: "" }]);

    try {
      for await (const event of api.chatTurn(threadId, text)) {
        if (event.type === "reply") {
          setEntries((prev) => prev.map((e) => (e.id === assistantEntryId ? { ...e, text: event.body } : e)));
        } else if (event.type === "card") {
          const offer: ChatCardOffer = { cardInstanceId: event.cardInstanceId, cardId: event.cardId, materialId: event.materialId };
          setEntries((prev) => prev.map((e) => (e.id === assistantEntryId ? { ...e, offer, offerPhase: "offered" } : e)));
        } else if (event.type === "error") {
          const message = event.message || "出错了，请重试";
          setEntries((prev) => prev.map((e) => (e.id === assistantEntryId ? { ...e, text: message } : e)));
        }
      }
    } finally {
      setSending(false);
    }
  }

  function handleAcceptOffer(entryId: string) {
    setEntries((prev) => prev.map((e) => (e.id === entryId ? { ...e, offerPhase: "accepted" } : e)));
  }

  function handleDismissOffer(entryId: string) {
    const entry = entriesRef.current.find((e) => e.id === entryId);
    if (entry?.offer && activeThreadId) void api.skipChatCard(activeThreadId, entry.offer.cardInstanceId);
    setEntries((prev) => prev.map((e) => (e.id === entryId ? { ...e, offerPhase: "resolved" } : e)));
  }

  function handleCardSubmit(entryId: string, env: CardInstance) {
    const entry = entriesRef.current.find((e) => e.id === entryId);
    if (!entry?.offer || !activeThreadId) return;
    void api.submitChatCard(activeThreadId, entry.offer.cardInstanceId, {
      field_values: env.field_values,
      event_trace: env.event_trace,
      anchors: [],
    });
    setEntries((prev) => prev.map((e) => (e.id === entryId ? { ...e, offerPhase: "resolved" } : e)));
  }

  function handleCardSkip(entryId: string) {
    handleDismissOffer(entryId);
  }

  // Gate: the report is only worth generating once the thread has an actual
  // AI reply to reflect on — an all-user or empty thread has nothing to
  // assess yet.
  const canOpenReport = !!activeThreadId && entries.some((e) => e.role === "assistant" && e.text.trim().length > 0);

  if (reportOpen && activeThreadId) {
    return <ChatReport threadId={activeThreadId} onClose={() => setReportOpen(false)} />;
  }

  return (
    <ChatSurface
      threads={threads}
      activeThreadId={activeThreadId}
      entries={entries}
      draft={draft}
      sending={sending}
      onDraftChange={setDraft}
      onSend={() => void handleSend()}
      onNewConversation={() => void handleNewConversation()}
      onSelectThread={handleSelectThread}
      onAcceptOffer={handleAcceptOffer}
      onDismissOffer={handleDismissOffer}
      onCardSubmit={handleCardSubmit}
      onCardSkip={handleCardSkip}
      canOpenReport={canOpenReport}
      onOpenReport={() => setReportOpen(true)}
    />
  );
}

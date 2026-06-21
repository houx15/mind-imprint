import { describe, it, expect } from "vitest";
import { makeMemoryStorage } from "../store";
import { createSession, SESSION_KEY } from "./session";

describe("createSession", () => {
  it("defaults to logged-out with the default avatar", () => {
    const s = createSession({ storage: makeMemoryStorage() });
    expect(s.getSnapshot()).toEqual({ authed: false, aiAvatar: "#2A3B7A" });
  });
  it("persists authed + avatar across instances", () => {
    const storage = makeMemoryStorage();
    const a = createSession({ storage });
    a.setAuthed(true);
    a.setAvatar("#D98263");
    const b = createSession({ storage });
    expect(b.getSnapshot()).toEqual({ authed: true, aiAvatar: "#D98263" });
  });
  it("returns a stable snapshot ref when nothing changes", () => {
    const s = createSession({ storage: makeMemoryStorage() });
    const before = s.getSnapshot();
    s.setAuthed(false); // no-op (already false)
    expect(s.getSnapshot()).toBe(before);
  });
  it("notifies subscribers on change only", () => {
    const s = createSession({ storage: makeMemoryStorage() });
    let n = 0;
    s.subscribe(() => { n++; });
    s.setAuthed(true);   // change
    s.setAuthed(true);   // no-op
    expect(n).toBe(1);
  });
  it("fail-soft on corrupt storage", () => {
    const storage = makeMemoryStorage();
    storage.setItem(SESSION_KEY, "{not json");
    const s = createSession({ storage });
    expect(s.getSnapshot()).toEqual({ authed: false, aiAvatar: "#2A3B7A" });
  });
});

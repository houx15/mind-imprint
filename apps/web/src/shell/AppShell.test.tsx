import { describe, it, expect, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { AppShell } from "./AppShell";
import { createStore } from "../store/createStore";
import { createSession } from "./session";

vi.mock("../api", async (orig) => {
  const real = await orig<typeof import("../api")>();
  return {
    ...real,
    api: {
      listTasks: vi.fn(async () => []),
      createTask: vi.fn(),
      getMe: vi.fn(async () => { throw new Error("401"); }),
      signout: vi.fn(async () => {}),
    },
  };
});

const ME = { id: "u1", email: "p@d.local", display_name: "Phoebe", role: "student", avatar_color: "#7C9CF0", school: { id: "s1", name: "Demo" }, classes: [] };
const mem = () => { let s = "{}"; return { getItem: () => s, setItem: (_: string, v: string) => { s = v; } }; };

describe("AppShell boot gate", () => {
  it("shows the directory when getMe succeeds", async () => {
    const store = createStore({});
    const session = createSession({ storage: mem() });
    const client = { getMe: vi.fn(async () => ME), signout: vi.fn(), listTasks: vi.fn(async () => []), createTask: vi.fn() };
    render(<AppShell store={store} session={session} client={client as never} />);
    await waitFor(() => expect(screen.getByText("今天你在尝试什么？")).toBeInTheDocument());
  });

  it("shows AuthScreen when getMe rejects (401)", async () => {
    const store = createStore({});
    const session = createSession({ storage: mem() });
    const client = { getMe: vi.fn(async () => { throw new Error("401"); }), signout: vi.fn() };
    render(<AppShell store={store} session={session} client={client as never} />);
    await waitFor(() => expect(screen.getAllByText("登录").length).toBeGreaterThan(0));
  });
});

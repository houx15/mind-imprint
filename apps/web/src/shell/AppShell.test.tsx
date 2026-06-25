import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { AppShell } from "./AppShell";
import { createStore } from "../store/createStore";
import { createSession } from "./session";

vi.mock("../api", () => ({ api: { listTasks: vi.fn(async () => []), createTask: vi.fn() } }));

describe("AppShell", () => {
  it("shows the directory directly when authed — no key gate", () => {
    const store = createStore({});
    const session = createSession({ storage: { getItem: () => JSON.stringify({ authed: true, aiAvatar: "" }), setItem: () => {} } });
    render(<AppShell store={store} session={session} />);
    expect(screen.getByText("今天你在尝试什么？")).toBeInTheDocument();
  });
});

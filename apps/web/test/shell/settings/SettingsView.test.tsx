import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { SettingsView } from "@/shell/settings/SettingsView";
import { createSession, makeMemoryStorage } from "@/shell/session";
import { AccentProvider } from "@/ui/accent";
import type { MeUser } from "@/api";

beforeEach(() => localStorage.clear());

const ME: MeUser = {
  id: "u1",
  email: "phoebe@ibschool.edu",
  display_name: "Phoebe Chen",
  role: "student",
  avatar_color: "vermilion", page_background: "paper",
  school: { id: "s1", name: "启明国际学校" },
  classes: [
    { id: "c1", name: "IB DP1 · A班", role_in_class: "student" },
    { id: "c2", name: "TOK", role_in_class: "student" },
  ],
};

function renderSettings(opts: { user?: MeUser | null; onLogout?: () => void; onPersist?: (id: string) => void } = {}) {
  const session = createSession({ storage: makeMemoryStorage() });
  const onLogout = opts.onLogout ?? vi.fn();
  const user = "user" in opts ? (opts.user ?? null) : ME;
  render(
    <AccentProvider onPersist={opts.onPersist}>
      <SettingsView session={session} onLogout={onLogout} user={user} />
    </AccentProvider>,
  );
  return { session, onLogout };
}

describe("SettingsView", () => {
  it("shows real identity: display_name, email, school, and classes", () => {
    renderSettings();
    expect(screen.getByText("Phoebe Chen")).toBeInTheDocument();
    expect(screen.getByDisplayValue("Phoebe Chen")).toBeInTheDocument();
    expect(screen.getByDisplayValue("phoebe@ibschool.edu")).toBeInTheDocument();
    expect(screen.getByText(/启明国际学校/)).toBeInTheDocument();
    expect(screen.getByText(/IB DP1 · A班/)).toBeInTheDocument();
    expect(screen.getByText(/TOK/)).toBeInTheDocument();
  });

  it("shows a graceful placeholder (not fake data) when user is null", () => {
    renderSettings({ user: null });
    expect(screen.queryByText("Phoebe Chen")).toBeNull();
    expect(screen.queryByDisplayValue("Phoebe Chen")).toBeNull();
    expect(screen.queryByText(/ibschool\.edu/)).toBeNull();
  });

  it("renders the 8 accent-preset swatches", () => {
    renderSettings();
    expect(screen.getAllByTestId("accent-swatch")).toHaveLength(8);
  });

  it("clicking a non-active swatch calls setAccent, updates selection, and persists to the server", () => {
    const onPersist = vi.fn();
    renderSettings({ onPersist });

    const teal = screen.getByRole("button", { name: "松石青" });
    expect(teal).toHaveAttribute("aria-pressed", "false");

    fireEvent.click(teal);

    expect(onPersist).toHaveBeenCalledWith("teal");
    expect(teal).toHaveAttribute("aria-pressed", "true");
    expect(localStorage.getItem("mk-accent")).toBe("teal");
  });

  it("fires onLogout from 退出登录", () => {
    const { onLogout } = renderSettings();
    fireEvent.click(screen.getByText("退出登录"));
    expect(onLogout).toHaveBeenCalledOnce();
  });

  it("does not reference the old local-only aiAvatar mechanism", () => {
    renderSettings();
    expect(screen.queryByTestId("avatar-option")).toBeNull();
  });
});

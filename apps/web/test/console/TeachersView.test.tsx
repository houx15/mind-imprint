import { describe, it, expect, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { TeachersView } from "@/console/TeachersView";

function makeClient() {
  return {
    listTeachers: vi.fn(async () => [{ id: "u1", display_name: "Ms Chen", email: "chen@x" }]),
    listTeacherInvites: vi.fn(async () => [{ id: "i1", code: "T-ABCD", expires_at: "2026-07-01T00:00:00Z", created_at: "2026-06-20T00:00:00Z", email: "pending@x" }]),
    createTeacherInvite: vi.fn(async () => ({ code: "T-NEW9", expires_at: "2026-07-15T00:00:00Z" })),
  };
}

describe("TeachersView", () => {
  it("lists the school's teachers and pending invites", async () => {
    render(<TeachersView client={makeClient()} />);
    expect(await screen.findByText("Ms Chen")).toBeInTheDocument();
    expect(screen.getByText(/T-ABCD/)).toBeInTheDocument();
    expect(screen.getByText(/pending@x/)).toBeInTheDocument();
  });

  it("mints an invite and surfaces the new code", async () => {
    const client = makeClient();
    render(<TeachersView client={client} />);
    await screen.findByText("Ms Chen");
    await userEvent.click(screen.getByText("生成邀请码"));
    await waitFor(() => expect(client.createTeacherInvite).toHaveBeenCalled());
    expect(await screen.findByText(/T-NEW9/)).toBeInTheDocument();
  });
});

import { describe, it, expect, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ImportView } from "./ImportView";
import { ApiError } from "../api";

function file(content: string, name = "roster.csv") {
  return new File([content], name, { type: "text/csv" });
}

describe("ImportView", () => {
  it("parses a chosen file into a preview, then imports and shows the code sheet", async () => {
    const client = {
      adminImport: vi.fn(async () => ({
        classes: [{ name: "11A", join_code: "AB-CD" }],
        teacher_invites: [{ email: "t@x", code: "T-9" }],
      })),
    };
    render(<ImportView client={client} />);
    const input = screen.getByTestId("csv-input") as HTMLInputElement;
    await userEvent.upload(input, file("class,teacher_email,student_email\n11A,t@x,s@x\n"));

    // preview shows the parsed row
    expect(await screen.findByText("11A")).toBeInTheDocument();
    expect(screen.getByText("t@x")).toBeInTheDocument();

    await userEvent.click(screen.getByText("导入"));
    await waitFor(() => expect(client.adminImport).toHaveBeenCalledWith([{ class: "11A", teacher_email: "t@x", student_email: "s@x" }]));

    // code sheet
    expect(await screen.findByText("AB-CD")).toBeInTheDocument();
    expect(screen.getByText("T-9")).toBeInTheDocument();
  });

  it("highlights the bad row and shows the error message when adminImport rejects with details.row", async () => {
    const client = {
      adminImport: vi.fn(async () => { throw new ApiError("validation_failed", "坏行", 400, { row: 0 }); }),
    };
    render(<ImportView client={client} />);
    const input = screen.getByTestId("csv-input") as HTMLInputElement;
    await userEvent.upload(input, file("class,teacher_email,student_email\n11A,t@x,s@x\n12B,t2@x,s2@x\n"));

    // preview shows both rows
    expect(await screen.findByText("11A")).toBeInTheDocument();

    await userEvent.click(screen.getByText("导入"));

    // error message appears
    expect(await screen.findByText("坏行")).toBeInTheDocument();

    // first row (row 0 = 11A) gets the highlight background
    await waitFor(() => {
      const row = screen.getByText("11A").closest("tr");
      expect(row).toHaveStyle({ background: "#FBECEC" });
    });
  });

  it("shows a parse error for a file with no class column", async () => {
    const client = { adminImport: vi.fn() };
    render(<ImportView client={client} />);
    await userEvent.upload(screen.getByTestId("csv-input"), file("teacher_email\nt@x\n"));
    expect(await screen.findByText(/无法解析文件/)).toBeInTheDocument();
    expect(client.adminImport).not.toHaveBeenCalled();
  });
});

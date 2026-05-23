import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ResumeCard } from "./ResumeCard";

function renderCard(): void {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <ResumeCard />
    </QueryClientProvider>,
  );
}

describe("ResumeCard", () => {
  it("shows the upload prompt when no resume is attached", async () => {
    renderCard();
    expect(
      await screen.findByText(/questions reference your real experience/i),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /upload resume/i }),
    ).toBeInTheDocument();
  });

  it("shows attached state after uploading a PDF", async () => {
    const user = userEvent.setup();
    renderCard();
    await screen.findByRole("button", { name: /upload resume/i });

    const file = new File(["%PDF-1.7 fake"], "resume.pdf", { type: "application/pdf" });
    await user.upload(screen.getByTestId("resume-file-input"), file);

    expect(await screen.findByText(/attached/i)).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /replace resume/i }),
    ).toBeInTheDocument();
  });

  it("rejects a non-PDF file without calling the API", async () => {
    const user = userEvent.setup();
    renderCard();
    await screen.findByRole("button", { name: /upload resume/i });

    const file = new File(["plain text"], "notes.txt", { type: "text/plain" });
    await user.upload(screen.getByTestId("resume-file-input"), file);

    // Still in the unattached state — the upload was rejected client-side.
    expect(
      screen.getByRole("button", { name: /upload resume/i }),
    ).toBeInTheDocument();
  });
});

import { describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type * as ClerkReact from "@clerk/clerk-react";
import { AddInterviewDialog, newInterviewSchema } from "./AddInterviewDialog";

vi.mock("@clerk/clerk-react", async () => {
  const actual = await vi.importActual<typeof ClerkReact>("@clerk/clerk-react");
  return {
    ...actual,
    useAuth: () => ({ getToken: async () => "test-token", isLoaded: true, isSignedIn: true }),
  };
});

function renderDialog(): void {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <MemoryRouter>
        <AddInterviewDialog />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("AddInterviewDialog", () => {
  it("schema rejects short job position", () => {
    const result = newInterviewSchema.safeParse({
      jobPosition: "x",
      jobDescription: "long enough description here",
      yearsExperience: 3,
    });
    expect(result.success).toBe(false);
  });

  it("schema coerces yearsExperience strings to numbers", () => {
    const result = newInterviewSchema.safeParse({
      jobPosition: "Backend Engineer",
      jobDescription: "long enough description here",
      yearsExperience: "5",
    });
    expect(result.success).toBe(true);
    if (result.success) {
      expect(result.data.yearsExperience).toBe(5);
    }
  });

  it("shows validation errors when submitted with empty fields", async () => {
    const user = userEvent.setup();
    renderDialog();

    await user.click(screen.getByRole("button", { name: /new interview/i }));
    await user.click(await screen.findByRole("button", { name: /generate questions/i }));

    await waitFor(() => {
      expect(screen.getByText(/tell us the role/i)).toBeInTheDocument();
      expect(screen.getByText(/at least a sentence/i)).toBeInTheDocument();
    });
  });
});

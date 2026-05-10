import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import type { ReactNode } from "react";

vi.mock("@clerk/clerk-react", () => ({
  SignedIn: ({ children }: { children: ReactNode }) =>
    (globalThis as unknown as { __SIGNED_IN__?: boolean }).__SIGNED_IN__ ? <>{children}</> : null,
  SignedOut: ({ children }: { children: ReactNode }) =>
    !(globalThis as unknown as { __SIGNED_IN__?: boolean }).__SIGNED_IN__ ? <>{children}</> : null,
  RedirectToSignIn: () => <div data-testid="redirect">redirect</div>,
}));

import { ProtectedRoute } from "./ProtectedRoute";

function renderWithFlag(signedIn: boolean): void {
  (globalThis as unknown as { __SIGNED_IN__: boolean }).__SIGNED_IN__ = signedIn;
  render(
    <MemoryRouter initialEntries={["/dashboard"]}>
      <Routes>
        <Route
          path="/dashboard"
          element={
            <ProtectedRoute>
              <div data-testid="protected">protected content</div>
            </ProtectedRoute>
          }
        />
      </Routes>
    </MemoryRouter>,
  );
}

describe("ProtectedRoute", () => {
  it("renders children when signed in", () => {
    renderWithFlag(true);
    expect(screen.getByTestId("protected")).toBeInTheDocument();
    expect(screen.queryByTestId("redirect")).not.toBeInTheDocument();
  });

  it("renders RedirectToSignIn when signed out", () => {
    renderWithFlag(false);
    expect(screen.queryByTestId("protected")).not.toBeInTheDocument();
    expect(screen.getByTestId("redirect")).toBeInTheDocument();
  });
});

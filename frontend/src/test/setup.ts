import "@testing-library/jest-dom/vitest";
import { afterAll, afterEach, beforeAll, vi } from "vitest";
import { setupServer } from "msw/node";
import { handlers } from "./msw/handlers";

// Default URL/env for env.ts and Clerk during tests; safe to mutate in vitest.
const testEnv = import.meta.env as Record<string, string>;
testEnv["VITE_API_BASE_URL"] = testEnv["VITE_API_BASE_URL"] ?? "/api/v1";
testEnv["VITE_CLERK_PUBLISHABLE_KEY"] =
  testEnv["VITE_CLERK_PUBLISHABLE_KEY"] ?? "pk_test_dGVzdC1jbGVyay50ZXN0JA";

const server = setupServer(...handlers);

beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());

// Stub APIs that jsdom does not implement.
Object.defineProperty(window, "matchMedia", {
  writable: true,
  value: vi.fn().mockImplementation((query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: vi.fn(),
    removeListener: vi.fn(),
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn(),
  })),
});

class FakeUtterance {
  text: string;
  onend: null | (() => void) = null;
  onerror: null | (() => void) = null;
  rate = 1;
  pitch = 1;
  constructor(text: string) {
    this.text = text;
  }
}
Object.defineProperty(window, "SpeechSynthesisUtterance", {
  writable: true,
  configurable: true,
  value: FakeUtterance,
});

const mediaDevicesMock = {
  getUserMedia: vi.fn().mockResolvedValue({
    getTracks: () => [{ stop: vi.fn() }],
  }),
};
Object.defineProperty(navigator, "mediaDevices", {
  writable: true,
  configurable: true,
  value: mediaDevicesMock,
});

export { server };

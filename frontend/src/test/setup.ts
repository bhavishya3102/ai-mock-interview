import "@testing-library/jest-dom/vitest";
import { afterAll, afterEach, beforeAll, vi } from "vitest";
import { setupServer } from "msw/node";
import { handlers } from "./msw/handlers";

// Default URL/env for env.ts and Clerk during tests; safe to mutate in vitest.
// Tests ALWAYS use an absolute base URL — Node's fetch (used by msw/node)
// cannot resolve relative URLs without a base. The .env file's "/api/v1"
// is overridden unconditionally; MSW's `*/api/v1` pattern matches any host
// so the chosen origin only has to satisfy URL parsing.
const testEnv = import.meta.env as Record<string, string>;
testEnv["VITE_API_BASE_URL"] = "http://localhost:8080/api/v1";
testEnv["VITE_CLERK_PUBLISHABLE_KEY"] =
  testEnv["VITE_CLERK_PUBLISHABLE_KEY"] ?? "pk_test_dGVzdC1jbGVyay50ZXN0JA";

const server = setupServer(...handlers);

beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());

// Stub APIs that jsdom does not implement. Guarded so test files that opt
// into the node environment (e.g. via `// @vitest-environment node` to
// avoid jsdom's AbortSignal incompatibility with undici) don't crash this
// setup when `window` / `navigator` are absent.
if (typeof window !== "undefined") {
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
}

if (typeof navigator !== "undefined") {
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
}

export { server };

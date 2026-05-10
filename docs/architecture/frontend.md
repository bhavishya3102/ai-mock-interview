# Frontend Architecture Plan — AI Mock Interview (React + Vite SPA)

Skills consulted: vercel-react-best-practices, frontend-design, web-design-guidelines, vite, vitest, clerk, clerk-setup.

## 1. File tree

```
frontend/
├── public/
│   ├── logo.png
│   └── interview-overlay.png
├── src/
│   ├── main.tsx                         # entry: ClerkProvider + QueryClient + Router
│   ├── App.tsx
│   ├── routes/
│   │   ├── index.tsx                    # createBrowserRouter config
│   │   ├── root.tsx                     # outlet + global toaster
│   │   ├── _layouts/
│   │   │   ├── DashboardLayout.tsx
│   │   │   └── AuthLayout.tsx
│   │   ├── _guards/
│   │   │   └── ProtectedRoute.tsx
│   │   ├── home/Home.tsx
│   │   ├── dashboard/Dashboard.tsx
│   │   ├── interview/
│   │   │   ├── PreInterview.tsx         # /interview/:mockId
│   │   │   ├── InterviewSession.tsx     # /interview/:mockId/start
│   │   │   └── Feedback.tsx             # /interview/:mockId/feedback
│   │   └── auth/
│   │       ├── SignInPage.tsx
│   │       └── SignUpPage.tsx
│   ├── components/
│   │   ├── ui/                          # shadcn primitives
│   │   ├── layout/
│   │   │   ├── Header.tsx
│   │   │   └── PageContainer.tsx
│   │   ├── dashboard/
│   │   │   ├── AddInterviewDialog.tsx
│   │   │   ├── InterviewCard.tsx
│   │   │   └── InterviewGrid.tsx
│   │   ├── interview/
│   │   │   ├── WebcamPanel.tsx
│   │   │   ├── QuestionPanel.tsx
│   │   │   ├── QuestionPills.tsx
│   │   │   ├── RecordAnswerControl.tsx
│   │   │   ├── SessionFooterControls.tsx
│   │   │   └── FeedbackItem.tsx
│   │   └── feedback/EmptyFeedback.tsx
│   ├── api/
│   │   ├── client.ts                    # fetch wrapper + ApiError + token getter wiring
│   │   ├── interviews.ts                # endpoint functions + types
│   │   └── queryKeys.ts
│   ├── hooks/
│   │   ├── useAuthFetch.ts
│   │   ├── useInterviews.ts
│   │   ├── useInterview.ts
│   │   ├── useInterviewMutations.ts
│   │   ├── useSpeechSynthesis.ts
│   │   ├── useSpeechToText.ts
│   │   ├── useWebcamPermission.ts
│   │   └── use-toast.ts
│   ├── lib/
│   │   ├── cn.ts
│   │   ├── env.ts                       # zod-validates import.meta.env at boot
│   │   ├── format.ts
│   │   └── tokenStore.ts                # holds getToken() ref for non-React callers
│   ├── types/
│   │   ├── api.ts
│   │   └── speech.d.ts
│   ├── styles/
│   │   ├── globals.css
│   │   └── tokens.css
│   └── test/
│       ├── setup.ts
│       └── msw/handlers.ts
├── components.json                      # shadcn config
├── index.html
├── vite.config.ts
├── tsconfig.json + tsconfig.node.json
├── tailwind.config.ts
├── postcss.config.js
├── .env.example                         # VITE_API_BASE_URL, VITE_CLERK_PUBLISHABLE_KEY
├── .eslintrc.cjs
└── package.json
```

## 2. Stack choices

- **Vite 5 + @vitejs/plugin-react-swc** — fast HMR, SWC > Babel.
- **React 18.3** — concurrent features (`useTransition`, `useDeferredValue`).
- **TypeScript strict** — required for typed API contracts + react-hook-form/zod inference.
- **react-router-dom v6.26 (data routers)** — `createBrowserRouter`, auth gating via element wrappers.
- **@tanstack/react-query v5** — request dedup + cache, replaces server-action fetching.
- **zustand 4** — small interview-session client state only.
- **Tailwind 3.4 + shadcn/ui (Radix)** — match old app's primitives + add sonner.
- **react-hook-form + zod + @hookform/resolvers** — typed forms.
- **react-webcam 7** — camera preview (existing dep).
- **react-hook-speech-to-text 0.8** — STT (existing dep). Wrap behind `useSpeechToText`.
- **native fetch** in thin client (no axios).
- **lucide-react** — direct named imports (no barrel).
- **date-fns 3** — replaces moment; tree-shakable.
- **@clerk/clerk-react v5 (Core 2)** — auth.

## 3. Routing map

| Path | Component | Auth | Data |
|---|---|---|---|
| `/` | Home | public | none |
| `/sign-in/*` | SignInPage (`<SignIn routing="path" path="/sign-in"/>`) | public | none |
| `/sign-up/*` | SignUpPage | public | none |
| `/dashboard` | Dashboard inside DashboardLayout | protected | `useInterviews()` |
| `/interview/:mockId` | PreInterview | protected | `useInterview(mockId)` |
| `/interview/:mockId/start` | InterviewSession | protected | `useInterview(mockId)` |
| `/interview/:mockId/feedback` | Feedback | protected | `useInterviewFeedback(mockId)` |
| `*` | NotFound | public | none |

Protection: `element={<ProtectedRoute><DashboardLayout/></ProtectedRoute>}` using Clerk's `<SignedIn>/<SignedOut>` + `<RedirectToSignIn>`.

## 4. Component breakdown

**Home** — `Header`, hero `Section`, primary CTA `Button` → `useNavigate("/dashboard")`. Static JSX hoisted outside component.

**Dashboard** — `Header`, `AddInterviewDialog`, `InterviewGrid` → maps `InterviewCard[]`. Card has Feedback + Start buttons; navigation uses `<Link>`.

**AddInterviewDialog** — controlled `Dialog`; form built with react-hook-form + zod (`jobPosition`, `jobDescription`, `yearsExperience: z.coerce.number().int().min(0)`); on submit calls `useCreateInterview` mutation → on success `invalidateQueries(["interviews"])` and `navigate('/interview/' + mockId)`.

**PreInterview** — `JobDetailsCard` (server data), `WebcamPanel` (handles permission flow with `onUserMedia/onUserMediaError`, fallback button "Enable webcam"), `Button` → `Link to="start"`.

**InterviewSession** — split layout (`flex md:flex-row`):
- Left: `QuestionPanel` = `QuestionPills` (mapped buttons; `activeQuestionIndex` from store) + current `QuestionText` + `SpeakerButton` (calls `useSpeechSynthesis().speak()`) + Note card.
- Right: `WebcamPanel` (preview only), `RecordAnswerControl` (mic button, transcript display), `SessionFooterControls` (Prev/Next/End — End → `navigate("feedback")`).
- Session store (zustand): `{ mockId, questions, activeIndex, setActive, transcript, setTranscript, reset }`.

**Feedback** — `EmptyFeedback` if zero rows; else `<Collapsible>` per `FeedbackItem` (question / userAnswer / correctAnswer / feedback / rating); "Go to Home" `Button`.

**Header** — logo `<Link to="/">`, nav links, `<UserButton/>` from Clerk; render only `<SignedIn>` controls when authed.

## 5. Auth integration

- `main.tsx` wraps everything in `<ClerkProvider>` then `<QueryClientProvider>` then `<RouterProvider>`.
- Token retrieval: `useAuth().getToken()` per request; register the function once in module-level holder for non-React callers.
  - `lib/tokenStore.ts` exposes `setTokenGetter(fn)` and `getAuthToken()`.
  - Small `<AuthBridge/>` component inside `<ClerkProvider>` calls `useAuth()` and on mount registers `getToken` via `setTokenGetter` (clears on unmount).
- API client interceptor: every `apiFetch` awaits `getAuthToken()`, sets `Authorization: Bearer <jwt>`, surfaces 401 by throwing `UnauthorizedError`.
- Route protection: `ProtectedRoute` returns `<><SignedIn>{children}</SignedIn><SignedOut><RedirectToSignIn/></SignedOut></>`.
- Sign-in/up: Clerk SPA components mounted with `routing="path"` + matching catch-all routes. `signInUrl="/sign-in"`, `signUpUrl="/sign-up"`, `afterSignInUrl="/dashboard"` set on `<ClerkProvider>`.

## 6. API client design

`api/client.ts` — single `apiFetch<T>(path, init?)`:
1. Reads `VITE_API_BASE_URL` from `lib/env.ts`.
2. Awaits `getAuthToken()`; if absent and protected, throws `UnauthorizedError`.
3. Sets `Content-Type: application/json` for non-FormData bodies, attaches bearer.
4. On non-2xx, parses `{error: {code, message}}` and throws typed `ApiError(status, message, code)`.
5. Handles 204 (returns `null`).

`api/interviews.ts` — typed pure functions:
- `listInterviews(): Promise<InterviewSummary[]>` → `GET /interviews`
- `getInterview(mockId): Promise<Interview>` → `GET /interviews/:mockId`
- `createInterview(input: CreateInterviewInput): Promise<Interview>` → `POST /interviews`
- `submitAnswer(mockId, payload: SubmitAnswerInput): Promise<UserAnswer>` → `POST /interviews/:mockId/answers`
- `getFeedback(mockId): Promise<UserAnswer[]>` → `GET /interviews/:mockId/feedback`

`hooks/useInterviews.ts` — TanStack Query wrappers. Query keys factory:
```
interviews.all = ["interviews"]
interviews.detail(id) = ["interviews", id]
interviews.feedback(id) = ["interviews", id, "feedback"]
```

## 7. TypeScript types (`src/types/api.ts`) — ALIGNED WITH GO BACKEND

```ts
type ID = string;
export interface Question { question: string; answer: string; }
export interface Interview {
  mockId: ID;
  jobPosition: string;
  jobDescription: string;
  yearsExperience: number;
  questions: Question[];
  createdAt: string;
}
export interface InterviewSummary {
  mockId: ID;
  jobPosition: string;
  jobDescription: string;
  yearsExperience: number;
  createdAt: string;
}
export interface UserAnswer {
  questionIndex: number;
  question: string;
  userAnswer: string;
  correctAnswer: string;
  feedback: string;
  rating: number;            // 1-10 int (NOT string)
  createdAt: string;
}
export interface CreateInterviewInput {
  jobPosition: string;
  jobDescription: string;
  yearsExperience: number;
}
export interface SubmitAnswerInput {
  questionIndex: number;
  userAnswer: string;
}
export interface ApiErrorBody {
  error: { code: string; message: string };
}
```

**API base path**: `/api/v1` (set in `VITE_API_BASE_URL`). Default dev value: `/api/v1` (Vite proxy maps `/api` → `:8080`).

## 8. Speech + webcam

- **`useWebcamPermission`**: `state: 'idle'|'granted'|'denied'|'unsupported'`. `<Webcam onUserMedia={...} onUserMediaError={...}/>`. Pre-check `navigator.mediaDevices?.getUserMedia` for unsupported. Defensive cleanup on unmount.
- **`useSpeechSynthesis`**: feature-detect `"speechSynthesis" in window`; expose `speak()` and `cancel()`. Cancel on unmount + route change.
- **`useSpeechToText`** wrapping `react-hook-speech-to-text`: `continuous: false`, `useLegacyResults: false`. On `results` change, append to session-store transcript via functional setState. On `!isRecording && transcript.length > 10`, fire `useSubmitAnswer` mutation; reset transcript on success. Edge cases: Safari `webkitSpeechRecognition`, permission denial, mid-recording navigation.

## 9. Styling

- Tailwind config mirrors old app's HSL CSS variables (`--primary`, `--secondary`). Add `tailwindcss-animate`.
- shadcn primitives via `npx shadcn@latest add button card dialog input textarea collapsible toast`.
- Font pair: `Fraunces` (display) + `Inter Tight` (body) via `<link rel="preload">`.
- Tokens in `styles/tokens.css` as CSS vars; primary: deep navy `--primary: 222 47% 18%`; accent: warm amber.

## 10. Build & dev config

`vite.config.ts`:
```ts
defineConfig({
  plugins: [react()],
  resolve: { alias: { "@": fileURLToPath(new URL("./src", import.meta.url)) } },
  envPrefix: "VITE_",
  server: {
    port: 5173,
    proxy: { "/api": { target: "http://localhost:8080", changeOrigin: true } },
  },
  build: { target: "es2022", sourcemap: true },
});
```

`lib/env.ts` validates `import.meta.env` with zod, fails fast on boot.

Scripts: `dev`, `build` (`tsc -b && vite build`), `preview`, `lint`, `test`, `test:watch`, `test:coverage`, `typecheck`.

## 11. Testing

- **Vitest 1.x + jsdom + @testing-library/react + user-event**.
- `setup.ts` registers `@testing-library/jest-dom`, stubs `matchMedia`, `SpeechSynthesisUtterance`, `getUserMedia`.
- **Unit**: `api/interviews.ts` against MSW handlers; `lib/format.ts`; zustand selectors.
- **Component**: `AddInterviewDialog` (validation), `RecordAnswerControl` (mic toggle + mocked speech), `Feedback` (empty + populated), `ProtectedRoute` (mocked Clerk via `vi.mock("@clerk/clerk-react")`).
- **MSW** at fetch boundary; same handlers feed Storybook later.

## 12. package.json

Runtime: react@18.3, react-dom@18.3, react-router-dom@6.26, @tanstack/react-query@5, @tanstack/react-query-devtools, zustand@4, @clerk/clerk-react@5, react-hook-form@7, zod@3, @hookform/resolvers@3, react-webcam@7, react-hook-speech-to-text@0.8, class-variance-authority, clsx, tailwind-merge, lucide-react, tailwindcss-animate, @radix-ui/react-dialog, @radix-ui/react-collapsible, @radix-ui/react-slot, @radix-ui/react-toast, date-fns@3.

Dev: vite@5, @vitejs/plugin-react-swc, typescript@5.5, @types/react, @types/react-dom, @types/node, tailwindcss@3.4, postcss, autoprefixer, vitest@1, jsdom, @testing-library/react, @testing-library/jest-dom, @testing-library/user-event, msw@2, eslint, eslint-plugin-react, eslint-plugin-react-hooks, @typescript-eslint/parser, @typescript-eslint/eslint-plugin, prettier, prettier-plugin-tailwindcss.

Performance hygiene: direct icon imports, route-level code splitting via `React.lazy` + `Suspense` for `InterviewSession`/`Feedback`, no inline component definitions, primitive deps in effects, `useTransition` on question-pill switching.

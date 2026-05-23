import { http, HttpResponse } from "msw";
import type {
  CoachReport,
  Interview,
  InterviewSummary,
  ListResponse,
  ResumeStatus,
  UserAnswer,
} from "@/types/api";

const BASE = "*/api/v1";

const SAMPLE_INTERVIEW: Interview = {
  mockId: "mock_test_1",
  jobPosition: "Senior Backend Engineer",
  jobDescription: "Go services with Postgres",
  yearsExperience: 5,
  questions: [
    { question: "Tell me about yourself.", answer: "Sample answer." },
    { question: "Describe a hard bug.", answer: "Sample answer." },
    { question: "Postgres indexes?", answer: "Sample answer." },
    { question: "Distributed systems CAP?", answer: "Sample answer." },
    { question: "On-call processes?", answer: "Sample answer." },
  ],
  createdAt: "2025-04-01T10:00:00Z",
};

const SAMPLE_SUMMARY: InterviewSummary = {
  mockId: SAMPLE_INTERVIEW.mockId,
  jobPosition: SAMPLE_INTERVIEW.jobPosition,
  jobDescription: SAMPLE_INTERVIEW.jobDescription,
  yearsExperience: SAMPLE_INTERVIEW.yearsExperience,
  createdAt: SAMPLE_INTERVIEW.createdAt,
};

const SAMPLE_FEEDBACK: UserAnswer[] = [
  {
    questionIndex: 0,
    question: SAMPLE_INTERVIEW.questions[0]?.question ?? "",
    userAnswer: "I am a backend engineer.",
    correctAnswer: "Strong elevator pitch.",
    feedback: "Add metrics.",
    rating: 8,
    fillerCount: 2,
    wordsPerMinute: 140,
    longPauseCount: 1,
    createdAt: "2025-04-01T10:05:00Z",
  },
];

export const handlers = [
  http.get(`${BASE}/interviews`, () =>
    HttpResponse.json<ListResponse<InterviewSummary>>({ items: [SAMPLE_SUMMARY] }),
  ),
  http.get(`${BASE}/interviews/:mockId`, ({ params }) => {
    if (params["mockId"] !== SAMPLE_INTERVIEW.mockId) {
      return HttpResponse.json({ error: { code: "NOT_FOUND", message: "Not found" } }, { status: 404 });
    }
    return HttpResponse.json(SAMPLE_INTERVIEW);
  }),
  http.post(`${BASE}/interviews`, async ({ request }) => {
    const body = (await request.json()) as Partial<Interview>;
    return HttpResponse.json<Interview>(
      {
        ...SAMPLE_INTERVIEW,
        mockId: "mock_test_new",
        jobPosition: typeof body.jobPosition === "string" ? body.jobPosition : SAMPLE_INTERVIEW.jobPosition,
        jobDescription:
          typeof body.jobDescription === "string" ? body.jobDescription : SAMPLE_INTERVIEW.jobDescription,
        yearsExperience:
          typeof body.yearsExperience === "number"
            ? body.yearsExperience
            : SAMPLE_INTERVIEW.yearsExperience,
      },
      { status: 201 },
    );
  }),
  http.post(`${BASE}/interviews/:mockId/answers`, async ({ request }) => {
    const payload = (await request.json()) as {
      questionIndex: number;
      userAnswer: string;
      fillerCount?: number;
      wordsPerMinute?: number;
      longPauseCount?: number;
    };
    return HttpResponse.json<UserAnswer>(
      {
        questionIndex: payload.questionIndex,
        question: SAMPLE_INTERVIEW.questions[payload.questionIndex]?.question ?? "Q",
        correctAnswer: "Reference answer.",
        userAnswer: payload.userAnswer,
        feedback: "Solid response.",
        rating: 7,
        fillerCount: payload.fillerCount ?? 0,
        wordsPerMinute: payload.wordsPerMinute ?? 0,
        longPauseCount: payload.longPauseCount ?? 0,
        createdAt: new Date().toISOString(),
      },
      { status: 201 },
    );
  }),
  http.get(`${BASE}/interviews/:mockId/feedback`, () =>
    HttpResponse.json<ListResponse<UserAnswer>>({ items: SAMPLE_FEEDBACK }),
  ),
  http.post(`${BASE}/interviews/:mockId/transcribe`, () =>
    HttpResponse.json({
      transcript: "um so I built it",
      analysis: SAMPLE_ANALYSIS,
    }),
  ),
  http.get(`${BASE}/resume`, () =>
    HttpResponse.json<ResumeStatus>({ attached: false, uploadedAt: null }),
  ),
  http.post(`${BASE}/resume`, () =>
    HttpResponse.json<ResumeStatus>({
      attached: true,
      uploadedAt: "2026-05-15T10:00:00Z",
    }),
  ),
  http.delete(`${BASE}/resume`, () => new HttpResponse(null, { status: 204 })),
  // Coach report endpoints. Default handlers cover the happy path; tests
  // that need 404/error variants override via server.use(...) per test.
  http.get(`${BASE}/interviews/:mockId/coach-report`, ({ params }) => {
    if (params["mockId"] !== SAMPLE_INTERVIEW.mockId) {
      return HttpResponse.json(
        { error: { code: "not_found", message: "resource not found" } },
        { status: 404 },
      );
    }
    return HttpResponse.json<CoachReport>(SAMPLE_COACH_REPORT);
  }),
  http.post(`${BASE}/interviews/:mockId/coach-report`, () =>
    HttpResponse.json<CoachReport>(SAMPLE_COACH_REPORT, { status: 201 }),
  ),
  http.post(`${BASE}/interviews/:mockId/coach-report/stream`, () => {
    // Build a minimal SSE response stream: two chunk events then done.
    const encoder = new TextEncoder();
    const body = new ReadableStream({
      start(controller) {
        controller.enqueue(encoder.encode(`event: chunk\ndata: {"text":"## Overall Performance\\n"}\n\n`));
        controller.enqueue(encoder.encode(`event: chunk\ndata: {"text":"streamed body"}\n\n`));
        controller.enqueue(
          encoder.encode(
            `event: done\ndata: {"mockId":"${SAMPLE_INTERVIEW.mockId}","tokensUsed":1500,"model":"gemini-test","createdAt":"2026-05-23T10:00:00Z"}\n\n`,
          ),
        );
        controller.close();
      },
    });
    return new HttpResponse(body, {
      headers: {
        "Content-Type": "text/event-stream",
        "Cache-Control": "no-cache",
      },
    });
  }),
];

const SAMPLE_ANALYSIS = {
  fillerCount: 2,
  wordsPerMinute: 145,
  longPauseCount: 1,
};

const SAMPLE_COACH_REPORT: CoachReport = {
  mockId: SAMPLE_INTERVIEW.mockId,
  content: "## Overall Performance\nYou did well on the backend questions.",
  tokensUsed: 1500,
  model: "gemini-test",
  createdAt: "2026-05-23T10:00:00Z",
};

export { SAMPLE_INTERVIEW, SAMPLE_SUMMARY, SAMPLE_FEEDBACK, SAMPLE_ANALYSIS, SAMPLE_COACH_REPORT };

import { http, HttpResponse } from "msw";
import type {
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
];

const SAMPLE_ANALYSIS = {
  fillerCount: 2,
  wordsPerMinute: 145,
  longPauseCount: 1,
};

export { SAMPLE_INTERVIEW, SAMPLE_SUMMARY, SAMPLE_FEEDBACK, SAMPLE_ANALYSIS };

// @vitest-environment node
//
// Run this API-client test file in Node, not jsdom. The streamCoachReport
// tests use AbortController, and jsdom ships a browser-shaped AbortSignal
// that undici's fetch (msw's backend) rejects with "Expected signal to be
// an instance of AbortSignal". The API client never touches the DOM, so
// running its tests in Node is safe and avoids the polyfill mismatch.
import { describe, expect, it, vi } from "vitest";
import { http, HttpResponse } from "msw";
import {
  createInterview,
  generateCoachReport,
  getCoachReport,
  getFeedback,
  getInterview,
  listInterviews,
  streamCoachReport,
  submitAnswer,
  transcribeAudio,
} from "./interviews";
import {
  SAMPLE_ANALYSIS,
  SAMPLE_COACH_REPORT,
  SAMPLE_INTERVIEW,
  SAMPLE_SUMMARY,
} from "@/test/msw/handlers";
import { server } from "@/test/setup";

const BASE = "*/api/v1";

describe("api/interviews against MSW", () => {
  it("listInterviews unwraps the items array", async () => {
    const result = await listInterviews();
    expect(result).toEqual([SAMPLE_SUMMARY]);
  });

  it("getInterview returns the full interview", async () => {
    const interview = await getInterview(SAMPLE_INTERVIEW.mockId);
    expect(interview.mockId).toBe(SAMPLE_INTERVIEW.mockId);
    expect(interview.questions).toHaveLength(5);
  });

  it("getInterview throws on 404", async () => {
    await expect(getInterview("missing")).rejects.toThrow();
  });

  it("createInterview echoes input fields", async () => {
    const created = await createInterview({
      jobPosition: "Frontend Engineer",
      jobDescription: "React, accessibility, performance",
      yearsExperience: 3,
    });
    expect(created.jobPosition).toBe("Frontend Engineer");
    expect(created.yearsExperience).toBe(3);
    expect(created.mockId).toBe("mock_test_new");
  });

  it("submitAnswer returns rating as a number", async () => {
    const answer = await submitAnswer(SAMPLE_INTERVIEW.mockId, {
      questionIndex: 0,
      userAnswer: "Detailed answer.",
      fillerCount: 0,
      wordsPerMinute: 0,
      longPauseCount: 0,
    });
    expect(typeof answer.rating).toBe("number");
    expect(answer.rating).toBeGreaterThanOrEqual(1);
    expect(answer.rating).toBeLessThanOrEqual(10);
  });

  it("getFeedback returns ordered items", async () => {
    const items = await getFeedback(SAMPLE_INTERVIEW.mockId);
    expect(items.length).toBeGreaterThan(0);
    expect(items[0]?.questionIndex).toBe(0);
  });

  it("transcribeAudio returns transcript plus delivery analysis", async () => {
    const fakeAudio = new Blob(["fake"], { type: "audio/webm" });
    const result = await transcribeAudio(SAMPLE_INTERVIEW.mockId, fakeAudio, 1);
    expect(typeof result.transcript).toBe("string");
    expect(result.transcript.length).toBeGreaterThan(0);
    expect(result.analysis).toEqual(SAMPLE_ANALYSIS);
  });

  it("getCoachReport returns the cached report", async () => {
    const report = await getCoachReport(SAMPLE_INTERVIEW.mockId);
    expect(report).toEqual(SAMPLE_COACH_REPORT);
  });

  it("getCoachReport throws ApiError on 404", async () => {
    await expect(getCoachReport("missing")).rejects.toMatchObject({ status: 404 });
  });

  it("generateCoachReport POSTs and returns the persisted report", async () => {
    const report = await generateCoachReport(SAMPLE_INTERVIEW.mockId);
    expect(report.mockId).toBe(SAMPLE_INTERVIEW.mockId);
    expect(report.content).toContain("Overall Performance");
  });
});

describe("streamCoachReport SSE parsing", () => {
  it("dispatches chunk events in order, then done", async () => {
    const chunks: string[] = [];
    const onDone = vi.fn();
    const onError = vi.fn();

    const ctrl = new AbortController();
    await streamCoachReport(SAMPLE_INTERVIEW.mockId, ctrl.signal, {
      onChunk: (t) => chunks.push(t),
      onDone,
      onError,
    });

    expect(chunks).toEqual(["## Overall Performance\n", "streamed body"]);
    expect(onError).not.toHaveBeenCalled();
    expect(onDone).toHaveBeenCalledTimes(1);
    expect(onDone.mock.calls[0]?.[0]).toMatchObject({
      mockId: SAMPLE_INTERVIEW.mockId,
      tokensUsed: 1500,
      model: "gemini-test",
    });
  });

  it("surfaces mid-stream error events via onError", async () => {
    server.use(
      http.post(`${BASE}/interviews/:mockId/coach-report/stream`, () => {
        const encoder = new TextEncoder();
        const body = new ReadableStream({
          start(controller) {
            controller.enqueue(encoder.encode(`event: chunk\ndata: {"text":"## Overall"}\n\n`));
            controller.enqueue(
              encoder.encode(`event: error\ndata: {"code":"llm_failure","message":"upstream model error"}\n\n`),
            );
            controller.close();
          },
        });
        return new HttpResponse(body, {
          headers: { "Content-Type": "text/event-stream" },
        });
      }),
    );

    const chunks: string[] = [];
    const onDone = vi.fn();
    const onError = vi.fn();
    const ctrl = new AbortController();

    await streamCoachReport(SAMPLE_INTERVIEW.mockId, ctrl.signal, {
      onChunk: (t) => chunks.push(t),
      onDone,
      onError,
    });

    expect(chunks).toEqual(["## Overall"]);
    expect(onDone).not.toHaveBeenCalled();
    expect(onError).toHaveBeenCalledWith({ code: "llm_failure", message: "upstream model error" });
  });

  it("surfaces pre-stream HTTP errors as a single onError without onChunk", async () => {
    server.use(
      http.post(`${BASE}/interviews/:mockId/coach-report/stream`, () =>
        HttpResponse.json(
          { error: { code: "validation_failed", message: "no answers" } },
          { status: 400 },
        ),
      ),
    );

    const onChunk = vi.fn();
    const onDone = vi.fn();
    const onError = vi.fn();
    const ctrl = new AbortController();

    await streamCoachReport(SAMPLE_INTERVIEW.mockId, ctrl.signal, {
      onChunk,
      onDone,
      onError,
    });

    expect(onChunk).not.toHaveBeenCalled();
    expect(onDone).not.toHaveBeenCalled();
    expect(onError).toHaveBeenCalledWith({
      code: "validation_failed",
      message: "no answers",
    });
  });

  it("returns silently when aborted before any bytes arrive", async () => {
    server.use(
      http.post(`${BASE}/interviews/:mockId/coach-report/stream`, async () => {
        // Hang long enough for the abort to land before the response starts.
        await new Promise((resolve) => setTimeout(resolve, 50));
        return new HttpResponse("", {
          headers: { "Content-Type": "text/event-stream" },
        });
      }),
    );

    const onChunk = vi.fn();
    const onDone = vi.fn();
    const onError = vi.fn();
    const ctrl = new AbortController();
    ctrl.abort();

    await streamCoachReport(SAMPLE_INTERVIEW.mockId, ctrl.signal, {
      onChunk,
      onDone,
      onError,
    });

    // Abort = user intent, not failure. None of the callbacks should fire.
    expect(onChunk).not.toHaveBeenCalled();
    expect(onDone).not.toHaveBeenCalled();
    expect(onError).not.toHaveBeenCalled();
  });
});

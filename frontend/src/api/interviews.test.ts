import { describe, expect, it } from "vitest";
import {
  createInterview,
  getFeedback,
  getInterview,
  listInterviews,
  submitAnswer,
  transcribeAudio,
} from "./interviews";
import { SAMPLE_ANALYSIS, SAMPLE_INTERVIEW, SAMPLE_SUMMARY } from "@/test/msw/handlers";

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
});

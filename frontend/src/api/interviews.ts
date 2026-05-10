import { apiFetch } from "./client";
import type {
  CreateInterviewInput,
  Interview,
  InterviewSummary,
  ListResponse,
  SubmitAnswerInput,
  UserAnswer,
} from "@/types/api";

export interface ListInterviewsParams {
  limit?: number;
  cursor?: string;
}

export async function listInterviews(
  params: ListInterviewsParams = {},
  signal?: AbortSignal,
): Promise<InterviewSummary[]> {
  const search = new URLSearchParams();
  if (params.limit !== undefined) search.set("limit", String(params.limit));
  if (params.cursor !== undefined) search.set("cursor", params.cursor);
  const query = search.toString();
  const path = query ? `/interviews?${query}` : "/interviews";
  const init = signal ? { signal } : {};
  const response = await apiFetch<ListResponse<InterviewSummary>>(path, init);
  return response.items;
}

export async function getInterview(mockId: string, signal?: AbortSignal): Promise<Interview> {
  const init = signal ? { signal } : {};
  return apiFetch<Interview>(`/interviews/${encodeURIComponent(mockId)}`, init);
}

export async function createInterview(input: CreateInterviewInput): Promise<Interview> {
  return apiFetch<Interview>("/interviews", {
    method: "POST",
    body: input,
  });
}

export async function submitAnswer(
  mockId: string,
  payload: SubmitAnswerInput,
): Promise<UserAnswer> {
  return apiFetch<UserAnswer>(`/interviews/${encodeURIComponent(mockId)}/answers`, {
    method: "POST",
    body: payload,
  });
}

export interface TranscribeAudioResponse {
  transcript: string;
}

export async function transcribeAudio(
  mockId: string,
  audio: Blob,
  signal?: AbortSignal,
): Promise<TranscribeAudioResponse> {
  const form = new FormData();
  // Filename is required by some servers' multipart parsers; the extension
  // hints the container but the backend trusts the blob's MIME type.
  const ext = audio.type.includes("webm")
    ? "webm"
    : audio.type.includes("ogg")
      ? "ogg"
      : audio.type.includes("mp4")
        ? "m4a"
        : audio.type.includes("wav")
          ? "wav"
          : "bin";
  form.append("audio", audio, `answer.${ext}`);
  const init: Parameters<typeof apiFetch>[1] = { method: "POST", body: form };
  if (signal) init.signal = signal;
  return apiFetch<TranscribeAudioResponse>(
    `/interviews/${encodeURIComponent(mockId)}/transcribe`,
    init,
  );
}

export async function getFeedback(
  mockId: string,
  signal?: AbortSignal,
): Promise<UserAnswer[]> {
  const init = signal ? { signal } : {};
  const response = await apiFetch<ListResponse<UserAnswer>>(
    `/interviews/${encodeURIComponent(mockId)}/feedback`,
    init,
  );
  return response.items;
}

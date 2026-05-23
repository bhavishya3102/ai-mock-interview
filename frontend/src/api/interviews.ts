import { apiFetch, ApiError } from "./client";
import { env } from "@/lib/env";
import { getAuthToken } from "@/lib/tokenStore";
import type {
  ApiErrorBody,
  CoachReport,
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

export interface SpeechAnalysis {
  fillerCount: number;
  wordsPerMinute: number;
  longPauseCount: number;
}

export interface TranscribeAudioResponse {
  transcript: string;
  analysis: SpeechAnalysis;
}

export async function transcribeAudio(
  mockId: string,
  audio: Blob,
  longPauseCount: number,
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
  form.append("longPauseCount", String(Math.max(0, Math.floor(longPauseCount))));
  const init: Parameters<typeof apiFetch>[1] = { method: "POST", body: form };
  if (signal) init.signal = signal;
  return apiFetch<TranscribeAudioResponse>(
    `/interviews/${encodeURIComponent(mockId)}/transcribe`,
    init,
  );
}

export interface FollowUpTurn {
  question: string;
  answer: string;
}

export interface JudgeFollowUpInput {
  questionIndex: number;
  mainAnswer: string;
  priorTurns: FollowUpTurn[];
}

export interface JudgeFollowUpResponse {
  followUp: string;
}

export async function judgeFollowUp(
  mockId: string,
  payload: JudgeFollowUpInput,
): Promise<JudgeFollowUpResponse> {
  return apiFetch<JudgeFollowUpResponse>(
    `/interviews/${encodeURIComponent(mockId)}/follow-up`,
    { method: "POST", body: payload },
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

export async function getCoachReport(
  mockId: string,
  signal?: AbortSignal,
): Promise<CoachReport> {
  const init = signal ? { signal } : {};
  return apiFetch<CoachReport>(`/interviews/${encodeURIComponent(mockId)}/coach-report`, init);
}

export async function generateCoachReport(mockId: string): Promise<CoachReport> {
  return apiFetch<CoachReport>(`/interviews/${encodeURIComponent(mockId)}/coach-report`, {
    method: "POST",
  });
}

export interface CoachReportDoneEvent {
  mockId: string;
  tokensUsed: number;
  model: string;
  createdAt: string;
}

export interface CoachReportErrorEvent {
  code: string;
  message: string;
}

export interface CoachReportStreamCallbacks {
  onChunk: (text: string) => void;
  onDone: (event: CoachReportDoneEvent) => void;
  onError: (event: CoachReportErrorEvent) => void;
}

// streamCoachReport opens an SSE stream to the coach-report streaming
// endpoint and dispatches parsed events to the provided callbacks. The
// caller controls cancellation through signal.
//
// EventSource is not usable here because it cannot send the Authorization
// header that Clerk requires; we fall back to fetch + ReadableStream and
// parse the SSE wire format ourselves.
//
// Behaviour:
//   - Pre-stream HTTP errors (e.g. 401, 404, 400 from the backend's plain
//     JSON envelope) are converted to a single onError call.
//   - Each `event: chunk` is parsed and forwarded to onChunk.
//   - The terminal `event: done` or `event: error` resolves the function;
//     the caller does not need to subscribe to the stream lifecycle
//     separately.
//   - An aborted signal resolves silently (the AbortError is swallowed)
//     since cancellation is a user action, not a failure.
export async function streamCoachReport(
  mockId: string,
  signal: AbortSignal,
  cb: CoachReportStreamCallbacks,
): Promise<void> {
  const token = await getAuthToken();
  const headers = new Headers();
  headers.set("Accept", "text/event-stream");
  if (token) headers.set("Authorization", `Bearer ${token}`);

  let response: Response;
  try {
    response = await fetch(
      `${env.VITE_API_BASE_URL}/interviews/${encodeURIComponent(mockId)}/coach-report/stream`,
      { method: "POST", headers, signal },
    );
  } catch (err) {
    if (signal.aborted) return;
    cb.onError({ code: "NETWORK_ERROR", message: err instanceof Error ? err.message : String(err) });
    return;
  }

  if (!response.ok) {
    // Pre-stream failure: server returned a JSON error envelope on a
    // non-200 status. Parse it and surface as a single onError.
    let envelope: ApiErrorBody | null = null;
    try {
      envelope = (await response.json()) as ApiErrorBody;
    } catch {
      // ignore — fall through to a generic error
    }
    cb.onError({
      code: envelope?.error.code ?? `HTTP_${response.status}`,
      message: envelope?.error.message ?? response.statusText ?? "Request failed",
    });
    return;
  }

  if (!response.body) {
    cb.onError({ code: "NO_STREAM", message: "Response body is empty" });
    return;
  }

  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";

  try {
    for (;;) {
      const { done, value } = await reader.read();
      if (done) return;
      buffer += decoder.decode(value, { stream: true });

      // SSE events are delimited by a blank line ("\n\n"). Anything after
      // the last delimiter is a partial event and stays in the buffer
      // until more bytes arrive.
      const blocks = buffer.split("\n\n");
      buffer = blocks.pop() ?? "";

      for (const block of blocks) {
        if (!block.trim()) continue;
        const parsed = parseSseBlock(block);
        if (!parsed) continue;
        dispatchSseEvent(parsed.event, parsed.data, cb);
      }
    }
  } catch (err) {
    if (signal.aborted) return;
    cb.onError({ code: "STREAM_ERROR", message: err instanceof Error ? err.message : String(err) });
  } finally {
    try {
      reader.releaseLock();
    } catch {
      // releaseLock throws if a read is in flight when the stream is
      // cancelled; safe to swallow.
    }
  }
}

interface ParsedSseBlock {
  event: string;
  data: string;
}

function parseSseBlock(block: string): ParsedSseBlock | null {
  let event = "";
  const dataLines: string[] = [];
  for (const line of block.split("\n")) {
    if (line.startsWith("event: ")) {
      event = line.slice(7).trim();
    } else if (line.startsWith("data: ")) {
      dataLines.push(line.slice(6));
    }
  }
  if (!event) return null;
  return { event, data: dataLines.join("\n") };
}

function dispatchSseEvent(event: string, data: string, cb: CoachReportStreamCallbacks): void {
  let parsed: unknown;
  try {
    parsed = JSON.parse(data);
  } catch {
    // Malformed event data — drop silently rather than killing the stream.
    return;
  }
  if (typeof parsed !== "object" || parsed === null) return;

  switch (event) {
    case "chunk": {
      const text = (parsed as { text?: unknown }).text;
      if (typeof text === "string") cb.onChunk(text);
      return;
    }
    case "done":
      cb.onDone(parsed as CoachReportDoneEvent);
      return;
    case "error":
      cb.onError(parsed as CoachReportErrorEvent);
      return;
    default:
      // Unknown event type — ignore, do not break the stream.
      return;
  }
}

// isNotFoundError is a small helper for hooks that treat 404 as "no
// coach report yet" rather than a failure to surface.
export function isNotFoundError(err: unknown): boolean {
  return err instanceof ApiError && err.status === 404;
}

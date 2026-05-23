import { useCallback, useEffect, useRef, useState } from "react";
import { useQuery, useQueryClient, type UseQueryResult } from "@tanstack/react-query";
import {
  getCoachReport,
  isNotFoundError,
  streamCoachReport,
  type CoachReportDoneEvent,
  type CoachReportErrorEvent,
} from "@/api/interviews";
import { queryKeys } from "@/api/queryKeys";
import type { CoachReport } from "@/types/api";

// useCoachReport queries the cached coach report for an interview. A 404
// from the backend means "not generated yet" and is normalised to
// `data: undefined` so consumers can treat the no-report state as a
// regular conditional render rather than an error.
export function useCoachReport(
  mockId: string | undefined,
): UseQueryResult<CoachReport | undefined, Error> {
  return useQuery({
    queryKey: queryKeys.interviews.coachReport(mockId ?? "__none__"),
    queryFn: async ({ signal }) => {
      if (!mockId) throw new Error("mockId is required");
      try {
        return await getCoachReport(mockId, signal);
      } catch (err) {
        if (isNotFoundError(err)) return undefined;
        throw err;
      }
    },
    enabled: Boolean(mockId),
    staleTime: 5 * 60_000,
  });
}

export type StreamingState = "idle" | "streaming" | "done" | "error";

export interface StreamingCoachReport {
  state: StreamingState;
  /** Accumulated markdown text from streamed chunks (and from cached hits
   *  which arrive as a single chunk). */
  text: string;
  /** Populated on the terminal `done` event. */
  metadata: CoachReportDoneEvent | null;
  /** Populated on the terminal `error` event or on transport failure. */
  error: CoachReportErrorEvent | null;
  /** Open the stream. Resets prior text/state. No-op if mockId missing. */
  start: () => void;
  /** Abort the in-flight stream. Transitions back to idle (not error). */
  cancel: () => void;
}

// useStreamingCoachReport drives the SSE flow. State is local to the hook
// so the consumer can render a typewriter effect by binding `text` to a
// markdown renderer. On the terminal `done` event we invalidate the
// cached-coach-report query so a subsequent refresh hits the persisted
// row instead of regenerating.
export function useStreamingCoachReport(mockId: string | undefined): StreamingCoachReport {
  const queryClient = useQueryClient();
  const [state, setState] = useState<StreamingState>("idle");
  const [text, setText] = useState<string>("");
  const [metadata, setMetadata] = useState<CoachReportDoneEvent | null>(null);
  const [error, setError] = useState<CoachReportErrorEvent | null>(null);
  const abortRef = useRef<AbortController | null>(null);

  // Unmount cleanup: cancel any in-flight stream. Without this an
  // unmounted component would keep updating state when chunks arrive.
  useEffect(() => {
    return () => {
      abortRef.current?.abort();
    };
  }, []);

  const start = useCallback(() => {
    if (!mockId) return;
    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;

    setState("streaming");
    setText("");
    setMetadata(null);
    setError(null);

    void streamCoachReport(mockId, controller.signal, {
      onChunk: (chunk) => {
        if (controller.signal.aborted) return;
        setText((prev) => prev + chunk);
      },
      onDone: (event) => {
        if (controller.signal.aborted) return;
        setMetadata(event);
        setState("done");
        // Make the just-persisted report available to useCoachReport so
        // navigation back to this page hits the cache, not the network.
        void queryClient.invalidateQueries({
          queryKey: queryKeys.interviews.coachReport(mockId),
        });
      },
      onError: (event) => {
        if (controller.signal.aborted) return;
        setError(event);
        setState("error");
      },
    });
  }, [mockId, queryClient]);

  const cancel = useCallback(() => {
    abortRef.current?.abort();
    abortRef.current = null;
    setState("idle");
  }, []);

  return { state, text, metadata, error, start, cancel };
}

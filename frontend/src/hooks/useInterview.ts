import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import { getFeedback, getInterview } from "@/api/interviews";
import { queryKeys } from "@/api/queryKeys";
import type { Interview, UserAnswer } from "@/types/api";

export function useInterview(
  mockId: string | undefined,
): UseQueryResult<Interview, Error> {
  return useQuery({
    queryKey: queryKeys.interviews.detail(mockId ?? "__none__"),
    queryFn: ({ signal }) => {
      if (!mockId) throw new Error("mockId is required");
      return getInterview(mockId, signal);
    },
    enabled: Boolean(mockId),
    staleTime: 5 * 60_000,
  });
}

export function useInterviewFeedback(
  mockId: string | undefined,
): UseQueryResult<UserAnswer[], Error> {
  return useQuery({
    queryKey: queryKeys.interviews.feedback(mockId ?? "__none__"),
    queryFn: ({ signal }) => {
      if (!mockId) throw new Error("mockId is required");
      return getFeedback(mockId, signal);
    },
    enabled: Boolean(mockId),
    staleTime: 30_000,
  });
}

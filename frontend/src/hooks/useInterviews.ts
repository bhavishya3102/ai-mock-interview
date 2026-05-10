import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import { listInterviews } from "@/api/interviews";
import { queryKeys } from "@/api/queryKeys";
import type { InterviewSummary } from "@/types/api";

export function useInterviews(): UseQueryResult<InterviewSummary[], Error> {
  return useQuery({
    queryKey: queryKeys.interviews.all,
    queryFn: ({ signal }) => listInterviews({}, signal),
    staleTime: 60_000,
  });
}

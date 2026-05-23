import {
  useMutation,
  useQuery,
  useQueryClient,
  type UseMutationResult,
  type UseQueryResult,
} from "@tanstack/react-query";
import { deleteResume, getResumeStatus, uploadResume } from "@/api/resume";
import { queryKeys } from "@/api/queryKeys";
import type { ResumeStatus } from "@/types/api";

export function useResumeStatus(): UseQueryResult<ResumeStatus, Error> {
  return useQuery({
    queryKey: queryKeys.resume.status,
    queryFn: ({ signal }) => getResumeStatus(signal),
    staleTime: 60_000,
  });
}

export function useUploadResume(): UseMutationResult<ResumeStatus, Error, File> {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: uploadResume,
    onSuccess: (data) => {
      qc.setQueryData(queryKeys.resume.status, data);
    },
  });
}

export function useDeleteResume(): UseMutationResult<void, Error, void> {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: deleteResume,
    onSuccess: () => {
      qc.setQueryData<ResumeStatus>(queryKeys.resume.status, {
        attached: false,
        uploadedAt: null,
      });
    },
  });
}

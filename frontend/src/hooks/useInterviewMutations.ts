import { useMutation, useQueryClient, type UseMutationResult } from "@tanstack/react-query";
import {
  createInterview,
  judgeFollowUp,
  submitAnswer,
  transcribeAudio,
  type JudgeFollowUpInput,
  type JudgeFollowUpResponse,
  type TranscribeAudioResponse,
} from "@/api/interviews";
import { queryKeys } from "@/api/queryKeys";
import type { CreateInterviewInput, Interview, SubmitAnswerInput, UserAnswer } from "@/types/api";

export function useCreateInterview(): UseMutationResult<Interview, Error, CreateInterviewInput> {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: createInterview,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: queryKeys.interviews.all });
    },
  });
}

export interface SubmitAnswerArgs {
  mockId: string;
  payload: SubmitAnswerInput;
}

export function useSubmitAnswer(): UseMutationResult<UserAnswer, Error, SubmitAnswerArgs> {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ mockId, payload }: SubmitAnswerArgs) => submitAnswer(mockId, payload),
    onSuccess: (_data, variables) => {
      void qc.invalidateQueries({ queryKey: queryKeys.interviews.feedback(variables.mockId) });
    },
  });
}

export interface TranscribeAudioArgs {
  mockId: string;
  audio: Blob;
  longPauseCount: number;
}

export function useTranscribeAudio(): UseMutationResult<TranscribeAudioResponse, Error, TranscribeAudioArgs> {
  return useMutation({
    mutationFn: ({ mockId, audio, longPauseCount }: TranscribeAudioArgs) =>
      transcribeAudio(mockId, audio, longPauseCount),
  });
}

export interface JudgeFollowUpArgs {
  mockId: string;
  payload: JudgeFollowUpInput;
}

export function useJudgeFollowUp(): UseMutationResult<JudgeFollowUpResponse, Error, JudgeFollowUpArgs> {
  return useMutation({
    mutationFn: ({ mockId, payload }: JudgeFollowUpArgs) => judgeFollowUp(mockId, payload),
  });
}

export const queryKeys = {
  interviews: {
    all: ["interviews"] as const,
    detail: (mockId: string) => ["interviews", mockId] as const,
    feedback: (mockId: string) => ["interviews", mockId, "feedback"] as const,
    coachReport: (mockId: string) => ["interviews", mockId, "coach-report"] as const,
  },
  resume: {
    status: ["resume"] as const,
  },
} as const;

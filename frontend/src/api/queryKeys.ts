export const queryKeys = {
  interviews: {
    all: ["interviews"] as const,
    detail: (mockId: string) => ["interviews", mockId] as const,
    feedback: (mockId: string) => ["interviews", mockId, "feedback"] as const,
  },
} as const;

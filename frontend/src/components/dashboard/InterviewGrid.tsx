import type { ReactElement } from "react";
import { InterviewCard } from "./InterviewCard";
import type { InterviewSummary } from "@/types/api";

interface InterviewGridProps {
  interviews: InterviewSummary[];
}

export function InterviewGrid({ interviews }: InterviewGridProps): ReactElement {
  if (interviews.length === 0) {
    return (
      <div className="rounded-2xl border border-dashed border-border/80 bg-card/50 p-12 text-center">
        <p className="font-display text-2xl font-semibold tracking-tight">No interviews yet</p>
        <p className="mt-2 text-sm text-muted-foreground">
          Spin up your first session above. We&apos;ll generate the questions in seconds.
        </p>
      </div>
    );
  }

  return (
    <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
      {interviews.map((interview) => (
        <InterviewCard key={interview.mockId} interview={interview} />
      ))}
    </div>
  );
}

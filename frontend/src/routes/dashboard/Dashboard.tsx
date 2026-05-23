import type { ReactElement } from "react";
import { Loader2 } from "lucide-react";
import { PageContainer } from "@/components/layout/PageContainer";
import { AddInterviewDialog } from "@/components/dashboard/AddInterviewDialog";
import { InterviewGrid } from "@/components/dashboard/InterviewGrid";
import { ResumeCard } from "@/components/dashboard/ResumeCard";
import { useInterviews } from "@/hooks/useInterviews";

export default function Dashboard(): ReactElement {
  const { data, isLoading, isError, error } = useInterviews();

  return (
    <PageContainer>
      <header className="flex flex-col items-start justify-between gap-4 md:flex-row md:items-end">
        <div>
          <p className="text-xs font-medium uppercase tracking-[0.2em] text-muted-foreground">
            Your sessions
          </p>
          <h1 className="mt-2 font-display text-4xl font-semibold tracking-tight md:text-5xl">
            Mock interviews
          </h1>
        </div>
        <AddInterviewDialog />
      </header>

      <div className="mt-8">
        <ResumeCard />
      </div>

      <section className="mt-12">
        {isLoading ? (
          <div className="flex items-center gap-2 rounded-2xl border border-dashed bg-card/40 p-12 text-sm text-muted-foreground">
            <Loader2 className="h-4 w-4 animate-spin" /> Loading your interviews
          </div>
        ) : isError ? (
          <div className="rounded-2xl border border-destructive/40 bg-destructive/5 p-12 text-sm text-destructive">
            {error.message || "Could not load interviews."}
          </div>
        ) : (
          <InterviewGrid interviews={data ?? []} />
        )}
      </section>
    </PageContainer>
  );
}

import { useEffect, type ReactElement } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { Loader2 } from "lucide-react";
import { PageContainer } from "@/components/layout/PageContainer";
import { QuestionPanel } from "@/components/interview/QuestionPanel";
import { WebcamPanel } from "@/components/interview/WebcamPanel";
import { RecordAnswerControl } from "@/components/interview/RecordAnswerControl";
import { SessionFooterControls } from "@/components/interview/SessionFooterControls";
import { useInterview } from "@/hooks/useInterview";
import { useSessionStore } from "@/store/sessionStore";

export default function InterviewSession(): ReactElement {
  const { mockId } = useParams<{ mockId: string }>();
  const navigate = useNavigate();
  const { data, isLoading, isError, error } = useInterview(mockId);

  const activeIndex = useSessionStore((s) => s.activeIndex);
  const storedMockId = useSessionStore((s) => s.mockId);
  const setMockId = useSessionStore((s) => s.setMockId);
  const setActive = useSessionStore((s) => s.setActive);
  const reset = useSessionStore((s) => s.reset);

  useEffect(() => {
    if (!mockId) return;
    if (storedMockId !== mockId) {
      reset();
      setMockId(mockId);
    }
  }, [mockId, storedMockId, reset, setMockId]);

  useEffect(() => {
    return () => {
      if (typeof window !== "undefined" && "speechSynthesis" in window) {
        window.speechSynthesis.cancel();
      }
    };
  }, []);

  if (!mockId) {
    return (
      <PageContainer>
        <p>Missing interview id.</p>
      </PageContainer>
    );
  }

  if (isLoading) {
    return (
      <PageContainer>
        <div className="flex items-center gap-2 rounded-2xl border border-dashed bg-card/40 p-12 text-sm text-muted-foreground">
          <Loader2 className="h-4 w-4 animate-spin" /> Loading session
        </div>
      </PageContainer>
    );
  }

  if (isError || !data) {
    return (
      <PageContainer>
        <div className="rounded-2xl border border-destructive/40 bg-destructive/5 p-12 text-sm text-destructive">
          {error?.message || "Could not load interview."}
        </div>
      </PageContainer>
    );
  }

  const safeIndex = Math.min(activeIndex, data.questions.length - 1);

  return (
    <PageContainer>
      <header className="mb-8 flex items-end justify-between gap-4">
        <div>
          <p className="text-xs font-semibold uppercase tracking-[0.2em] text-muted-foreground">
            In session
          </p>
          <h1 className="mt-2 font-display text-3xl font-semibold tracking-tight md:text-4xl">
            {data.jobPosition}
          </h1>
        </div>
      </header>

      <div className="grid gap-6 md:grid-cols-2">
        <QuestionPanel questions={data.questions} activeIndex={safeIndex} onSelect={setActive} />
        <div className="flex flex-col gap-4">
          <WebcamPanel />
          <RecordAnswerControl mockId={mockId} questionIndex={safeIndex} />
        </div>
      </div>

      <div className="mt-8">
        <SessionFooterControls
          questionCount={data.questions.length}
          activeIndex={safeIndex}
          onPrev={() => setActive(Math.max(0, safeIndex - 1))}
          onNext={() => setActive(Math.min(data.questions.length - 1, safeIndex + 1))}
          onEnd={() => navigate(`/interview/${mockId}/feedback`)}
        />
      </div>
    </PageContainer>
  );
}

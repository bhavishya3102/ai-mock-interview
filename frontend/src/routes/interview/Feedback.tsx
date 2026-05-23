import { useState, type ReactElement, type ReactNode } from "react";
import { Link, useParams } from "react-router-dom";
import { ArrowLeft, Home, Loader2, MessageSquare, Sparkles, Trophy } from "lucide-react";
import { Button } from "@/components/ui/button";
import { PageContainer } from "@/components/layout/PageContainer";
import { FeedbackItem } from "@/components/interview/FeedbackItem";
import { CoachReportCard } from "@/components/interview/CoachReportCard";
import { EmptyFeedback } from "@/components/feedback/EmptyFeedback";
import { useInterviewFeedback } from "@/hooks/useInterview";
import { cn } from "@/lib/cn";

type FeedbackTab = "feedback" | "coach";

export default function Feedback(): ReactElement {
  const { mockId } = useParams<{ mockId: string }>();
  const { data, isLoading, isError, error } = useInterviewFeedback(mockId);
  // Tab state is local — tabs are a presentation concern. URL sync is
  // intentionally not wired so a refresh keeps the user on the default
  // (Feedback) tab; users land on the coach report intentionally.
  const [tab, setTab] = useState<FeedbackTab>("feedback");

  if (!mockId) {
    return (
      <PageContainer>
        <p>Missing interview id.</p>
      </PageContainer>
    );
  }

  const items = data ?? [];
  const average =
    items.length === 0 ? 0 : items.reduce((sum, item) => sum + item.rating, 0) / items.length;
  const strongCount = items.filter((i) => i.rating >= 8).length;
  const okCount = items.filter((i) => i.rating >= 5 && i.rating < 8).length;
  const weakCount = items.filter((i) => i.rating < 5).length;

  const summaryTone =
    average >= 8 ? "emerald" : average >= 5 ? "amber" : items.length === 0 ? "muted" : "rose";

  const hasItems = items.length > 0;

  return (
    <PageContainer>
      <Button asChild variant="ghost" size="sm" className="mb-6">
        <Link to="/dashboard">
          <ArrowLeft className="h-4 w-4" /> Back to dashboard
        </Link>
      </Button>

      <header className="flex flex-col gap-2 md:flex-row md:items-end md:justify-between">
        <div>
          <p className="text-xs font-semibold uppercase tracking-[0.2em] text-muted-foreground">
            Review
          </p>
          <h1 className="mt-2 font-display text-4xl font-semibold tracking-tight md:text-5xl">
            How you did
          </h1>
          <p className="mt-3 max-w-xl text-sm text-muted-foreground">
            Per-question breakdown plus a personalised coach report that synthesises everything into
            one narrative.
          </p>
        </div>
        <Button asChild variant="ghost">
          <Link to="/">
            <Home className="h-4 w-4" /> Go to home
          </Link>
        </Button>
      </header>

      {hasItems ? (
        <SummaryCard
          average={average}
          total={items.length}
          strong={strongCount}
          ok={okCount}
          weak={weakCount}
          tone={summaryTone}
        />
      ) : null}

      {hasItems ? (
        <TabBar tab={tab} onSelect={setTab} />
      ) : null}

      {/* When there are no answers yet there's no useful content for the
          coach tab — collapse the page back to the original single-section
          layout and skip tabs entirely. */}
      {!hasItems ? (
        <section className="mt-8 space-y-3">
          {isLoading ? (
            <div className="flex items-center gap-2 rounded-2xl border border-dashed bg-card/40 p-12 text-sm text-muted-foreground">
              <Loader2 className="h-4 w-4 animate-spin" /> Loading feedback
            </div>
          ) : isError ? (
            <div className="rounded-2xl border border-destructive/40 bg-destructive/5 p-12 text-sm text-destructive">
              {error.message || "Could not load feedback."}
            </div>
          ) : (
            <EmptyFeedback mockId={mockId} />
          )}
        </section>
      ) : tab === "feedback" ? (
        <section role="tabpanel" aria-labelledby="tab-feedback" className="mt-6 space-y-3">
          {items.map((item) => (
            <FeedbackItem key={item.questionIndex} item={item} />
          ))}
        </section>
      ) : (
        <section role="tabpanel" aria-labelledby="tab-coach" className="mt-6">
          <CoachReportCard mockId={mockId} />
        </section>
      )}
    </PageContainer>
  );
}

interface TabBarProps {
  tab: FeedbackTab;
  onSelect: (tab: FeedbackTab) => void;
}

function TabBar({ tab, onSelect }: TabBarProps): ReactElement {
  return (
    <div role="tablist" aria-label="Review sections" className="mt-8 border-b">
      <nav className="-mb-px flex gap-1 overflow-x-auto">
        <TabButton id="tab-feedback" active={tab === "feedback"} onClick={() => onSelect("feedback")}>
          <MessageSquare className="h-4 w-4" /> Feedback
        </TabButton>
        <TabButton id="tab-coach" active={tab === "coach"} onClick={() => onSelect("coach")}>
          <Sparkles className="h-4 w-4" /> AI Coach Report
        </TabButton>
      </nav>
    </div>
  );
}

interface TabButtonProps {
  id: string;
  active: boolean;
  onClick: () => void;
  children: ReactNode;
}

function TabButton({ id, active, onClick, children }: TabButtonProps): ReactElement {
  return (
    <button
      id={id}
      role="tab"
      type="button"
      aria-selected={active}
      onClick={onClick}
      className={cn(
        "inline-flex items-center gap-2 whitespace-nowrap border-b-2 px-4 py-3 text-sm font-medium transition-colors",
        active
          ? "border-primary text-foreground"
          : "border-transparent text-muted-foreground hover:text-foreground",
      )}
    >
      {children}
    </button>
  );
}

interface SummaryCardProps {
  average: number;
  total: number;
  strong: number;
  ok: number;
  weak: number;
  tone: "emerald" | "amber" | "rose" | "muted";
}

function SummaryCard({ average, total, strong, ok, weak, tone }: SummaryCardProps): ReactElement {
  const ringColor =
    tone === "emerald"
      ? "stroke-emerald-500"
      : tone === "amber"
        ? "stroke-amber-500"
        : tone === "rose"
          ? "stroke-rose-500"
          : "stroke-muted-foreground";

  const textColor =
    tone === "emerald"
      ? "text-emerald-700 dark:text-emerald-300"
      : tone === "amber"
        ? "text-amber-700 dark:text-amber-300"
        : tone === "rose"
          ? "text-rose-700 dark:text-rose-300"
          : "text-muted-foreground";

  const verdict =
    tone === "emerald"
      ? "Solid performance"
      : tone === "amber"
        ? "Room to improve"
        : tone === "rose"
          ? "Needs significant work"
          : "Awaiting answers";

  const radius = 32;
  const circumference = 2 * Math.PI * radius;
  const progress = Math.max(0, Math.min(1, average / 10));
  const dash = circumference * progress;

  return (
    <div className="mt-8 grid gap-4 rounded-2xl border bg-card p-5 shadow-sm md:grid-cols-[auto_1fr_auto] md:items-center md:gap-6 md:p-6">
      <div className="relative flex h-24 w-24 shrink-0 items-center justify-center">
        <svg viewBox="0 0 80 80" className="h-24 w-24 -rotate-90">
          <circle cx="40" cy="40" r={radius} className="fill-none stroke-muted" strokeWidth={6} />
          <circle
            cx="40"
            cy="40"
            r={radius}
            className={cn("fill-none transition-all", ringColor)}
            strokeWidth={6}
            strokeLinecap="round"
            strokeDasharray={`${dash} ${circumference}`}
          />
        </svg>
        <div className="absolute inset-0 flex flex-col items-center justify-center">
          <span className={cn("font-display text-2xl font-semibold leading-none tabular-nums", textColor)}>
            {average.toFixed(1)}
          </span>
          <span className="text-[10px] font-medium uppercase tracking-[0.16em] text-muted-foreground">
            / 10
          </span>
        </div>
      </div>

      <div className="min-w-0">
        <div className="flex items-center gap-2">
          <Trophy className={cn("h-4 w-4", textColor)} />
          <p className={cn("text-sm font-semibold", textColor)}>{verdict}</p>
        </div>
        <p className="mt-1 text-sm text-muted-foreground">
          Average across {total} answer{total === 1 ? "" : "s"}.
        </p>
        <div className="mt-3 flex flex-wrap items-center gap-3 text-xs">
          <Stat label="Strong" value={strong} dotClass="bg-emerald-500" />
          <Stat label="Needs work" value={ok} dotClass="bg-amber-500" />
          <Stat label="Off track" value={weak} dotClass="bg-rose-500" />
        </div>
      </div>
    </div>
  );
}

function Stat({
  label,
  value,
  dotClass,
}: {
  label: string;
  value: number;
  dotClass: string;
}): ReactElement {
  return (
    <span className="inline-flex items-center gap-1.5 rounded-full bg-secondary/60 px-2.5 py-1 text-foreground">
      <span className={cn("h-1.5 w-1.5 rounded-full", dotClass)} />
      <span className="font-semibold tabular-nums">{value}</span>
      <span className="text-muted-foreground">{label}</span>
    </span>
  );
}

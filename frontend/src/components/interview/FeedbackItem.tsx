import type { ReactElement } from "react";
import { ChevronDown, Gauge, MessageSquare, PauseCircle, Sparkles, User, Wind } from "lucide-react";
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible";
import { cn } from "@/lib/cn";
import type { UserAnswer } from "@/types/api";

interface FeedbackItemProps {
  item: UserAnswer;
}

interface RatingStyle {
  badge: string;
  bar: string;
  label: string;
  text: string;
}

function ratingStyle(rating: number): RatingStyle {
  if (rating >= 8) {
    return {
      badge: "bg-emerald-50 text-emerald-700 ring-emerald-200 dark:bg-emerald-500/10 dark:text-emerald-300 dark:ring-emerald-500/30",
      bar: "bg-emerald-500",
      label: "Strong",
      text: "text-emerald-700 dark:text-emerald-300",
    };
  }
  if (rating >= 5) {
    return {
      badge: "bg-amber-50 text-amber-700 ring-amber-200 dark:bg-amber-500/10 dark:text-amber-300 dark:ring-amber-500/30",
      bar: "bg-amber-500",
      label: "Needs work",
      text: "text-amber-700 dark:text-amber-300",
    };
  }
  return {
    badge: "bg-rose-50 text-rose-700 ring-rose-200 dark:bg-rose-500/10 dark:text-rose-300 dark:ring-rose-500/30",
    bar: "bg-rose-500",
    label: "Off track",
    text: "text-rose-700 dark:text-rose-300",
  };
}

export function FeedbackItem({ item }: FeedbackItemProps): ReactElement {
  const tone = ratingStyle(item.rating);
  const ratingPercent = Math.max(0, Math.min(100, (item.rating / 10) * 100));
  const questionTitle = item.question?.trim() || `Question ${item.questionIndex + 1}`;

  return (
    <Collapsible className="overflow-hidden rounded-2xl border bg-card shadow-sm transition-shadow hover:shadow-md">
      <CollapsibleTrigger className="group flex w-full items-start justify-between gap-6 p-5 text-left">
        <div className="flex min-w-0 flex-1 flex-col gap-2">
          <div className="flex items-center gap-2">
            <span
              className={cn(
                "inline-flex items-center rounded-full px-2 py-0.5 text-[10px] font-semibold uppercase tracking-[0.18em] ring-1 ring-inset",
                tone.badge,
              )}
            >
              Question {item.questionIndex + 1}
            </span>
            <span className={cn("text-xs font-medium", tone.text)}>{tone.label}</span>
          </div>
          <span className="font-display text-lg font-semibold leading-snug tracking-tight text-foreground">
            {questionTitle}
          </span>
        </div>
        <div className="flex shrink-0 items-center gap-3">
          <div className="flex flex-col items-end">
            <span className={cn("font-display text-3xl font-semibold leading-none tabular-nums", tone.text)}>
              {item.rating}
              <span className="ml-0.5 text-base font-normal text-muted-foreground">/10</span>
            </span>
            <div className="mt-1.5 h-1 w-20 overflow-hidden rounded-full bg-muted">
              <div
                className={cn("h-full rounded-full transition-all", tone.bar)}
                style={{ width: `${ratingPercent}%` }}
              />
            </div>
          </div>
          <ChevronDown className="h-4 w-4 text-muted-foreground transition-transform duration-200 group-data-[state=open]:rotate-180" />
        </div>
      </CollapsibleTrigger>

      <CollapsibleContent className="overflow-hidden data-[state=closed]:animate-accordion-up data-[state=open]:animate-accordion-down">
        <div className="grid gap-4 border-t bg-secondary/30 p-5 text-sm md:grid-cols-2">
          <div className="md:col-span-2">
            <DeliveryStrip
              fillerCount={item.fillerCount}
              longPauseCount={item.longPauseCount}
              wordsPerMinute={item.wordsPerMinute}
            />
          </div>
          <Section
            label="Your answer"
            icon={<User className="h-3.5 w-3.5" />}
            body={item.userAnswer}
            emptyHint="No answer was recorded for this question."
            tone="default"
          />
          <Section
            label="Reference answer"
            icon={<Sparkles className="h-3.5 w-3.5" />}
            body={item.correctAnswer}
            emptyHint="Reference answer unavailable."
            tone="reference"
          />
          <div className="md:col-span-2">
            <Section
              label="AI feedback"
              icon={<MessageSquare className="h-3.5 w-3.5" />}
              body={item.feedback}
              emptyHint="No feedback was generated."
              tone="feedback"
            />
          </div>
        </div>
      </CollapsibleContent>
    </Collapsible>
  );
}

interface DeliveryStripProps {
  fillerCount: number;
  longPauseCount: number;
  wordsPerMinute: number;
}

function DeliveryStrip({ fillerCount, longPauseCount, wordsPerMinute }: DeliveryStripProps): ReactElement {
  return (
    <div className="flex flex-wrap items-center gap-2">
      <span className="text-[10px] font-semibold uppercase tracking-[0.18em] text-muted-foreground">
        Delivery
      </span>
      <Chip
        icon={<Wind className="h-3.5 w-3.5" />}
        label="Filler words"
        value={String(fillerCount)}
        tone={fillerCount === 0 ? "good" : fillerCount <= 3 ? "neutral" : "warn"}
      />
      <Chip
        icon={<PauseCircle className="h-3.5 w-3.5" />}
        label="Long pauses"
        value={String(longPauseCount)}
        tone={longPauseCount === 0 ? "good" : longPauseCount <= 2 ? "neutral" : "warn"}
      />
      <Chip
        icon={<Gauge className="h-3.5 w-3.5" />}
        label="Words / min"
        value={wordsPerMinute > 0 ? String(wordsPerMinute) : "—"}
        tone="neutral"
      />
    </div>
  );
}

interface ChipProps {
  icon: ReactElement;
  label: string;
  value: string;
  tone: "good" | "neutral" | "warn";
}

function Chip({ icon, label, value, tone }: ChipProps): ReactElement {
  const toneClass = cn(
    "inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium ring-1 ring-inset",
    tone === "good" &&
      "bg-emerald-50 text-emerald-700 ring-emerald-200 dark:bg-emerald-500/10 dark:text-emerald-300 dark:ring-emerald-500/30",
    tone === "neutral" &&
      "bg-secondary text-foreground ring-border",
    tone === "warn" &&
      "bg-amber-50 text-amber-800 ring-amber-200 dark:bg-amber-500/10 dark:text-amber-300 dark:ring-amber-500/30",
  );
  return (
    <span className={toneClass}>
      {icon}
      <span className="text-muted-foreground">{label}</span>
      <span className="tabular-nums font-semibold">{value}</span>
    </span>
  );
}

interface SectionProps {
  label: string;
  icon: ReactElement;
  body: string | undefined;
  emptyHint: string;
  tone: "default" | "reference" | "feedback";
}

function Section({ label, icon, body, emptyHint, tone }: SectionProps): ReactElement {
  const text = body?.trim() ?? "";
  const isEmpty = text.length === 0;

  const containerClass = cn(
    "flex h-full flex-col gap-2 rounded-xl border p-4",
    tone === "reference" && "border-emerald-200/60 bg-emerald-50/40 dark:border-emerald-500/20 dark:bg-emerald-500/5",
    tone === "feedback" && "border-primary/15 bg-primary/5",
    tone === "default" && "border-border/60 bg-background",
  );

  const labelClass = cn(
    "inline-flex items-center gap-1.5 text-[10px] font-semibold uppercase tracking-[0.18em]",
    tone === "reference" && "text-emerald-700 dark:text-emerald-300",
    tone === "feedback" && "text-primary",
    tone === "default" && "text-muted-foreground",
  );

  return (
    <div className={containerClass}>
      <p className={labelClass}>
        {icon}
        {label}
      </p>
      {isEmpty ? (
        <p className="text-sm italic text-muted-foreground">{emptyHint}</p>
      ) : (
        <p className="whitespace-pre-wrap leading-relaxed text-foreground">{text}</p>
      )}
    </div>
  );
}

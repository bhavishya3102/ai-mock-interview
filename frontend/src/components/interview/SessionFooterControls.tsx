import type { ReactElement } from "react";
import { ArrowLeft, ArrowRight, Flag } from "lucide-react";
import { Button } from "@/components/ui/button";

interface SessionFooterControlsProps {
  questionCount: number;
  activeIndex: number;
  onPrev: () => void;
  onNext: () => void;
  onEnd: () => void;
}

export function SessionFooterControls({
  questionCount,
  activeIndex,
  onPrev,
  onNext,
  onEnd,
}: SessionFooterControlsProps): ReactElement {
  const isFirst = activeIndex === 0;
  const isLast = activeIndex === questionCount - 1;

  return (
    <div className="flex items-center gap-2 rounded-2xl border bg-card p-3 shadow-sm">
      <Button type="button" variant="ghost" onClick={onPrev} disabled={isFirst} className="shrink-0">
        <ArrowLeft className="h-4 w-4" /> Previous
      </Button>
      <p className="min-w-0 flex-1 text-center text-xs text-muted-foreground">
        Question {activeIndex + 1} of {questionCount}
      </p>
      <div className="shrink-0">
        {isLast ? (
          <Button type="button" onClick={onEnd} variant="accent">
            <Flag className="h-4 w-4" /> End interview
          </Button>
        ) : (
          <Button type="button" onClick={onNext}>
            Next <ArrowRight className="h-4 w-4" />
          </Button>
        )}
      </div>
    </div>
  );
}

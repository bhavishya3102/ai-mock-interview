import type { ReactElement } from "react";
import { Link } from "react-router-dom";
import { Button } from "@/components/ui/button";

interface EmptyFeedbackProps {
  mockId: string;
}

export function EmptyFeedback({ mockId }: EmptyFeedbackProps): ReactElement {
  return (
    <div className="rounded-2xl border border-dashed border-border/80 bg-card/50 p-12 text-center">
      <p className="font-display text-2xl font-semibold tracking-tight">No answers yet</p>
      <p className="mt-2 text-sm text-muted-foreground">
        Record at least one answer to see ratings and coaching here.
      </p>
      <Button asChild className="mt-6">
        <Link to={`/interview/${mockId}/start`}>Start the session</Link>
      </Button>
    </div>
  );
}

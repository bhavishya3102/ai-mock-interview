import type { ReactElement } from "react";
import { Link } from "react-router-dom";
import { ArrowRight, MessageSquare } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { formatRelative, pluralize } from "@/lib/format";
import type { InterviewSummary } from "@/types/api";

interface InterviewCardProps {
  interview: InterviewSummary;
}

export function InterviewCard({ interview }: InterviewCardProps): ReactElement {
  return (
    <Card className="flex h-full flex-col">
      <CardHeader>
        <CardTitle className="line-clamp-2">{interview.jobPosition}</CardTitle>
        <p className="text-xs uppercase tracking-[0.18em] text-muted-foreground">
          {pluralize(interview.yearsExperience, "year")} of experience · {formatRelative(interview.createdAt)}
        </p>
      </CardHeader>
      <CardContent className="flex flex-1 flex-col justify-between gap-6">
        <p className="line-clamp-3 text-sm text-muted-foreground">{interview.jobDescription}</p>
        <div className="flex items-center gap-2">
          <Button asChild variant="outline" size="sm" className="flex-1">
            <Link to={`/interview/${interview.mockId}/feedback`}>
              <MessageSquare className="h-4 w-4" /> Feedback / AI Career Coach
            </Link>
          </Button>
          <Button asChild size="sm" className="flex-1">
            <Link to={`/interview/${interview.mockId}`}>
              Start <ArrowRight className="h-4 w-4" />
            </Link>
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}

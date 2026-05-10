import type { ReactElement } from "react";
import { Link, useParams } from "react-router-dom";
import { ArrowLeft, ArrowRight, Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { PageContainer } from "@/components/layout/PageContainer";
import { WebcamPanel } from "@/components/interview/WebcamPanel";
import { useInterview } from "@/hooks/useInterview";
import { pluralize } from "@/lib/format";

export default function PreInterview(): ReactElement {
  const { mockId } = useParams<{ mockId: string }>();
  const { data, isLoading, isError, error } = useInterview(mockId);

  if (!mockId) {
    return (
      <PageContainer>
        <p>Missing interview id.</p>
      </PageContainer>
    );
  }

  return (
    <PageContainer>
      <Button asChild variant="ghost" size="sm" className="mb-6">
        <Link to="/dashboard">
          <ArrowLeft className="h-4 w-4" /> Back to dashboard
        </Link>
      </Button>

      {isLoading ? (
        <div className="flex items-center gap-2 rounded-2xl border border-dashed bg-card/40 p-12 text-sm text-muted-foreground">
          <Loader2 className="h-4 w-4 animate-spin" /> Loading interview
        </div>
      ) : isError || !data ? (
        <div className="rounded-2xl border border-destructive/40 bg-destructive/5 p-12 text-sm text-destructive">
          {error?.message || "Could not load interview."}
        </div>
      ) : (
        <div className="grid gap-8 lg:grid-cols-[3fr_2fr]">
          <Card>
            <CardHeader>
              <p className="text-xs font-semibold uppercase tracking-[0.2em] text-muted-foreground">
                Mock interview
              </p>
              <CardTitle className="text-3xl">{data.jobPosition}</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4 text-sm">
              <Detail label="Years of experience" value={pluralize(data.yearsExperience, "year")} />
              <Detail label="Job description" value={data.jobDescription} multiline />
              <Detail label="Questions" value={`${data.questions.length} prepared`} />
            </CardContent>
          </Card>

          <div className="flex flex-col gap-4">
            <WebcamPanel />
            <div className="rounded-2xl border bg-card p-5 shadow-sm">
              <p className="text-xs font-semibold uppercase tracking-[0.2em] text-muted-foreground">
                Before you start
              </p>
              <ul className="mt-3 list-disc space-y-1.5 pl-5 text-sm text-muted-foreground">
                <li>Find a quiet spot and allow the microphone when the browser asks.</li>
                <li>
                  For voice answers, use <strong className="font-medium text-foreground">Chrome or Edge</strong>{" "}
                  on desktop for the most reliable transcription.
                </li>
                <li>Use the speaker icon on each question to hear it read aloud.</li>
                <li>Press Stop when you finish speaking — your text saves right after.</li>
              </ul>
            </div>
            <Button asChild size="lg" className="self-end">
              <Link to={`/interview/${mockId}/start`}>
                Start interview <ArrowRight className="h-4 w-4" />
              </Link>
            </Button>
          </div>
        </div>
      )}
    </PageContainer>
  );
}

interface DetailProps {
  label: string;
  value: string;
  multiline?: boolean;
}

function Detail({ label, value, multiline = false }: DetailProps): ReactElement {
  return (
    <div>
      <p className="text-xs font-semibold uppercase tracking-[0.18em] text-muted-foreground">{label}</p>
      <p className={multiline ? "mt-1 whitespace-pre-line leading-relaxed" : "mt-1 leading-relaxed"}>
        {value}
      </p>
    </div>
  );
}

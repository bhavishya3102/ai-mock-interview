import { useMemo, type ReactElement } from "react";
import { AlertCircle, Loader2, Sparkles, StopCircle } from "lucide-react";
import ReactMarkdown from "react-markdown";
import { Button } from "@/components/ui/button";
import { useCoachReport, useStreamingCoachReport } from "@/hooks/useCoachReport";
import { cn } from "@/lib/cn";

interface CoachReportCardProps {
  mockId: string;
}

// CoachReportCard renders the post-interview coaching report below the
// per-question feedback. Behaviour:
//   - On mount it queries the cached report (404 = "not generated yet").
//   - If cached, the markdown is rendered immediately.
//   - Otherwise a Generate button opens the SSE stream; chunks accumulate
//     into the rendered markdown (typewriter effect for free).
//   - Stream can be cancelled mid-flight; errors are surfaced with retry.
export function CoachReportCard({ mockId }: CoachReportCardProps): ReactElement {
  const cached = useCoachReport(mockId);
  const stream = useStreamingCoachReport(mockId);

  // While the stream is active (or has just finished) prefer its buffer over
  // the React-Query cached row — the cached row is the prior render's view
  // of the same data and gets refreshed by stream.onDone's invalidation.
  const isLive = stream.state !== "idle";
  const displayText = isLive ? stream.text : cached.data?.content ?? "";
  const hasContent = displayText.length > 0;
  const isStreaming = stream.state === "streaming";

  // Footer line: "Powered by gemini-2.5-flash • 1,842 tokens"
  const footer = useMemo(() => {
    const meta = stream.metadata
      ? { model: stream.metadata.model, tokens: stream.metadata.tokensUsed }
      : cached.data
        ? { model: cached.data.model, tokens: cached.data.tokensUsed }
        : null;
    if (!meta) return null;
    return `Powered by ${meta.model} • ${meta.tokens.toLocaleString()} tokens`;
  }, [stream.metadata, cached.data]);

  return (
    <section className="mt-10 overflow-hidden rounded-2xl border bg-card shadow-sm">
      <header className="flex items-start justify-between gap-4 border-b bg-muted/30 p-5">
        <div className="flex items-start gap-3">
          <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-primary/10 text-primary">
            <Sparkles className="h-4 w-4" />
          </span>
          <div>
            <p className="text-xs font-semibold uppercase tracking-[0.2em] text-muted-foreground">
              AI Coach
            </p>
            <h2 className="mt-1 font-display text-lg font-semibold leading-tight">
              Your coaching report
            </h2>
            <p className="mt-1 text-sm text-muted-foreground">
              A narrative summary across all your answers — strengths, gaps, and what to study next.
            </p>
          </div>
        </div>

        {/* Action button(s) live in the header so the body stays focused on content. */}
        <CoachReportActions
          mockId={mockId}
          cachedLoading={cached.isLoading}
          cachedHasData={Boolean(cached.data)}
          state={stream.state}
          onStart={stream.start}
          onCancel={stream.cancel}
        />
      </header>

      <div className="p-5">
        {cached.isLoading && !isLive ? (
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <Loader2 className="h-4 w-4 animate-spin" />
            Checking for an existing report…
          </div>
        ) : stream.state === "error" ? (
          <ErrorState
            code={stream.error?.code ?? "UNKNOWN"}
            message={stream.error?.message ?? "Could not generate the report."}
            onRetry={stream.start}
          />
        ) : hasContent ? (
          <article
            className={cn(
              "prose prose-sm max-w-none dark:prose-invert",
              "prose-headings:font-display prose-headings:tracking-tight",
              "prose-h2:mt-0 prose-h2:text-xl",
              "prose-h3:text-base prose-h3:mb-2",
              "prose-li:my-0",
            )}
            aria-live={isStreaming ? "polite" : "off"}
            aria-busy={isStreaming}
          >
            <ReactMarkdown>{displayText}</ReactMarkdown>
            {isStreaming ? <StreamingCaret /> : null}
          </article>
        ) : (
          <EmptyState />
        )}

        {footer && !isStreaming ? (
          <p className="mt-6 text-[11px] uppercase tracking-[0.16em] text-muted-foreground">
            {footer}
          </p>
        ) : null}
      </div>
    </section>
  );
}

interface CoachReportActionsProps {
  mockId: string;
  cachedLoading: boolean;
  cachedHasData: boolean;
  state: ReturnType<typeof useStreamingCoachReport>["state"];
  onStart: () => void;
  onCancel: () => void;
}

function CoachReportActions({
  mockId,
  cachedLoading,
  cachedHasData,
  state,
  onStart,
  onCancel,
}: CoachReportActionsProps): ReactElement | null {
  // Don't show any action while we're still checking the cache; otherwise
  // the Generate button can flash and look like the user missed the cached
  // copy that was about to load.
  if (cachedLoading && state === "idle") return null;
  // mockId is destructured so the disabled state can short-circuit when
  // the parent forgot to provide it.
  const disabled = !mockId;

  if (state === "streaming") {
    return (
      <Button variant="outline" size="sm" onClick={onCancel} disabled={disabled} data-testid="coach-report-cancel">
        <StopCircle className="h-4 w-4" />
        Stop
      </Button>
    );
  }
  // For both "no cached data and idle" and "error" we offer the same
  // primary action — the error UI in the body provides its own retry too.
  if (!cachedHasData && (state === "idle" || state === "error")) {
    return (
      <Button onClick={onStart} disabled={disabled} size="sm" data-testid="coach-report-generate">
        <Sparkles className="h-4 w-4" />
        Generate report
      </Button>
    );
  }
  return null;
}

function EmptyState(): ReactElement {
  return (
    <div className="rounded-xl border border-dashed bg-muted/20 p-6 text-sm text-muted-foreground">
      No report yet. Click <span className="font-medium text-foreground">Generate report</span> to
      have the coach review your answers and write a personalised summary. This usually takes
      10–20 seconds and the report streams in as it&apos;s written.
    </div>
  );
}

interface ErrorStateProps {
  code: string;
  message: string;
  onRetry: () => void;
}

function ErrorState({ code, message, onRetry }: ErrorStateProps): ReactElement {
  return (
    <div className="rounded-xl border border-destructive/40 bg-destructive/5 p-5">
      <div className="flex items-start gap-3 text-sm text-destructive">
        <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" />
        <div className="flex-1">
          <p className="font-semibold">Couldn&apos;t generate the report.</p>
          <p className="mt-1 text-destructive/80">{message}</p>
          <p className="mt-2 text-[11px] uppercase tracking-[0.16em] text-destructive/60">
            code: {code}
          </p>
        </div>
      </div>
      <Button variant="outline" size="sm" className="mt-4" onClick={onRetry}>
        Try again
      </Button>
    </div>
  );
}

// StreamingCaret is the small pulsing block that appears at the end of
// streamed text so the user has a visible "still writing" signal.
function StreamingCaret(): ReactElement {
  return (
    <span
      className="ml-0.5 inline-block h-4 w-1.5 translate-y-0.5 animate-pulse rounded-sm bg-primary align-middle"
      aria-hidden="true"
    />
  );
}

import { useEffect, useRef, useState, type ReactElement } from "react";
import { CheckCircle2, ChevronRight, Loader2, Mic, Square, RotateCcw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useAudioRecorder } from "@/hooks/useAudioRecorder";
import {
  useJudgeFollowUp,
  useSubmitAnswer,
  useTranscribeAudio,
} from "@/hooks/useInterviewMutations";
import { useToast } from "@/hooks/use-toast";
import { useSessionStore } from "@/store/sessionStore";
import { ApiError } from "@/api/client";
import { cn } from "@/lib/cn";
import type { FollowUpTurn, SpeechAnalysis } from "@/api/interviews";
import { SpeechAnalysisCard } from "@/components/interview/SpeechAnalysisCard";

// Mirrors the backend's MaxFollowUpsPerQuestion. Defensive client-side cap —
// the backend already enforces it, but we use this to disable the Record
// button once the cap is hit so the UI doesn't promise another turn that
// the server will refuse.
const MAX_FOLLOW_UPS = 2;

interface RecordAnswerControlProps {
  mockId: string;
  questionIndex: number;
}

function formatElapsed(totalSeconds: number): string {
  const m = Math.floor(totalSeconds / 60)
    .toString()
    .padStart(2, "0");
  const s = Math.floor(totalSeconds % 60)
    .toString()
    .padStart(2, "0");
  return `${m}:${s}`;
}

function composeUserAnswer(main: string, turns: FollowUpTurn[]): string {
  let s = main.trim();
  for (const t of turns) {
    s += `\n\n[Follow-up: ${t.question.trim()}]\n${t.answer.trim()}`;
  }
  return s;
}

export function RecordAnswerControl({
  mockId,
  questionIndex,
}: RecordAnswerControlProps): ReactElement {
  const { isRecording, elapsedSeconds, isSupported, error: recorderError, start, stop } =
    useAudioRecorder();

  const transcribe = useTranscribeAudio();
  const judge = useJudgeFollowUp();
  const submit = useSubmitAnswer();
  const { toast } = useToast();

  // The audio-recorder pipeline writes the latest transcribed snippet here.
  // We read it to display interim state, but we never auto-submit from it —
  // a separate state machine below owns the conversation.
  const setTranscript = useSessionStore((s) => s.setTranscript);

  // Conversation state for the current main question.
  const [mainAnswer, setMainAnswer] = useState<string>("");
  const [turns, setTurns] = useState<FollowUpTurn[]>([]);
  const [currentFollowUp, setCurrentFollowUp] = useState<string | null>(null);
  const [showSavedHint, setShowSavedHint] = useState(false);
  // Delivery analysis of the most recent recording. Cleared per-question
  // because metrics for question N don't apply to question N+1.
  const [latestAnalysis, setLatestAnalysis] = useState<SpeechAnalysis | null>(null);

  // Aggregated metrics across all recordings (main + follow-ups) for the
  // current question — what we ultimately send with SubmitAnswer so the
  // feedback page can show per-question delivery stats. A ref (not state)
  // so finalize() reads the latest value synchronously from async callbacks.
  // wpm is averaged over clips with wpm>0 (zero means "too short to estimate").
  const metricsRef = useRef<{
    fillerCount: number;
    longPauseCount: number;
    wpmSum: number;
    wpmSamples: number;
  }>({ fillerCount: 0, longPauseCount: 0, wpmSum: 0, wpmSamples: 0 });

  function resetMetrics(): void {
    metricsRef.current = { fillerCount: 0, longPauseCount: 0, wpmSum: 0, wpmSamples: 0 };
  }

  function addClipMetrics(a: SpeechAnalysis): void {
    metricsRef.current.fillerCount += a.fillerCount;
    metricsRef.current.longPauseCount += a.longPauseCount;
    if (a.wordsPerMinute > 0) {
      metricsRef.current.wpmSum += a.wordsPerMinute;
      metricsRef.current.wpmSamples += 1;
    }
  }

  function replaceWithClipMetrics(a: SpeechAnalysis): void {
    metricsRef.current = {
      fillerCount: a.fillerCount,
      longPauseCount: a.longPauseCount,
      wpmSum: a.wordsPerMinute > 0 ? a.wordsPerMinute : 0,
      wpmSamples: a.wordsPerMinute > 0 ? 1 : 0,
    };
  }

  // Reset every transient piece of state when the active question changes.
  // Without this, partway-completed follow-ups from question N leak into N+1.
  useEffect(() => {
    setMainAnswer("");
    setTurns([]);
    setCurrentFollowUp(null);
    setShowSavedHint(false);
    setLatestAnalysis(null);
    resetMetrics();
    setTranscript("");
    judge.reset();
    submit.reset();
    transcribe.reset();
    // We intentionally exclude the mutation refs — they're stable per render
    // and including them would re-run reset on every keystroke.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [mockId, questionIndex]);

  useEffect(() => {
    if (!showSavedHint) return;
    const t = window.setTimeout(() => setShowSavedHint(false), 4500);
    return () => window.clearTimeout(t);
  }, [showSavedHint]);

  const isBusy =
    isRecording || transcribe.isPending || judge.isPending || submit.isPending;
  const capReached = turns.length >= MAX_FOLLOW_UPS;
  const phase: "idle" | "main" | "follow-up" | "finalized" = showSavedHint
    ? "finalized"
    : !mainAnswer
      ? "idle"
      : currentFollowUp
        ? "follow-up"
        : "main";

  // submitRef lets the async callbacks read the latest mutation handle without
  // re-creating closures every render.
  const submitRef = useRef(submit);
  submitRef.current = submit;

  async function finalize(main: string, turnsList: FollowUpTurn[]): Promise<void> {
    const m = metricsRef.current;
    const wpm = m.wpmSamples > 0 ? Math.round(m.wpmSum / m.wpmSamples) : 0;
    try {
      await submitRef.current.mutateAsync({
        mockId,
        payload: {
          questionIndex,
          userAnswer: composeUserAnswer(main, turnsList),
          fillerCount: m.fillerCount,
          wordsPerMinute: wpm,
          longPauseCount: m.longPauseCount,
        },
      });
      setShowSavedHint(true);
      toast({
        title: "Answer saved",
        description: "Open Feedback after the interview to see your rating.",
      });
    } catch (err) {
      const message = err instanceof ApiError ? err.message : "Could not save answer.";
      toast({ title: "Save failed", description: message, variant: "destructive" });
    }
  }

  async function askForNextFollowUp(main: string, turnsList: FollowUpTurn[]): Promise<void> {
    if (turnsList.length >= MAX_FOLLOW_UPS) {
      // Defensive — backend would return "" anyway, but skip the round trip.
      await finalize(main, turnsList);
      return;
    }
    try {
      const result = await judge.mutateAsync({
        mockId,
        payload: {
          questionIndex,
          mainAnswer: main,
          priorTurns: turnsList,
        },
      });
      const next = result.followUp.trim();
      if (!next) {
        await finalize(main, turnsList);
        return;
      }
      setCurrentFollowUp(next);
      setTranscript("");
    } catch (err) {
      // Don't block the user on a judge failure — finalize what we have.
      const message =
        err instanceof ApiError ? err.message : "Could not judge follow-up.";
      toast({
        title: "Skipping follow-up",
        description: `${message} — saving your current answer instead.`,
        variant: "destructive",
      });
      await finalize(main, turnsList);
    }
  }

  async function handleStartStop(): Promise<void> {
    if (isRecording) {
      const recording = await stop();
      if (!recording) {
        toast({
          title: "Nothing was captured",
          description: "We didn't pick up any audio. Try recording again.",
          variant: "destructive",
        });
        return;
      }
      let text = "";
      let analysis: SpeechAnalysis | null = null;
      try {
        const result = await transcribe.mutateAsync({
          mockId,
          audio: recording.blob,
          longPauseCount: recording.longPauseCount,
        });
        text = result.transcript.trim();
        analysis = result.analysis;
        setLatestAnalysis(result.analysis);
      } catch (err) {
        const message =
          err instanceof ApiError ? err.message : "Could not transcribe audio.";
        toast({ title: "Transcription failed", description: message, variant: "destructive" });
        return;
      }
      if (!text || !analysis) {
        toast({
          title: "No speech detected",
          description: "We couldn't hear anything intelligible. Speak closer to the mic and try again.",
          variant: "destructive",
        });
        return;
      }

      if (!mainAnswer) {
        // First recording for this question — this is the main answer.
        replaceWithClipMetrics(analysis);
        setMainAnswer(text);
        setTranscript(text);
        await askForNextFollowUp(text, []);
      } else if (currentFollowUp) {
        // Recording is the candidate's reply to the current follow-up.
        addClipMetrics(analysis);
        const nextTurns: FollowUpTurn[] = [
          ...turns,
          { question: currentFollowUp, answer: text },
        ];
        setTurns(nextTurns);
        setCurrentFollowUp(null);
        setTranscript(text);
        await askForNextFollowUp(mainAnswer, nextTurns);
      } else {
        // No active follow-up but mainAnswer exists — treat as a re-record of
        // the main answer (user used the Clear button, or hit Record again).
        replaceWithClipMetrics(analysis);
        setMainAnswer(text);
        setTurns([]);
        setTranscript(text);
        await askForNextFollowUp(text, []);
      }
      return;
    }

    judge.reset();
    submit.reset();
    transcribe.reset();
    await start();
  }

  async function handleMoveOn(): Promise<void> {
    if (!mainAnswer || isBusy) return;
    // Drop any pending follow-up question — user opted out of answering it.
    setCurrentFollowUp(null);
    await finalize(mainAnswer, turns);
  }

  function handleClear(): void {
    setMainAnswer("");
    setTurns([]);
    setCurrentFollowUp(null);
    setShowSavedHint(false);
    setLatestAnalysis(null);
    resetMetrics();
    setTranscript("");
    judge.reset();
    submit.reset();
    transcribe.reset();
  }

  const supportMessage =
    !isSupported && !recorderError
      ? "Audio recording isn't available in this browser. Try Chrome, Firefox, or Edge."
      : null;

  const recordDisabled =
    !isSupported || transcribe.isPending || judge.isPending || submit.isPending ||
    (phase === "main" && capReached);

  const recordButtonLabel = !mainAnswer
    ? "Record answer"
    : currentFollowUp
      ? "Record follow-up answer"
      : "Re-record main answer";

  return (
    <div className="rounded-2xl border bg-card p-5 shadow-sm">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div className="min-w-0 flex-1">
          <p className="text-xs font-semibold uppercase tracking-[0.2em] text-muted-foreground">
            {phase === "follow-up" ? "Follow-up" : "Your answer"}
          </p>
          <p className="mt-1 text-sm text-muted-foreground">
            {isRecording
              ? "Recording — speak clearly. Press Stop when you're finished."
              : transcribe.isPending
                ? "Transcribing your answer…"
                : judge.isPending
                  ? "Interviewer is thinking…"
                  : submit.isPending
                    ? "Saving your answer…"
                    : phase === "follow-up"
                      ? "The interviewer asked a follow-up. Record your reply, or click Move on to skip."
                      : mainAnswer
                        ? "Saved your main answer. Awaiting next step."
                        : "Press Record answer, speak, then Stop. Your transcript appears once we process the audio."}
          </p>
          {isRecording ? (
            <div
              className="mt-2 flex items-center gap-2 text-sm font-medium text-destructive"
              aria-live="polite"
            >
              <span className="relative flex h-2.5 w-2.5">
                <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-destructive opacity-75" />
                <span className="relative inline-flex h-2.5 w-2.5 rounded-full bg-destructive" />
              </span>
              Recording… {formatElapsed(elapsedSeconds)}
            </div>
          ) : null}
        </div>
        <div className="flex shrink-0 items-center gap-2">
          {mainAnswer && !isBusy ? (
            <Button
              type="button"
              variant="ghost"
              size="icon"
              onClick={handleClear}
              aria-label="Clear and start over"
            >
              <RotateCcw className="h-4 w-4" />
            </Button>
          ) : null}
          <Button
            type="button"
            onClick={() => {
              void handleStartStop();
            }}
            variant={isRecording ? "destructive" : "default"}
            disabled={recordDisabled}
            aria-pressed={isRecording}
            className={cn(
              isRecording &&
                "animate-pulse shadow-md ring-2 ring-destructive ring-offset-2 ring-offset-background",
            )}
          >
            {transcribe.isPending ? (
              <>
                <Loader2 className="h-4 w-4 animate-spin" /> Transcribing
              </>
            ) : judge.isPending ? (
              <>
                <Loader2 className="h-4 w-4 animate-spin" /> Thinking
              </>
            ) : submit.isPending ? (
              <>
                <Loader2 className="h-4 w-4 animate-spin" /> Saving
              </>
            ) : isRecording ? (
              <>
                <Square className="h-4 w-4 fill-current" /> Stop recording
              </>
            ) : (
              <>
                <Mic className="h-4 w-4" /> {recordButtonLabel}
              </>
            )}
          </Button>
        </div>
      </div>

      {mainAnswer ? (
        <div className="mt-5 rounded-lg border bg-secondary/30 p-4 text-sm">
          <p className="text-xs font-semibold uppercase tracking-[0.2em] text-muted-foreground">
            Main answer
          </p>
          <p className="mt-2 whitespace-pre-wrap leading-relaxed">{mainAnswer}</p>
        </div>
      ) : null}

      {latestAnalysis ? <SpeechAnalysisCard analysis={latestAnalysis} /> : null}

      {turns.map((t, i) => (
        <div
          key={`turn-${i}`}
          className="mt-3 rounded-lg border bg-secondary/30 p-4 text-sm"
        >
          <p className="text-xs font-semibold uppercase tracking-[0.2em] text-muted-foreground">
            Follow-up {i + 1}
          </p>
          <p className="mt-2 font-medium">{t.question}</p>
          <p className="mt-1 whitespace-pre-wrap leading-relaxed text-muted-foreground">
            {t.answer}
          </p>
        </div>
      ))}

      {currentFollowUp ? (
        <div className="mt-3 rounded-lg border border-amber-300/60 bg-amber-50/70 p-4 text-sm dark:border-amber-400/30 dark:bg-amber-950/40">
          <p className="text-xs font-semibold uppercase tracking-[0.2em] text-amber-900 dark:text-amber-200">
            Interviewer follow-up
          </p>
          <p className="mt-2 font-medium text-amber-950 dark:text-amber-50">
            {currentFollowUp}
          </p>
        </div>
      ) : null}

      {!mainAnswer ? (
        <div
          className={cn(
            "mt-5 min-h-[120px] rounded-lg border border-dashed bg-secondary/40 p-4 text-sm leading-relaxed transition-colors",
            isRecording && "border-destructive/40 bg-destructive/[0.04] dark:bg-destructive/10",
          )}
        >
          {transcribe.isPending ? (
            <p className="flex items-center gap-2 italic text-muted-foreground">
              <Loader2 className="h-4 w-4 animate-spin" />
              Transcribing your answer — this usually takes a few seconds.
            </p>
          ) : isRecording ? (
            <p className="italic text-muted-foreground">
              Listening… your transcript will appear after you press Stop.
            </p>
          ) : (
            <p className="italic text-muted-foreground">
              Your transcript will show up here after you record an answer.
            </p>
          )}
        </div>
      ) : null}

      {mainAnswer && !showSavedHint ? (
        <div className="mt-4 flex flex-wrap items-center justify-between gap-3 rounded-md border bg-card p-3">
          <p className="text-xs text-muted-foreground">
            {capReached
              ? `Cap reached (${MAX_FOLLOW_UPS} follow-ups). Click Move on to save.`
              : `${turns.length} of ${MAX_FOLLOW_UPS} possible follow-ups used.`}
          </p>
          <Button
            type="button"
            variant="secondary"
            size="sm"
            onClick={() => {
              void handleMoveOn();
            }}
            disabled={isBusy}
          >
            Move on <ChevronRight className="h-4 w-4" />
          </Button>
        </div>
      ) : null}

      {showSavedHint ? (
        <p className="mt-3 flex items-center gap-2 text-xs font-medium text-emerald-700 dark:text-emerald-400">
          <CheckCircle2 className="h-3.5 w-3.5 shrink-0" />
          Saved — move to the next question or keep practicing here.
        </p>
      ) : null}

      {recorderError ? (
        <p className="mt-3 rounded-md border border-destructive/40 bg-destructive/10 p-3 text-sm text-destructive">
          {recorderError}
        </p>
      ) : null}

      {supportMessage ? (
        <p className="mt-3 rounded-md border border-amber-300/50 bg-amber-50/80 p-3 text-sm text-amber-950 dark:border-amber-400/30 dark:bg-amber-950/40 dark:text-amber-100">
          {supportMessage}
        </p>
      ) : null}
    </div>
  );
}

import { useEffect, useRef, useState, type ReactElement } from "react";
import { CheckCircle2, Loader2, Mic, Square, RotateCcw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useAudioRecorder } from "@/hooks/useAudioRecorder";
import { useSubmitAnswer, useTranscribeAudio } from "@/hooks/useInterviewMutations";
import { useToast } from "@/hooks/use-toast";
import { useSessionStore } from "@/store/sessionStore";
import { ApiError } from "@/api/client";
import { cn } from "@/lib/cn";

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

export function RecordAnswerControl({
  mockId,
  questionIndex,
}: RecordAnswerControlProps): ReactElement {
  const { isRecording, elapsedSeconds, isSupported, error: recorderError, start, stop } =
    useAudioRecorder();

  const transcribe = useTranscribeAudio();
  const submit = useSubmitAnswer();
  const { toast } = useToast();

  const transcript = useSessionStore((s) => s.transcript);
  const setTranscript = useSessionStore((s) => s.setTranscript);

  const [showSavedHint, setShowSavedHint] = useState(false);
  const [saveRetryKey, setSaveRetryKey] = useState(0);
  const triggeredRef = useRef(false);

  const trimmed = transcript.trim();
  const wordCount = trimmed ? trimmed.split(/\s+/).filter(Boolean).length : 0;

  const submitResetRef = useRef(submit.reset);
  submitResetRef.current = submit.reset;
  const transcribeResetRef = useRef(transcribe.reset);
  transcribeResetRef.current = transcribe.reset;

  // Reset all transient state when the active question changes — otherwise a
  // half-completed transcript from question 1 leaks into question 2.
  useEffect(() => {
    triggeredRef.current = false;
    submitResetRef.current();
    transcribeResetRef.current();
    setShowSavedHint(false);
  }, [mockId, questionIndex]);

  useEffect(() => {
    if (!showSavedHint) return;
    const t = window.setTimeout(() => setShowSavedHint(false), 4500);
    return () => window.clearTimeout(t);
  }, [showSavedHint]);

  // Auto-submit once we have a non-empty transcript and we're not still busy
  // capturing or transcribing audio. Keeps the existing "stop, save, move on"
  // flow intact — just sourced from MediaRecorder + Gemini instead of Web
  // Speech.
  useEffect(() => {
    if (isRecording || transcribe.isPending) {
      triggeredRef.current = false;
      return;
    }
    if (triggeredRef.current) return;
    if (trimmed.length < 1) return;

    triggeredRef.current = true;
    submit.mutate(
      { mockId, payload: { questionIndex, userAnswer: trimmed } },
      {
        onSuccess: () => {
          setShowSavedHint(true);
          toast({
            title: "Answer saved",
            description: "Open Feedback after the interview to see your rating.",
          });
          setTranscript("");
        },
        onError: (err) => {
          const message = err instanceof ApiError ? err.message : "Could not save answer.";
          toast({ title: "Save failed", description: message, variant: "destructive" });
        },
      },
    );
  }, [
    isRecording,
    transcribe.isPending,
    trimmed,
    mockId,
    questionIndex,
    saveRetryKey,
    submit,
    toast,
    setTranscript,
  ]);

  const handleStartStop = async () => {
    if (isRecording) {
      const blob = await stop();
      if (!blob) {
        toast({
          title: "Nothing was captured",
          description: "We didn't pick up any audio. Try recording again.",
          variant: "destructive",
        });
        return;
      }
      try {
        const result = await transcribe.mutateAsync({ mockId, audio: blob });
        const text = result.transcript.trim();
        if (!text) {
          toast({
            title: "No speech detected",
            description: "We couldn't hear anything intelligible. Speak closer to the mic and try again.",
            variant: "destructive",
          });
          return;
        }
        setTranscript(text);
      } catch (err) {
        const message = err instanceof ApiError ? err.message : "Could not transcribe audio.";
        toast({ title: "Transcription failed", description: message, variant: "destructive" });
      }
      return;
    }

    triggeredRef.current = false;
    submit.reset();
    transcribe.reset();
    await start();
  };

  const supportMessage =
    !isSupported && !recorderError
      ? "Audio recording isn't available in this browser. Try Chrome, Firefox, or Edge."
      : null;

  const isBusy = isRecording || transcribe.isPending || submit.isPending;

  return (
    <div className="rounded-2xl border bg-card p-5 shadow-sm">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div className="min-w-0 flex-1">
          <p className="text-xs font-semibold uppercase tracking-[0.2em] text-muted-foreground">
            Your answer
          </p>
          <p className="mt-1 text-sm text-muted-foreground">
            {isRecording
              ? "Recording — speak clearly. Press Stop when you're finished."
              : transcribe.isPending
                ? "Transcribing your answer…"
                : transcript
                  ? "Re-record to replace this answer, or clear and try again. Saving runs automatically."
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
          {transcript && !isBusy ? (
            <Button
              type="button"
              variant="ghost"
              size="icon"
              onClick={() => {
                setTranscript("");
                submit.reset();
                transcribe.reset();
                triggeredRef.current = false;
              }}
              aria-label="Clear transcript"
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
            disabled={!isSupported || transcribe.isPending || submit.isPending}
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
                <Mic className="h-4 w-4" /> Record answer
              </>
            )}
          </Button>
        </div>
      </div>

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
        ) : transcript ? (
          <p className="whitespace-pre-wrap">{transcript}</p>
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

      {transcript && !isBusy ? (
        <p className="mt-2 text-xs text-muted-foreground">
          {wordCount} word{wordCount === 1 ? "" : "s"} · {trimmed.length} characters
        </p>
      ) : null}

      {submit.isPending ? (
        <p className="mt-2 flex items-center gap-2 text-xs font-medium text-muted-foreground">
          <Loader2 className="h-3.5 w-3.5 animate-spin" />
          Saving your answer…
        </p>
      ) : null}

      {showSavedHint ? (
        <p className="mt-2 flex items-center gap-2 text-xs font-medium text-emerald-700 dark:text-emerald-400">
          <CheckCircle2 className="h-3.5 w-3.5 shrink-0" />
          Saved — move to the next question or keep practicing here.
        </p>
      ) : null}

      {submit.isError && trimmed.length >= 1 && !isRecording && !transcribe.isPending ? (
        <div className="mt-3 flex flex-col gap-2 rounded-md border border-destructive/40 bg-destructive/10 p-3">
          <p className="text-sm text-destructive">
            Your answer did not save. Check your connection or sign-in, then try again.
          </p>
          <Button
            type="button"
            variant="secondary"
            size="sm"
            className="self-start"
            onClick={() => {
              submit.reset();
              triggeredRef.current = false;
              setSaveRetryKey((k) => k + 1);
            }}
          >
            Try saving again
          </Button>
        </div>
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

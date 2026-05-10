import type { ReactElement } from "react";
import { Lightbulb, Volume2, VolumeX } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useSpeechSynthesis } from "@/hooks/useSpeechSynthesis";
import { QuestionPills } from "./QuestionPills";
import type { Question } from "@/types/api";

interface QuestionPanelProps {
  questions: Question[];
  activeIndex: number;
  onSelect: (index: number) => void;
}

export function QuestionPanel({
  questions,
  activeIndex,
  onSelect,
}: QuestionPanelProps): ReactElement {
  const { speak, cancel, isSpeaking, isSupported } = useSpeechSynthesis();
  const current = questions[activeIndex];

  return (
    <div className="flex h-full flex-col gap-6">
      <QuestionPills count={questions.length} activeIndex={activeIndex} onSelect={onSelect} />
      <div className="rounded-2xl border bg-card p-6 shadow-sm">
        <div className="flex items-start justify-between gap-3">
          <p className="text-xs font-medium uppercase tracking-[0.2em] text-muted-foreground">
            Question {activeIndex + 1} of {questions.length}
          </p>
          {isSupported ? (
            <Button
              type="button"
              size="icon"
              variant="ghost"
              onClick={() => (isSpeaking ? cancel() : current && speak(current.question))}
              aria-label={isSpeaking ? "Stop speaking" : "Read question aloud"}
            >
              {isSpeaking ? <VolumeX className="h-4 w-4" /> : <Volume2 className="h-4 w-4" />}
            </Button>
          ) : null}
        </div>
        <h2 className="mt-4 font-display text-2xl font-semibold tracking-tight md:text-3xl">
          {current?.question ?? ""}
        </h2>
      </div>
      <div className="rounded-2xl border border-amber-300/40 bg-amber-50/60 p-5 text-sm text-amber-900/90 dark:border-amber-300/20 dark:bg-amber-200/10 dark:text-amber-100">
        <p className="flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.2em]">
          <Lightbulb className="h-4 w-4" /> Note
        </p>
        <p className="mt-2 leading-relaxed">
          Press <strong className="font-semibold">Record answer</strong>, speak, then{" "}
          <strong className="font-semibold">Stop</strong> — we save the text automatically (no audio is stored).
          Voice works best in <strong className="font-semibold">Chrome or Edge</strong>.
        </p>
      </div>
    </div>
  );
}

import type { ReactElement } from "react";
import { Gauge, MessageCircleQuestion, Timer } from "lucide-react";
import type { SpeechAnalysis } from "@/api/interviews";
import { cn } from "@/lib/cn";

interface SpeechAnalysisCardProps {
  analysis: SpeechAnalysis;
}

type Tone = "emerald" | "amber" | "rose";

// Thresholds tuned for typical interview answers (30s–3min).
// WPM: 130–160 reads as natural; below 110 drags; above 180 is rushed.
// Fillers: ≤2 is fine, ≤5 is noticeable, beyond that distracts.
// Long pauses: 0 ideal, 1–2 still ok, 3+ feels stuck.
function fillerTone(n: number): Tone {
  if (n <= 2) return "emerald";
  if (n <= 5) return "amber";
  return "rose";
}

function paceTone(wpm: number): Tone {
  if (wpm === 0) return "amber";
  if (wpm >= 130 && wpm <= 160) return "emerald";
  if (wpm >= 110 && wpm <= 180) return "amber";
  return "rose";
}

function pauseTone(n: number): Tone {
  if (n === 0) return "emerald";
  if (n <= 2) return "amber";
  return "rose";
}

function paceHint(wpm: number): string {
  if (wpm === 0) return "couldn't measure pace";
  if (wpm < 110) return "a touch slow";
  if (wpm > 180) return "speeding up";
  if (wpm >= 130 && wpm <= 160) return "natural pace";
  return "near natural pace";
}

const TONE_CLASSES: Record<Tone, string> = {
  emerald:
    "border-emerald-300/60 bg-emerald-50 text-emerald-900 dark:border-emerald-400/30 dark:bg-emerald-950/40 dark:text-emerald-100",
  amber:
    "border-amber-300/60 bg-amber-50 text-amber-900 dark:border-amber-400/30 dark:bg-amber-950/40 dark:text-amber-100",
  rose:
    "border-rose-300/60 bg-rose-50 text-rose-900 dark:border-rose-400/30 dark:bg-rose-950/40 dark:text-rose-100",
};

export function SpeechAnalysisCard({ analysis }: SpeechAnalysisCardProps): ReactElement {
  const { fillerCount, wordsPerMinute, longPauseCount } = analysis;

  return (
    <div
      data-testid="speech-analysis"
      className="mt-3 rounded-lg border bg-card p-3 shadow-sm"
    >
      <p className="text-xs font-semibold uppercase tracking-[0.2em] text-muted-foreground">
        Delivery
      </p>
      <div className="mt-2 flex flex-wrap gap-2">
        <Chip
          tone={fillerTone(fillerCount)}
          icon={<MessageCircleQuestion className="h-3.5 w-3.5" />}
          label="Filler words"
          value={fillerCount.toString()}
          hint={fillerCount === 0 ? "none — clean" : fillerCount === 1 ? "1 filler" : `${fillerCount} fillers`}
        />
        <Chip
          tone={paceTone(wordsPerMinute)}
          icon={<Gauge className="h-3.5 w-3.5" />}
          label="Words/min"
          value={wordsPerMinute.toString()}
          hint={paceHint(wordsPerMinute)}
        />
        <Chip
          tone={pauseTone(longPauseCount)}
          icon={<Timer className="h-3.5 w-3.5" />}
          label="Long pauses"
          value={longPauseCount.toString()}
          hint={longPauseCount === 0 ? "no gaps >2s" : `${longPauseCount} gap${longPauseCount === 1 ? "" : "s"} >2s`}
        />
      </div>
    </div>
  );
}

interface ChipProps {
  tone: Tone;
  icon: ReactElement;
  label: string;
  value: string;
  hint: string;
}

function Chip({ tone, icon, label, value, hint }: ChipProps): ReactElement {
  return (
    <div
      className={cn(
        "flex items-center gap-2 rounded-full border px-3 py-1.5 text-xs",
        TONE_CLASSES[tone],
      )}
    >
      {icon}
      <span className="font-semibold tabular-nums">{value}</span>
      <span className="font-medium">{label}</span>
      <span className="opacity-70">· {hint}</span>
    </div>
  );
}

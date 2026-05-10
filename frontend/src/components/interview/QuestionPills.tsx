import { useTransition, type ReactElement } from "react";
import { cn } from "@/lib/cn";

interface QuestionPillsProps {
  count: number;
  activeIndex: number;
  onSelect: (index: number) => void;
}

export function QuestionPills({ count, activeIndex, onSelect }: QuestionPillsProps): ReactElement {
  const [isPending, startTransition] = useTransition();
  return (
    <div className={cn("flex flex-wrap gap-2", isPending && "opacity-70")}>
      {Array.from({ length: count }, (_, i) => i).map((i) => (
        <button
          key={i}
          type="button"
          onClick={() => startTransition(() => onSelect(i))}
          aria-current={i === activeIndex ? "step" : undefined}
          className={cn(
            "rounded-full border px-4 py-1.5 text-xs font-medium uppercase tracking-[0.18em] transition-colors",
            i === activeIndex
              ? "border-primary bg-primary text-primary-foreground"
              : "border-border bg-background text-muted-foreground hover:border-foreground/30 hover:text-foreground",
          )}
        >
          Q{i + 1}
        </button>
      ))}
    </div>
  );
}

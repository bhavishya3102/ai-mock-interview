import { useRef, type ChangeEvent, type ReactElement } from "react";
import { CheckCircle2, FileText, Loader2, Trash2, Upload } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useToast } from "@/hooks/use-toast";
import { useDeleteResume, useResumeStatus, useUploadResume } from "@/hooks/useResume";
import { ApiError } from "@/api/client";

// Mirrors the backend's maxResumeBytes (5 MiB).
const MAX_RESUME_BYTES = 5 * 1024 * 1024;

function formatUploadedAt(iso: string | null): string {
  if (!iso) return "";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  return d.toLocaleDateString(undefined, { year: "numeric", month: "short", day: "numeric" });
}

export function ResumeCard(): ReactElement {
  const { data, isLoading } = useResumeStatus();
  const upload = useUploadResume();
  const remove = useDeleteResume();
  const { toast } = useToast();
  const inputRef = useRef<HTMLInputElement>(null);

  const attached = data?.attached ?? false;
  const busy = upload.isPending || remove.isPending;

  async function handleFile(e: ChangeEvent<HTMLInputElement>): Promise<void> {
    const file = e.target.files?.[0];
    e.target.value = ""; // let the user re-pick the same file
    if (!file) return;

    const isPdf =
      file.type === "application/pdf" || file.name.toLowerCase().endsWith(".pdf");
    if (!isPdf) {
      toast({
        title: "Not a PDF",
        description: "Upload your resume as a PDF file.",
        variant: "destructive",
      });
      return;
    }
    if (file.size > MAX_RESUME_BYTES) {
      toast({
        title: "File too large",
        description: "Resume must be under 5 MB.",
        variant: "destructive",
      });
      return;
    }

    try {
      await upload.mutateAsync(file);
      toast({
        title: "Resume saved",
        description: "New interviews will ask questions based on your resume.",
      });
    } catch (err) {
      const message = err instanceof ApiError ? err.message : "Could not upload resume.";
      toast({ title: "Upload failed", description: message, variant: "destructive" });
    }
  }

  async function handleRemove(): Promise<void> {
    try {
      await remove.mutateAsync();
      toast({ title: "Resume removed" });
    } catch (err) {
      const message = err instanceof ApiError ? err.message : "Could not remove resume.";
      toast({ title: "Remove failed", description: message, variant: "destructive" });
    }
  }

  return (
    <div className="flex flex-wrap items-center justify-between gap-4 rounded-2xl border bg-card p-5 shadow-sm">
      <div className="flex min-w-0 items-start gap-3">
        <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary">
          <FileText className="h-5 w-5" />
        </span>
        <div className="min-w-0">
          <p className="font-display text-lg font-semibold leading-tight tracking-tight">
            Resume
          </p>
          {isLoading ? (
            <p className="mt-0.5 text-sm text-muted-foreground">Checking…</p>
          ) : attached ? (
            <p className="mt-0.5 flex items-center gap-1.5 text-sm text-emerald-700 dark:text-emerald-400">
              <CheckCircle2 className="h-3.5 w-3.5 shrink-0" />
              Attached{data?.uploadedAt ? ` · ${formatUploadedAt(data.uploadedAt)}` : ""} —
              questions will reference it.
            </p>
          ) : (
            <p className="mt-0.5 text-sm text-muted-foreground">
              Upload a PDF resume so interview questions reference your real experience.
            </p>
          )}
        </div>
      </div>

      <div className="flex shrink-0 items-center gap-2">
        <input
          ref={inputRef}
          type="file"
          accept="application/pdf,.pdf"
          className="hidden"
          onChange={(e) => {
            void handleFile(e);
          }}
          data-testid="resume-file-input"
        />
        {attached && !busy ? (
          <Button
            type="button"
            variant="ghost"
            size="icon"
            onClick={() => {
              void handleRemove();
            }}
            aria-label="Remove resume"
          >
            <Trash2 className="h-4 w-4" />
          </Button>
        ) : null}
        <Button
          type="button"
          variant={attached ? "secondary" : "default"}
          onClick={() => inputRef.current?.click()}
          disabled={busy}
        >
          {upload.isPending ? (
            <>
              <Loader2 className="h-4 w-4 animate-spin" /> Uploading
            </>
          ) : remove.isPending ? (
            <>
              <Loader2 className="h-4 w-4 animate-spin" /> Removing
            </>
          ) : (
            <>
              <Upload className="h-4 w-4" /> {attached ? "Replace resume" : "Upload resume"}
            </>
          )}
        </Button>
      </div>
    </div>
  );
}

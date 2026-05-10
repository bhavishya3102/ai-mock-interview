import { useState, type ReactElement } from "react";
import { useNavigate } from "react-router-dom";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { Loader2, Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { useCreateInterview } from "@/hooks/useInterviewMutations";
import { useToast } from "@/hooks/use-toast";
import { ApiError } from "@/api/client";

export const newInterviewSchema = z.object({
  jobPosition: z.string().trim().min(2, "Tell us the role").max(120, "Keep it under 120 characters"),
  jobDescription: z
    .string()
    .trim()
    .min(10, "Add at least a sentence of context")
    .max(4000, "Keep it under 4000 characters"),
  yearsExperience: z.coerce.number().int().min(0, "Must be 0 or more").max(60, "Must be 60 or less"),
});

export type NewInterviewInput = z.infer<typeof newInterviewSchema>;

export function AddInterviewDialog(): ReactElement {
  const [open, setOpen] = useState(false);
  const navigate = useNavigate();
  const { toast } = useToast();
  const mutation = useCreateInterview();

  const form = useForm<NewInterviewInput>({
    resolver: zodResolver(newInterviewSchema),
    defaultValues: { jobPosition: "", jobDescription: "", yearsExperience: 0 },
    mode: "onSubmit",
  });

  const onSubmit = form.handleSubmit(async (values) => {
    try {
      const interview = await mutation.mutateAsync(values);
      toast({ title: "Interview ready", description: `Generated ${interview.questions.length} questions.` });
      setOpen(false);
      form.reset();
      navigate(`/interview/${interview.mockId}`);
    } catch (error) {
      const message =
        error instanceof ApiError ? error.message : "Could not generate interview. Try again.";
      toast({ title: "Something went wrong", description: message, variant: "destructive" });
    }
  });

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button size="lg" className="gap-2">
          <Plus className="h-4 w-4" /> New interview
        </Button>
      </DialogTrigger>
      <DialogContent className="sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>Set up a mock interview</DialogTitle>
          <DialogDescription>
            We&apos;ll tailor five questions to the role and your experience.
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={onSubmit} className="space-y-5">
          <div className="space-y-1.5">
            <label htmlFor="jobPosition" className="text-sm font-medium">
              Job position
            </label>
            <Input
              id="jobPosition"
              placeholder="Senior Backend Engineer"
              {...form.register("jobPosition")}
              autoComplete="off"
            />
            {form.formState.errors.jobPosition ? (
              <p className="text-xs text-destructive">{form.formState.errors.jobPosition.message}</p>
            ) : null}
          </div>
          <div className="space-y-1.5">
            <label htmlFor="jobDescription" className="text-sm font-medium">
              Job description / required tech
            </label>
            <Textarea
              id="jobDescription"
              rows={5}
              placeholder="Go services, Postgres, distributed systems, on-call..."
              {...form.register("jobDescription")}
            />
            {form.formState.errors.jobDescription ? (
              <p className="text-xs text-destructive">{form.formState.errors.jobDescription.message}</p>
            ) : null}
          </div>
          <div className="space-y-1.5">
            <label htmlFor="yearsExperience" className="text-sm font-medium">
              Years of experience
            </label>
            <Input
              id="yearsExperience"
              type="number"
              inputMode="numeric"
              min={0}
              max={60}
              {...form.register("yearsExperience")}
            />
            {form.formState.errors.yearsExperience ? (
              <p className="text-xs text-destructive">
                {form.formState.errors.yearsExperience.message}
              </p>
            ) : null}
          </div>
          <DialogFooter>
            <Button
              type="button"
              variant="ghost"
              onClick={() => setOpen(false)}
              disabled={mutation.isPending}
            >
              Cancel
            </Button>
            <Button type="submit" disabled={mutation.isPending}>
              {mutation.isPending ? (
                <>
                  <Loader2 className="h-4 w-4 animate-spin" /> Generating
                </>
              ) : (
                "Generate questions"
              )}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

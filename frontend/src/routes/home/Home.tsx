import type { ReactElement } from "react";
import { Link } from "react-router-dom";
import { ArrowRight, MicVocal, Sparkles, Target } from "lucide-react";
import { Header } from "@/components/layout/Header";
import { PageContainer } from "@/components/layout/PageContainer";
import { Button } from "@/components/ui/button";

const FEATURES = [
  {
    icon: MicVocal,
    title: "Voice-first practice",
    body: "Speak your answers like in the real thing. We transcribe and rate them in seconds.",
  },
  {
    icon: Target,
    title: "Role-tailored prompts",
    body: "Five questions calibrated to your job role, description, and experience level.",
  },
  {
    icon: Sparkles,
    title: "Actionable feedback",
    body: "Per-question rating, reference answer, and concrete coaching to level up fast.",
  },
] as const;

export default function Home(): ReactElement {
  return (
    <div className="relative min-h-screen overflow-hidden bg-background">
      <div
        aria-hidden
        className="pointer-events-none absolute inset-x-0 top-0 -z-10 h-[640px] bg-[radial-gradient(circle_at_18%_-10%,hsl(var(--accent)/0.22),transparent_55%),radial-gradient(circle_at_85%_-30%,hsl(var(--primary)/0.18),transparent_55%)]"
      />
      <Header />
      <PageContainer className="pt-16 md:pt-24">
        <section className="mx-auto max-w-3xl text-center">
          <span className="inline-flex items-center gap-2 rounded-full border border-border/70 bg-background/60 px-3 py-1 text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">
            AI mock interview
          </span>
          <h1 className="mt-6 font-display text-5xl font-semibold tracking-tight text-balance md:text-7xl">
            Rehearse the conversation that lands the job.
          </h1>
          <p className="mt-6 text-balance text-lg text-muted-foreground md:text-xl">
            Eunoia turns any role description into a five-question simulation. Speak your answers,
            get rated, and walk in confident.
          </p>
          <div className="mt-10 flex flex-col items-center justify-center gap-3 sm:flex-row">
            <Button asChild size="lg">
              <Link to="/dashboard">
                Start a session <ArrowRight className="h-4 w-4" />
              </Link>
            </Button>
            <Button asChild variant="ghost" size="lg">
              <Link to="/sign-up">Create account</Link>
            </Button>
          </div>
        </section>

        <section className="mx-auto mt-24 grid w-full max-w-5xl gap-4 md:mt-32 md:grid-cols-3">
          {FEATURES.map(({ icon: Icon, title, body }) => (
            <article
              key={title}
              className="group relative rounded-2xl border bg-card p-6 transition-shadow hover:shadow-md"
            >
              <Icon className="h-6 w-6 text-accent" aria-hidden />
              <h3 className="mt-4 font-display text-xl font-semibold tracking-tight">{title}</h3>
              <p className="mt-2 text-sm text-muted-foreground">{body}</p>
            </article>
          ))}
        </section>
      </PageContainer>
    </div>
  );
}

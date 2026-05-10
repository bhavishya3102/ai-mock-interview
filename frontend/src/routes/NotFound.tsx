import type { ReactElement } from "react";
import { Link } from "react-router-dom";
import { Button } from "@/components/ui/button";
import { PageContainer } from "@/components/layout/PageContainer";

export default function NotFound(): ReactElement {
  return (
    <PageContainer className="flex min-h-screen flex-col items-center justify-center text-center">
      <p className="text-sm font-medium uppercase tracking-[0.2em] text-muted-foreground">404</p>
      <h1 className="mt-4 font-display text-5xl font-semibold tracking-tight md:text-6xl">
        Lost the thread.
      </h1>
      <p className="mt-4 max-w-md text-balance text-muted-foreground">
        The page you tried to reach is not here. Pick a path back below.
      </p>
      <div className="mt-8 flex gap-3">
        <Button asChild>
          <Link to="/">Go home</Link>
        </Button>
        <Button asChild variant="ghost">
          <Link to="/dashboard">Open dashboard</Link>
        </Button>
      </div>
    </PageContainer>
  );
}

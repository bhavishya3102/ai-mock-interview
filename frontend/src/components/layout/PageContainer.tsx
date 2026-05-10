import type { ReactElement, ReactNode } from "react";
import { cn } from "@/lib/cn";

interface PageContainerProps {
  children: ReactNode;
  className?: string;
}

export function PageContainer({ children, className }: PageContainerProps): ReactElement {
  return (
    <main className={cn("mx-auto w-full max-w-6xl px-6 py-10 md:py-14", className)}>{children}</main>
  );
}

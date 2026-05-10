import type { ReactElement } from "react";
import { Outlet } from "react-router-dom";

export default function AuthLayout(): ReactElement {
  return (
    <div className="min-h-screen bg-background">
      <div
        aria-hidden
        className="pointer-events-none absolute inset-0 -z-10 bg-[radial-gradient(circle_at_30%_-20%,hsl(var(--accent)/0.18),transparent_60%),radial-gradient(circle_at_80%_120%,hsl(var(--primary)/0.18),transparent_55%)]"
      />
      <div className="flex min-h-screen items-center justify-center px-6 py-16">
        <Outlet />
      </div>
    </div>
  );
}

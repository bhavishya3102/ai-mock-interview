import type { ReactElement } from "react";
import { Outlet, ScrollRestoration } from "react-router-dom";
import { useAuthFetch } from "@/hooks/useAuthFetch";
import { Toaster } from "@/components/ui/toaster";

function AuthBridge(): null {
  useAuthFetch();
  return null;
}

export default function Root(): ReactElement {
  return (
    <>
      <AuthBridge />
      <Outlet />
      <Toaster />
      <ScrollRestoration />
    </>
  );
}

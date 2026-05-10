import type { ReactElement } from "react";
import { Outlet } from "react-router-dom";
import { Header } from "@/components/layout/Header";

export default function DashboardLayout(): ReactElement {
  return (
    <div className="min-h-screen bg-background">
      <Header />
      <Outlet />
    </div>
  );
}

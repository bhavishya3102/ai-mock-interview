import { Suspense, lazy, type ReactElement } from "react";
import { Loader2 } from "lucide-react";
import { createBrowserRouter } from "react-router-dom";
import Root from "./root";
import Home from "./home/Home";
import Dashboard from "./dashboard/Dashboard";
import PreInterview from "./interview/PreInterview";
import NotFound from "./NotFound";
import SignInPage from "./auth/SignInPage";
import SignUpPage from "./auth/SignUpPage";
import DashboardLayout from "./_layouts/DashboardLayout";
import AuthLayout from "./_layouts/AuthLayout";
import { ProtectedRoute } from "./_guards/ProtectedRoute";

const InterviewSession = lazy(() => import("./interview/InterviewSession"));
const Feedback = lazy(() => import("./interview/Feedback"));

function RouteFallback(): ReactElement {
  return (
    <div className="flex min-h-[60vh] items-center justify-center text-sm text-muted-foreground">
      <Loader2 className="mr-2 h-4 w-4 animate-spin" /> Loading
    </div>
  );
}

export const router = createBrowserRouter([
  {
    element: <Root />,
    children: [
      { path: "/", element: <Home /> },
      {
        element: <AuthLayout />,
        children: [
          { path: "/sign-in/*", element: <SignInPage /> },
          { path: "/sign-up/*", element: <SignUpPage /> },
        ],
      },
      {
        element: (
          <ProtectedRoute>
            <DashboardLayout />
          </ProtectedRoute>
        ),
        children: [
          { path: "/dashboard", element: <Dashboard /> },
          { path: "/interview/:mockId", element: <PreInterview /> },
          {
            path: "/interview/:mockId/start",
            element: (
              <Suspense fallback={<RouteFallback />}>
                <InterviewSession />
              </Suspense>
            ),
          },
          {
            path: "/interview/:mockId/feedback",
            element: (
              <Suspense fallback={<RouteFallback />}>
                <Feedback />
              </Suspense>
            ),
          },
        ],
      },
      { path: "*", element: <NotFound /> },
    ],
  },
]);

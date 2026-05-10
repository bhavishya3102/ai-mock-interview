import type { ReactElement } from "react";
import { RouterProvider } from "react-router-dom";
import { router } from "./routes";

export default function App(): ReactElement {
  return <RouterProvider router={router} />;
}

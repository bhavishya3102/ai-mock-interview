import type { ReactElement } from "react";
import { SignIn } from "@clerk/clerk-react";

export default function SignInPage(): ReactElement {
  return (
    <SignIn
      routing="path"
      path="/sign-in"
      signUpUrl="/sign-up"
      fallbackRedirectUrl="/dashboard"
      forceRedirectUrl="/dashboard"
    />
  );
}

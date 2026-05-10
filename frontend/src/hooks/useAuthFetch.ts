import { useAuth } from "@clerk/clerk-react";
import { useEffect } from "react";
import { setTokenGetter } from "@/lib/tokenStore";

export function useAuthFetch(): void {
  const { getToken, isLoaded } = useAuth();

  useEffect(() => {
    if (!isLoaded) return;
    setTokenGetter(() => getToken());
    return () => setTokenGetter(null);
  }, [getToken, isLoaded]);
}

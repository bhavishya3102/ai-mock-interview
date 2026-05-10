import { useCallback, useState } from "react";

export type WebcamPermissionState = "idle" | "granted" | "denied" | "unsupported";

export interface UseWebcamPermissionReturn {
  state: WebcamPermissionState;
  request: () => Promise<void>;
  onUserMedia: () => void;
  onUserMediaError: (error: string | DOMException) => void;
}

function detectInitialState(): WebcamPermissionState {
  if (typeof navigator === "undefined") return "unsupported";
  const mediaDevices = navigator.mediaDevices as MediaDevices | undefined;
  if (!mediaDevices || typeof mediaDevices.getUserMedia !== "function") {
    return "unsupported";
  }
  return "idle";
}

export function useWebcamPermission(): UseWebcamPermissionReturn {
  const [state, setState] = useState<WebcamPermissionState>(detectInitialState);

  const onUserMedia = useCallback(() => setState("granted"), []);
  const onUserMediaError = useCallback(() => setState("denied"), []);

  const request = useCallback(async () => {
    if (typeof navigator === "undefined") {
      setState("unsupported");
      return;
    }
    const mediaDevices = navigator.mediaDevices as MediaDevices | undefined;
    if (!mediaDevices || typeof mediaDevices.getUserMedia !== "function") {
      setState("unsupported");
      return;
    }
    try {
      const stream = await mediaDevices.getUserMedia({ video: true, audio: false });
      stream.getTracks().forEach((t) => t.stop());
      setState("granted");
    } catch {
      setState("denied");
    }
  }, []);

  return { state, request, onUserMedia, onUserMediaError };
}
